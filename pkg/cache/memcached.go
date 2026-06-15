package cache

import (
	"github.com/bradfitz/gomemcache/memcache"
)

func NewMemcached(servers ...string) *memcache.Client {
	return memcache.New(servers...)
}
