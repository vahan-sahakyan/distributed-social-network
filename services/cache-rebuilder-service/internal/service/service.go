package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/vahan-sahakyan/distributed-social-network/cache-rebuilder-service/internal/repository"
)

type Service struct {
	repo *repository.Repository
	mc   *memcache.Client
}

func New(repo *repository.Repository, mc *memcache.Client) *Service {
	return &Service{repo: repo, mc: mc}
}

func (s *Service) RebuildCache(ctx context.Context) error {
	log.Println("starting cache rebuild from ClickHouse...")

	states, err := s.repo.GetPostStates(ctx)
	if err != nil {
		return fmt.Errorf("failed to get post states: %w", err)
	}

	for _, state := range states {
		key := fmt.Sprintf("post_state:%s", state.PostID)
		data, err := json.Marshal(state)
		if err != nil {
			log.Printf("error marshaling state for post %s: %v", state.PostID, err)
			continue
		}
		if err := s.mc.Set(&memcache.Item{
			Key:        key,
			Value:      data,
			Expiration: 3600,
		}); err != nil {
			log.Printf("error setting cache for post %s: %v", state.PostID, err)
		}
	}

	log.Printf("cache rebuild complete: %d post states restored", len(states))
	return nil
}
