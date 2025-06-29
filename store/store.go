package store

import (
	EvictionPolicy "MeghanshBansal/cache/evictionPolicies"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Store struct {
	Id        string
	data      map[string]*CacheData
	policy    EvictionPolicy.Policy
	mu        *sync.Mutex
	limit     int // if limit is 0, limit logic will not be implemented
	size      int
	CloseChan chan struct{}
}

func CreateStore(limit int, policy *EvictionPolicy.Policy) *Store {
	id := uuid.New().String()
	store := Store{
		Id:     id,
		data:   make(map[string]*CacheData),
		mu:     &sync.Mutex{},
		limit:  limit,
		size:   0,
		policy: *policy,
	}
	return &store
}

func (s *Store) Has(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.data[key]
	if !ok {
		return false
	}

	if time.Now().After(item.createdAt.Add(item.ttl)) {
		s.removeItem(key)
	}

	item.lastUsed = time.Now()
	item.accessCnt++

	if s.policy != nil {
		s.policy.RecordAccess(item)
	}
	return ok
}

func (s *Store) Get(key string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.data[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(item.createdAt.Add(item.ttl)) {
		s.removeItem(key)
	}

	item.lastUsed = time.Now()
	item.accessCnt++

	if s.policy != nil {
		s.policy.RecordAccess(item)
	}
	return item.value, ok
}

func (s *Store) Set(key string, value any, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ttl <= 0 {
		ttl = time.Hour * 24 * 7 // Default 1 week
	}

	// if updating the existing
	if item, ok := s.data[key]; ok {
		item.value = value
		item.lastUsed = time.Now()
		item.ttl = ttl

		if s.policy != nil {
			s.policy.RecordAccess(item)
		}
	}

	if s.limit > 0 && s.size >= s.limit {
		s.evictOne()
	}

	item := &CacheData{
		key:       key,
		value:     value,
		ttl:       ttl,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
		accessCnt: 0,
	}
	s.data[key] = item
	s.size++

	if s.policy != nil {
		s.policy.RecordAccess(item)
	}
}

func (s *Store) evictOne() {
	if s.policy != nil || s.size == 0 {
		return
	}

	key := s.policy.Evict()
	if key != "" {
		s.removeItem(key)
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

	expiredKeys := make([]string, 0)
	for key, item := range s.data {
		if time.Now().After(item.createdAt.Add(item.ttl)) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		s.removeItem(key)
	}
}

func (s *Store) Size() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.size
}

func (s *Store) IsLimited() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.limit > 0
}

func (s *Store) removeItem(key string) {
	_, exists := s.data[key]
	if !exists {
		return
	}

	// Remove from policy if set
	if s.policy != nil {
		s.policy.RemoveItem(key)
	}

	delete(s.data, key)
	s.size--
}
