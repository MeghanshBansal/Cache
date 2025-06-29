package cache_test

import (
	EvictionPolicy "MeghanshBansal/cache/evictionPolicies"
	Store "MeghanshBansal/cache/store"
	"runtime"
	"runtime/debug"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Helper function to create test cache with proper cleanup
func createTestCache(t *testing.T, limit int, policy EvictionPolicy.Policy) *Store.Store {
	c := Store.CreateStore(limit, policy)
	t.Cleanup(func() {
		close(c.CloseChan)
	})
	return c
}

// --- Basic Functionality Tests ---

// TestBasicOperations tests Get, Set, and operations
func TestBasicOperations(t *testing.T) {
	t.Parallel()
	c := createTestCache(t, 0, nil) // Unlimited cache

	// Test Set and Get
	c.Set("key1", "value1", time.Hour)
	val, found := c.Get("key1")
	assert.True(t, found)
	assert.Equal(t, "value1", val)

	// Test updating value
	c.Set("key1", "new_value", time.Hour)
	val, found = c.Get("key1")
	assert.True(t, found)
	assert.Equal(t, "new_value", val)

	// Test non-existent key
	_, found = c.Get("nonexistent")
	assert.False(t, found)
}

// TestExpiration tests TTL functionality
func TestExpiration(t *testing.T) {
	t.Parallel()
	c := createTestCache(t, 0, nil)

	// Set item with short TTL
	c.Set("temp", "value", 2*time.Second)

	// Should exist initially
	_, found := c.Get("temp")
	assert.True(t, found)

	// Wait for expiration with buffer
	time.Sleep(3 * time.Second)
	_, found = c.Get("temp")
	assert.False(t, found, "Item should have expired")
}


// --- Eviction Policy Tests ---

func TestLRUEviction(t *testing.T) {
	t.Parallel()
	lruPolicy := EvictionPolicy.NewLRUPolicy()
	c := createTestCache(t, 3, lruPolicy) // Cache with size 3

	// Fill the cache
	c.Set("key1", "val1", time.Hour)
	c.Set("key2", "val2", time.Hour)
	c.Set("key3", "val3", time.Hour)

	// Access pattern to establish LRU order
	c.Get("key1") // key1 becomes MRU
	c.Get("key2") // key2 becomes MRU, key1 becomes middle
	c.Get("key1") // key1 becomes MRU again

	// Add one more item - should evict least recently used (key3)
	c.Set("key4", "val4", time.Hour)

	// Verify key3 was evicted
	_, found := c.Get("key3")
	assert.False(t, found, "key3 should be evicted")

	// Verify other keys exist
	_, found = c.Get("key1")
	assert.True(t, found, "key1 should exist")
	_, found = c.Get("key2")
	assert.True(t, found, "key2 should exist")
	_, found = c.Get("key4")
	assert.True(t, found, "key4 should exist")
}

// TestRandomEviction tests Random EvictionPolicy policy
func TestRandomEviction(t *testing.T) {
	t.Parallel()
	randomPolicy := EvictionPolicy.NewRandomPolicy()
	c := createTestCache(t, 3, randomPolicy)

	// Fill the cache
	c.Set("key1", "val1", time.Hour)
	c.Set("key2", "val2", time.Hour)
	c.Set("key3", "val3", time.Hour)

	// Add one more item - should evict randomly
	c.Set("key4", "val4", time.Hour)

	// Verify we have exactly 3 items
	count := 0
	if _, found := c.Get("key1"); found { count++ }
	if _, found := c.Get("key2"); found { count++ }
	if _, found := c.Get("key3"); found { count++ }
	if _, found := c.Get("key4"); found { count++ }
	assert.Equal(t, 3, count, "Cache should only have 3 items")
}

// TestNoEviction tests unlimited cache behavior
func TestNoEviction(t *testing.T) {
	t.Parallel()
	c := createTestCache(t, 0, nil) // Unlimited cache

	// Add many items
	for i := 0; i < 100; i++ { // Reduced from 1000 for faster parallel tests
		c.Set(string(rune(i)), i, time.Hour)
	}

	// Check a sample of items
	for i := 0; i < 100; i += 10 { // Check every 10th item
		_, found := c.Get(string(rune(i)))
		assert.True(t, found, "Item should exist")
	}
}


// --- Memory Tests ---

func TestMemoryUsage(t *testing.T) {
	t.Parallel()
	c := createTestCache(t, 0, nil)

	// Disable GC for accurate measurement
	defer debug.SetGCPercent(debug.SetGCPercent(-1))

	// Measure baseline memory
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Add items to cache
	const itemCount = 10000
	for i := 0; i < itemCount; i++ {
		c.Set(string(rune(i)), make([]byte, 1024), time.Hour) // 1KB items
	}

	// Measure memory after adding items
	runtime.ReadMemStats(&m2)
	used := m2.HeapInuse - m1.HeapInuse

	t.Logf("Memory used for %d items: %.2f MB", itemCount, float64(used)/1024/1024)
	assert.Less(t, used, uint64(itemCount*1200), "Memory usage should be <1.2KB per item") // Allowing 20% overhead
}

func TestMemoryReclamation(t *testing.T) {
	t.Parallel()
	c := createTestCache(t, 0, nil)

	// Add large items
	for i := 0; i < 1000; i++ {
		c.Set(string(rune(i)), make([]byte, 10*1024), 50*time.Millisecond) // 10KB items
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Force GC to reclaim memory
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	initial := m1.HeapInuse

	// Add new items
	for i := 0; i < 1000; i++ {
		c.Set(string(rune(i+1000)), make([]byte, 10*1024), time.Hour)
	}

	// Force GC and check memory
	runtime.GC()
	runtime.ReadMemStats(&m2)
	assert.Less(t, m2.HeapInuse, initial*2, "Memory should be reclaimed")
}

// --- Load Tests ---

func TestHighVolumeWrites(t *testing.T) {
	t.Parallel()
	c := createTestCache(t, 100000, EvictionPolicy.NewLRUPolicy())

	start := time.Now()
	const workers = 10
	const writesPerWorker = 10000

	var wg sync.WaitGroup
	wg.Add(workers)

	for w := 0; w < workers; w++ {
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < writesPerWorker; i++ {
				key := string(rune(workerID*writesPerWorker + i))
				c.Set(key, make([]byte, 100), time.Hour) // 100B items
			}
		}(w)
	}

	wg.Wait()
	duration := time.Since(start)

	t.Logf("Completed %d writes in %v (%.0f ops/sec)", 
		workers*writesPerWorker, 
		duration,
		float64(workers*writesPerWorker)/duration.Seconds())
}

func TestMixedReadWriteLoad(t *testing.T) {
	t.Parallel()
	c := createTestCache(t, 10000, EvictionPolicy.NewLRUPolicy())

	// Pre-populate cache
	for i := 0; i < 5000; i++ {
		c.Set(string(rune(i)), i, time.Hour)
	}

	const workers = 20
	const operations = 5000
	var wg sync.WaitGroup
	wg.Add(workers)

	start := time.Now()

	for w := 0; w < workers; w++ {
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < operations; i++ {
				key := string(rune(i % 10000))
				if i%4 == 0 { // 25% writes
					c.Set(key, i, time.Hour)
				} else { // 75% reads
					c.Get(key)
				}
			}
		}(w)
	}

	wg.Wait()
	duration := time.Since(start)

	t.Logf("Completed %d operations in %v (%.0f ops/sec)",
		workers*operations,
		duration,
		float64(workers*operations)/duration.Seconds())
}

