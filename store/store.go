package store

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type Store struct {
	Id        string
	Data      map[string]CacheData
	mu        *sync.Mutex
	limit     int
	CloseChan chan struct{}
}

func (s *Store) createStore(limit int) *Store {
	id := uuid.New().String()
	store := Store{
		Id:    id,
		Data:  make(map[string]CacheData),
		mu:    &sync.Mutex{},
		limit: limit,
	}
	return &store
}

func (s *Store) get(key string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.Data[key]
	return value, ok
}

func (s *Store) set(key string, value any, sec int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Data[key] = CacheData{
		Value:   value,
		TTL:     time.Duration(sec) * time.Second,
		Created: time.Now(),
	}
}

func (s *Store) StartCleaner(dur time.Duration) {
	ticker := time.NewTicker(dur)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanBackground()
		case <-s.CloseChan:
			return
		}
	}
}

func (s *Store) cleanBackground() {
	s.mu.Lock()
	s.mu.Unlock()

	for key, item := range s.Data {
		if item.HasExpired() {
			delete(s.Data, key)
		}
	}
}
