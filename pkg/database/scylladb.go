package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gocql/gocql"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/retry"
)

// NewScyllaDB creates a session, retrying until the cluster answers or ctx is done.
func NewScyllaDB(ctx context.Context, hosts string, keyspace string) (*gocql.Session, error) {
	cluster := gocql.NewCluster(strings.Split(hosts, ",")...)
	cluster.Keyspace = keyspace
	cluster.Consistency = gocql.Quorum
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second

	var session *gocql.Session
	err := retry.Do(ctx, "connecting to scylladb", func(context.Context) error {
		var err error
		session, err = cluster.CreateSession()
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("connecting to scylladb: %w", err)
	}
	return session, nil
}

// MigrateScylla connects to the cluster without a keyspace and executes each
// semicolon-separated CQL statement in sql. Intended for keyspace + table setup
// before the main session (which requires the keyspace to already exist) is created.
// The connection is retried until the cluster answers or ctx is done.
func MigrateScylla(ctx context.Context, hosts, sql string) error {
	cluster := gocql.NewCluster(strings.Split(hosts, ",")...)
	cluster.Timeout = 30 * time.Second
	cluster.ConnectTimeout = 30 * time.Second

	var session *gocql.Session
	err := retry.Do(ctx, "connecting to scylladb for migration", func(context.Context) error {
		var err error
		session, err = cluster.CreateSession()
		return err
	})
	if err != nil {
		return fmt.Errorf("connecting to scylladb for migration: %w", err)
	}
	defer session.Close()

	for _, stmt := range strings.Split(sql, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if err := session.Query(stmt).WithContext(ctx).Exec(); err != nil {
			return fmt.Errorf("migration statement failed: %w", err)
		}
	}
	return nil
}
