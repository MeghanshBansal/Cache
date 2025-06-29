package store

import (
	EvictionPolicy "MeghanshBansal/cache/evictionPolicies"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Store struct {
	id        string
	data      map[string]*CacheData
	policy    EvictionPolicy.Policy
	mu        sync.RWMutex // Use RWMutex for better read performance
	limit     int
	size      int
	closeChan chan struct{}
}

func CreateStore(limit int, policy EvictionPolicy.Policy) *Store {
	return &Store{
		id:        uuid.New().String(),
		data:      make(map[string]*CacheData),
		policy:    policy,
		limit:     limit,
		closeChan: make(chan struct{}),
	}
}

func (s *Store) StartCleaner(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.cleanExpired()
			case <-s.closeChan:
				return
			}
		}
	}()
}

func (s *Store) Shutdown() {
	close(s.closeChan)
}

func (s *Store) Get(key string) (interface{}, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.data[key]
	if !exists {
		return nil, false
	}

	if s.isItemExpired(item) {
		s.removeItem(key)
		return nil, false
	}

	s.updateItemAccess(item)
	return item.value, true
}

func (s *Store) Set(key string, value interface{}, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ttl <= 0 {
		ttl = time.Hour * 24 * 7 // Default 1 week
	}

	// Update existing item
	if item, exists := s.data[key]; exists {
		item.value = value
		item.ttl = ttl
		s.updateItemAccess(item)
		return
	}

	// Check if we need to evict
	if s.limit > 0 && s.size >= s.limit {
		s.evictOne()
	}

	// Add new item
	item := &CacheData{
		key:       key,
		value:     value,
		ttl:       ttl,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
	}
	s.data[key] = item
	s.size++

	if s.policy != nil {
		s.policy.AddItem(item)
	}
}

func (s *Store) evictOne() {
	if s.size == 0 {
		return
	}

	if s.policy == nil {
		// Random eviction if no policy
		for key := range s.data {
			s.removeItem(key)
			return
		}
		return
	}

	if key := s.policy.Evict(); key != "" {
		s.removeItem(key)
	}
}

func (s *Store) cleanExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()

	expiredKeys := make([]string, 0, len(s.data)/4) // Pre-allocate some space
	for key, item := range s.data {
		if s.isItemExpired(item) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		s.removeItem(key)
	}
}

func (s *Store) isItemExpired(item *CacheData) bool {
	return time.Now().After(item.createdAt.Add(item.ttl))
}

func (s *Store) updateItemAccess(item *CacheData) {
	item.lastUsed = time.Now()
	item.accessCnt++
	if s.policy != nil {
		s.policy.RecordAccess(item)
	}
}

func (s *Store) removeItem(key string) {
	_, exists := s.data[key]
	if !exists {
		return
	}

	if s.policy != nil {
		s.policy.RemoveItem(key)
	}

	delete(s.data, key)
	s.size--
}

func (s *Store) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.size
}

func (s *Store) Has(key string) bool {
	_, exists := s.Get(key)
	return exists
}

func (s *Store) IsLimited() bool {
	return s.limit > 0
}
