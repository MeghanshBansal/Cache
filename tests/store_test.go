package store_test

import (
	"sync"
	"testing"
	"time"

	EvictionPolicy "MeghanshBansal/cache/evictionPolicies"
	Store "MeghanshBansal/cache/store"
	"github.com/stretchr/testify/assert"
)

func createTestStore(t *testing.T, limit int, policy EvictionPolicy.Policy) *Store.Store {
	store := Store.CreateStore(limit, policy)
	t.Cleanup(func() {
		store.Shutdown()
	})
	return store
}

func TestStore_Creation(t *testing.T) {
	t.Parallel()

	t.Run("unlimited store", func(t *testing.T) {
		s := createTestStore(t, 0, nil)
		assert.Equal(t, 0, s.Size())
		assert.False(t, s.IsLimited())
	})

	t.Run("limited store", func(t *testing.T) {
		s := createTestStore(t, 100, nil)
		assert.Equal(t, 0, s.Size())
		assert.True(t, s.IsLimited())
	})
}

func TestStore_BasicOperations(t *testing.T) {
	t.Parallel()

	s := createTestStore(t, 0, nil)

	t.Run("set and get", func(t *testing.T) {
		val := "test value"
		s.Set("key1", val, time.Hour)

		retrieved, found := s.Get("key1")
		assert.True(t, found)
		assert.Equal(t, val, retrieved)
	})

	t.Run("non-existent key", func(t *testing.T) {
		_, found := s.Get("nonexistent")
		assert.False(t, found)
	})

	t.Run("has key", func(t *testing.T) {
		s.Set("key2", "value", time.Hour)
		assert.True(t, s.Has("key2"))
		assert.False(t, s.Has("nonexistent"))
	})

	t.Run("update existing key", func(t *testing.T) {
		s.Set("key3", "old", time.Hour)
		s.Set("key3", "new", time.Hour)

		val, found := s.Get("key3")
		assert.True(t, found)
		assert.Equal(t, "new", val)
	})
}

func TestStore_Expiration(t *testing.T) {
	t.Parallel()

	s := createTestStore(t, 0, nil)
	go s.StartCleaner(50 * time.Millisecond)

	t.Run("item expires", func(t *testing.T) {
		s.Set("temp", "value", 100*time.Millisecond)
		_, found := s.Get("temp")
		assert.True(t, found)

		time.Sleep(150 * time.Millisecond)
		_, found = s.Get("temp")
		assert.False(t, found)
	})

	t.Run("item with default TTL", func(t *testing.T) {
		s.Set("perm", "value", 0) // Should use default 1 week
		_, found := s.Get("perm")
		assert.True(t, found)
	})
}

func TestStore_Eviction(t *testing.T) {
	t.Parallel()

	t.Run("no eviction when unlimited", func(t *testing.T) {
		s := createTestStore(t, 0, nil)
		for i := 0; i < 1000; i++ {
			s.Set(string(rune(i)), i, time.Hour)
		}
		assert.Equal(t, 1000, s.Size())
	})

	t.Run("basic eviction when limited", func(t *testing.T) {
		s := createTestStore(t, 2, nil)
		s.Set("a", 1, time.Hour)
		s.Set("b", 2, time.Hour)
		s.Set("c", 3, time.Hour) // Should evict one item

		assert.Equal(t, 2, s.Size())
	})

	t.Run("LRU eviction policy", func(t *testing.T) {
		lru := EvictionPolicy.NewLRUPolicy()
		s := createTestStore(t, 3, lru)

		// Fill cache
		s.Set("a", 1, time.Hour)
		s.Set("b", 2, time.Hour)
		s.Set("c", 3, time.Hour)

		// Access pattern: a is MRU, b is middle, c is LRU
		s.Get("a")
		s.Get("b")
		s.Get("a")

		// Add new item - should evict c
		s.Set("d", 4, time.Hour)

		assert.Equal(t, 3, s.Size())
		assert.False(t, s.Has("c"))
		assert.True(t, s.Has("a"))
		assert.True(t, s.Has("b"))
		assert.True(t, s.Has("d"))
	})

	t.Run("random eviction policy", func(t *testing.T) {
		random := EvictionPolicy.NewRandomPolicy()
		s := createTestStore(t, 3, random)

		// Fill cache
		s.Set("a", 1, time.Hour)
		s.Set("b", 2, time.Hour)
		s.Set("c", 3, time.Hour)

		// Add new item - should evict randomly
		s.Set("d", 4, time.Hour)

		assert.Equal(t, 3, s.Size())
	})
}

func TestStore_Concurrency(t *testing.T) {
	t.Parallel()

	s := createTestStore(t, 1000, EvictionPolicy.NewLRUPolicy())
	go s.StartCleaner(100 * time.Millisecond)

	var wg sync.WaitGroup
	workers := 10
	opsPerWorker := 100

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerWorker; j++ {
				key := string(rune(id*1000 + j))
				s.Set(key, j, time.Second)
				s.Get(key)
				if j%10 == 0 {
					s.Has(key)
				}
			}
		}(i)
	}

	wg.Wait()
	assert.True(t, s.Size() <= 1000, "Cache should respect size limit")
}

func TestStore_MemoryManagement(t *testing.T) {
	t.Parallel()

	s := createTestStore(t, 0, nil)
	go s.StartCleaner(100 * time.Millisecond)

	t.Run("memory reclamation", func(t *testing.T) {
		// Add items with short TTL
		for i := 0; i < 100; i++ {
			s.Set(string(rune(i)), make([]byte, 1024), 50*time.Millisecond)
		}

		initialSize := s.Size()
		time.Sleep(150 * time.Millisecond)

		assert.Less(t, s.Size(), initialSize, "Expired items should be cleaned")
	})
}

func TestStore_EdgeCases(t *testing.T) {
	t.Parallel()

	s := createTestStore(t, 0, nil)

	t.Run("empty key", func(t *testing.T) {
		s.Set("", "empty", time.Hour)
		val, found := s.Get("")
		assert.True(t, found)
		assert.Equal(t, "empty", val)
	})

	t.Run("nil value", func(t *testing.T) {
		s.Set("nil", nil, time.Hour)
		val, found := s.Get("nil")
		assert.True(t, found)
		assert.Nil(t, val)
	})

	t.Run("negative TTL", func(t *testing.T) {
		s.Set("neg", "value", -time.Hour)
		_, found := s.Get("neg")
		assert.True(t, found, "Items with negative TTL should persist")
	})
}

func TestStore_Shutdown(t *testing.T) {
	t.Parallel()

	s := Store.CreateStore(0, nil)
	s.StartCleaner(1000 * time.Millisecond)

	// Add some items
	s.Set("a", 1, time.Hour)
	s.Set("b", 2, 100*time.Millisecond)

	// Shutdown and verify cleaner stops
	s.Shutdown()

	// Verify we can still access non-expired items
	_, found := s.Get("a")
	assert.True(t, found)

	// Wait to see if cleaner runs (it shouldn't)
	time.Sleep(1500 * time.Millisecond)
	_, found = s.Get("b")
	assert.True(t, found, "Item should still exist because cleaner stopped")
}

func BenchmarkStore_Set(b *testing.B) {
	s := Store.CreateStore(0, nil)
	defer s.Shutdown()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Set(string(rune(i)), i, time.Hour)
	}
}