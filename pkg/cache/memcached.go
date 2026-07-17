package cache

import (
	"context"
	"log"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
)

// NewMemcached returns a connected memcache client, retrying until the server is reachable or ctx is done.
func NewMemcached(ctx context.Context, servers ...string) *memcache.Client {
	for {
		mc := memcache.New(servers...)
		if err := mc.Ping(); err == nil {
			return mc
		}
		log.Printf("waiting for memcached at %v...", servers)
		select {
		case <-ctx.Done():
			log.Fatalf("context cancelled waiting for memcached")
		case <-time.After(2 * time.Second):
		}
	}
}
