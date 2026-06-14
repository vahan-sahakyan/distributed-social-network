package cache

import (
	"github.com/redis/go-redis/v9"
)

func NewRedis(url string) *redis.Client {
	opts, err := redis.ParseURL(url)
	if err != nil {
		opts = &redis.Options{Addr: "localhost:6379"}
	}
	return redis.NewClient(opts)
}
