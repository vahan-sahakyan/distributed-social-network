package database

import (
	"fmt"
	"strings"
	"time"

	"github.com/gocql/gocql"
)

func NewScyllaDB(hosts string, keyspace string) (*gocql.Session, error) {
	cluster := gocql.NewCluster(strings.Split(hosts, ",")...)
	cluster.Keyspace = keyspace
	cluster.Consistency = gocql.Quorum
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second

	session, err := cluster.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to scylladb: %w", err)
	}

	return session, nil
}

// MigrateScylla connects to the cluster without a keyspace and executes each
// semicolon-separated CQL statement in sql. Intended for keyspace + table setup
// before the main session (which requires the keyspace to already exist) is created.
func MigrateScylla(hosts, sql string) error {
	cluster := gocql.NewCluster(strings.Split(hosts, ",")...)
	cluster.Timeout = 30 * time.Second
	cluster.ConnectTimeout = 30 * time.Second

	session, err := cluster.CreateSession()
	if err != nil {
		return fmt.Errorf("failed to connect for migration: %w", err)
	}
	defer session.Close()

	for _, stmt := range strings.Split(sql, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if err := session.Query(stmt).Exec(); err != nil {
			return fmt.Errorf("migration statement failed: %w", err)
		}
	}
	return nil
}
