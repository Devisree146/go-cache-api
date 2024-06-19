package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// Cache defines the unified cache interface.
type Cache interface {
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Get(ctx context.Context, key string) (interface{}, error)
	Delete(ctx context.Context, key string) error
	AsyncSet(ctx context.Context, key string, value interface{}, ttl time.Duration)
	AsyncDelete(ctx context.Context, key string)
}

// InMemoryCache implements Cache using an in-memory map with LRU eviction.
type InMemoryCache struct {
	maxSize int
	cache   map[string]*entry
	lruList *list
	lock    sync.Mutex
}

// entry represents a key-value pair in the in-memory cache.
type entry struct {
	key   string
	value interface{}
	ttl   time.Time
}

// list implements a doubly linked list for LRU eviction.
type list struct {
	head *node
	tail *node
	size int
}

// node represents a node in the linked list.
type node struct {
	prev *node
	next *node
	entr *entry
}

// NewInMemoryCache creates a new instance of InMemoryCache.
func NewInMemoryCache(maxSize int) *InMemoryCache {
	return &InMemoryCache{
		maxSize: maxSize,
		cache:   make(map[string]*entry),
		lruList: &list{},
	}
}

// Set adds or updates a key-value pair in the cache and handles LRU eviction.
func (c *InMemoryCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	// If the key already exists, update the value and TTL, and move it to the front.
	if ent, exists := c.cache[key]; exists {
		c.updateEntry(ent, value, ttl)
		c.lruList.moveToFront(ent)
		return nil
	}

	// If the cache is at its maximum size, evict the least recently used element.
	if len(c.cache) >= c.maxSize {
		c.evict()
	}

	// Add the new key-value pair to the cache.
	newEntry := &entry{
		key:   key,
		value: value,
		ttl:   time.Now().Add(ttl),
	}
	c.lruList.pushFront(newEntry)
	c.cache[key] = newEntry

	return nil
}

// Get retrieves the value for a key from the cache and updates its position in the LRU list.
func (c *InMemoryCache) Get(ctx context.Context, key string) (interface{}, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	// Check if the key exists in the cache.
	if ent, exists := c.cache[key]; exists {
		// Check if the entry has expired.
		if ent.ttl.After(time.Now()) {
			c.lruList.moveToFront(ent)
			return ent.value, nil
		}
		// If the entry has expired, remove it.
		c.removeEntry(ent)
	}

	return nil, errors.New("key not found")
}

// Delete removes a key from the cache.
func (c *InMemoryCache) Delete(ctx context.Context, key string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if ent, exists := c.cache[key]; exists {
		c.removeEntry(ent)
		return nil
	}

	return errors.New("key not found")
}

// AsyncSet asynchronously sets a key-value pair in the cache.
func (c *InMemoryCache) AsyncSet(ctx context.Context, key string, value interface{}, ttl time.Duration) {
	go func() {
		_ = c.Set(ctx, key, value, ttl)
	}()
}

// AsyncDelete asynchronously deletes a key from the cache.
func (c *InMemoryCache) AsyncDelete(ctx context.Context, key string) {
	go func() {
		_ = c.Delete(ctx, key)
	}()
}

// updateEntry updates the value and TTL of an existing entry.
func (c *InMemoryCache) updateEntry(ent *entry, value interface{}, ttl time.Duration) {
	ent.value = value
	ent.ttl = time.Now().Add(ttl)
}

// removeEntry removes an entry from the cache.
func (c *InMemoryCache) removeEntry(ent *entry) {
	c.lruList.remove(ent)
	delete(c.cache, ent.key)
}

// evict removes the least recently used entry from the cache.
func (c *InMemoryCache) evict() {
	ent := c.lruList.back()
	if ent != nil {
		fmt.Printf("Evicting value: %v\n", ent.value)
		c.removeEntry(ent)
	}
}

// list methods for LRU operations

// pushFront adds a new entry to the front of the list.
func (l *list) pushFront(ent *entry) {
	node := &node{entr: ent}
	if l.head == nil {
		l.head = node
		l.tail = node
	} else {
		node.next = l.head
		l.head.prev = node
		l.head = node
	}
	l.size++
}

// moveToFront moves an existing entry to the front of the list.
func (l *list) moveToFront(ent *entry) {
	node := &node{entr: ent}
	if ent == l.head {
		return
	}
	if ent == l.tail {
		l.tail = ent.prev
		ent.prev.next = nil
	} else {
		ent.prev.next = ent.next
		ent.next.prev = ent.prev
	}
	node.next = l.head
	l.head.prev = node
	l.head = node
}

// remove removes an entry from the list.
func (l *list) remove(ent *entry) {
	node := &node{entr: ent}
	if ent == l.head {
		l.head = ent.next
	} else {
		ent.prev.next = ent.next
	}
	if ent == l.tail {
		l.tail = ent.prev
	} else {
		ent.next.prev = ent.prev
	}
	l.size--
}

// back returns the last entry in the list.
func (l *list) back() *entry {
	return l.tail.entr
}

// RedisCache implements Cache using Redis.
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache creates a new RedisCache instance.
func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{client: client}
}

// Set adds or updates a key-value pair in Redis with the specified TTL.
func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

// Get retrieves the value associated with the given key from Redis.
func (c *RedisCache) Get(ctx context.Context, key string) (interface{}, error) {
	return c.client.Get(ctx, key).Result()
}

// Delete removes the entry associated with the given key from Redis.
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// AsyncSet asynchronously sets a key-value pair in Redis.
func (c *RedisCache) AsyncSet(ctx context.Context, key string, value interface{}, ttl time.Duration) {
	go func() {
		_ = c.Set(ctx, key, value, ttl)
	}()
}

// AsyncDelete asynchronously deletes a key from Redis.
func (c *RedisCache) AsyncDelete(ctx context.Context, key string) {
	go func() {
		_ = c.Delete(ctx, key)
	}()
}

func main() {
	// Example usage of InMemoryCache
	cache := NewInMemoryCache(3)
	cache.Set(context.Background(), "key1", 10, 5*time.Minute)
	cache.Set(context.Background(), "key2", 20, 5*time.Minute)

	// Example usage of RedisCache
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	redisCache := NewRedisCache(rdb)
	redisCache.Set(context.Background(), "key3", 30, 5*time.Minute)

	// Use WaitGroup to wait for all asynchronous operations to complete
	var wg sync.WaitGroup

	// Example WaitGroup usage for InMemoryCache
	wg.Add(2) // Number of asynchronous operations in InMemoryCache
	cache.AsyncSet(context.Background(), "asyncKey1", 100, 10*time.Minute)
	cache.AsyncDelete(context.Background(), "key2")

	// Example WaitGroup usage for RedisCache
	wg.Add(1) // Number of asynchronous operations in RedisCache
	redisCache.AsyncSet(context.Background(), "asyncKey3", 300, 10*time.Minute)

	// Wait for all goroutines to finish
	wg.Wait()

	// After waiting, perform Get operations to see results
	val1, _ := cache.Get(context.Background(), "key1")
	fmt.Println("Value of 'key1' from InMemoryCache:", val1)

	val2, _ := redisCache.Get(context.Background(), "key3")
	fmt.Println("Value of 'key3' from RedisCache:", val2)

	// Delete keys and wait for asynchronous delete operations to complete
	cache.Delete(context.Background(), "key1")
	redisCache.Delete(context.Background(), "key3")

	// Close Redis client connection
	if err := rdb.Close(); err != nil {
		fmt.Println("Error closing Redis client:", err)
	}
}
