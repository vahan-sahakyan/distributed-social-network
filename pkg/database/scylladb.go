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
