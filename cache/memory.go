package cache

import (
	"context"
	"time"

	gocache "github.com/patrickmn/go-cache"
)

type memoryStore struct {
	client *gocache.Cache
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		client: gocache.New(5*time.Minute, 10*time.Minute),
	}
}

func (s *memoryStore) Set(_ context.Context, key string, value string, ttl time.Duration) error {
	s.client.Set(key, value, ttl)
	return nil
}

func (s *memoryStore) Get(_ context.Context, key string) (string, bool, error) {
	value, ok := s.client.Get(key)
	if !ok {
		return "", false, nil
	}
	text, ok := value.(string)
	if !ok {
		return "", false, nil
	}
	return text, true, nil
}

func (s *memoryStore) Delete(_ context.Context, key string) error {
	s.client.Delete(key)
	return nil
}

func (s *memoryStore) Close() error {
	s.client.Flush()
	return nil
}