// --- Race Condition Tests ---

func TestConcurrentEvictions(t *testing.T) {
	t.Parallel()
	c := createTestCache(t, 100, EvictionPolicy.NewRandomPolicy())

	var wg sync.WaitGroup
	const workers = 10

	wg.Add(workers * 2) // Readers and writers

	// Writers
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				c.Set(string(rune(i)), i, time.Millisecond*10)
			}
		}()
	}

	// Readers
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				c.Get(string(rune(i)))
			}
		}()
	}

	wg.Wait()
	assert.True(t, c.Size() <= 100, "Cache should respect size limit")
}

// --- Failure Mode Tests ---

func TestCacheFullBehavior(t *testing.T) {
	t.Parallel()
	c := createTestCache(t, 100, EvictionPolicy.NewLRUPolicy())

	// Fill cache
	for i := 0; i < 100; i++ {
		c.Set(string(rune(i)), i, time.Hour)
	}

	// Additional writes should succeed (with evictions)
	for i := 100; i < 200; i++ {
		c.Set(string(rune(i)), i, time.Hour)
	}

	assert.Equal(t, 100, c.Size(), "Cache should maintain size limit")
}

func TestZeroTTLBehavior(t *testing.T) {
	t.Parallel()
	c := createTestCache(t, 0, nil)

	c.Set("key1", "value1", 0)
	_, found := c.Get("key1")
	assert.True(t, found, "Items with zero TTL should persist")

	c.Set("key2", "value2", -1) // Negative TTL
	_, found = c.Get("key2")
	assert.True(t, found, "Items with negative TTL should persist")
}

// --- Benchmark Tests ---

func BenchmarkCacheGet(b *testing.B) {
	c := Store.CreateStore(0, nil)
	defer close(c.CloseChan)

	// Pre-populate cache
	for i := 0; i < 10000; i++ {
		c.Set(string(rune(i)), i, time.Hour)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(string(rune(i % 10000)))
	}
}

func BenchmarkCacheSet(b *testing.B) {
	c := Store.CreateStore(10000, EvictionPolicy.NewLRUPolicy())
	defer close(c.CloseChan)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(string(rune(i)), i, time.Hour)
	}
}

func BenchmarkMixedWorkload(b *testing.B) {
	c := Store.CreateStore(10000, EvictionPolicy.NewLRUPolicy())
	defer close(c.CloseChan)

	// Pre-populate
	for i := 0; i < 5000; i++ {
		c.Set(string(rune(i)), i, time.Hour)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%4 == 0 { // 25% writes
			c.Set(string(rune(i)), i, time.Hour)
		} else { // 75% reads
			c.Get(string(rune(i % 5000)))
		}
	}
}