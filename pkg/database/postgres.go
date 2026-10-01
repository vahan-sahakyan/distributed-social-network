package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/retry"
)

// NewPostgres connects and pings, retrying until the database answers or ctx is done.
func NewPostgres(ctx context.Context, connString string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("parsing connection string: %w", err)
	}

	var pool *pgxpool.Pool
	err = retry.Do(ctx, "connecting to postgres", func(ctx context.Context) error {
		p, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			return err
		}
		if err := p.Ping(ctx); err != nil {
			p.Close()
			return err
		}
		pool = p
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("connecting to postgres: %w", err)
	}
	return pool, nil
}

// MigratePostgres runs each semicolon-separated statement in sql against pool.
func MigratePostgres(ctx context.Context, pool *pgxpool.Pool, sql string) error {
	for _, stmt := range strings.Split(sql, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}
	return nil
}
