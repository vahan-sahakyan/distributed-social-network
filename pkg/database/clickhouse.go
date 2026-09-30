package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/retry"
)

// NewClickHouse opens a connection and retries the ping until the server answers or ctx is done.
func NewClickHouse(ctx context.Context, addr string, database string) (driver.Conn, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
		Auth: clickhouse.Auth{
			Database: database,
			Username: "default",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open clickhouse connection: %w", err)
	}

	if err := retry.Do(ctx, "connecting to clickhouse", conn.Ping); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("connecting to clickhouse: %w", err)
	}

	return conn, nil
}

// MigrateClickHouse runs each semicolon-separated statement in sql against conn.
// Line comments are stripped first, since a semicolon inside one would otherwise
// split a single statement into two invalid fragments.
func MigrateClickHouse(ctx context.Context, conn driver.Conn, sql string) error {
	for _, stmt := range strings.Split(stripLineComments(sql), ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if err := conn.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}
	return nil
}

// stripLineComments removes "--" comments, leaving "--" inside single-quoted
// string literals alone.
func stripLineComments(sql string) string {
	var out strings.Builder
	for _, line := range strings.Split(sql, "\n") {
		inQuote := false
		for i := 0; i < len(line); i++ {
			if line[i] == '\'' {
				inQuote = !inQuote
				continue
			}
			if !inQuote && line[i] == '-' && i+1 < len(line) && line[i+1] == '-' {
				line = line[:i]
				break
			}
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return out.String()
}
