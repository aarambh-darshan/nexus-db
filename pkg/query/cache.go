package query

import (
	"container/list"
	"database/sql"
	"sync"
)

// StmtCache is an LRU cache for prepared statements.
type StmtCache struct {
	mu       sync.RWMutex
	capacity int
	items    map[string]*list.Element
	order    *list.List
	db       *sql.DB
}

// cacheEntry holds a cached prepared statement.
type cacheEntry struct {
	key  string
	stmt *sql.Stmt
}

// NewStmtCache creates a new prepared statement cache.
func NewStmtCache(db *sql.DB, capacity int) *StmtCache {
	if capacity <= 0 {
		capacity = 100
	}
	return &StmtCache{
		capacity: capacity,
		items:    make(map[string]*list.Element),
		order:    list.New(),
		db:       db,
	}
}

// Get retrieves a prepared statement from the cache or creates a new one.
func (c *StmtCache) Get(query string) (*sql.Stmt, error) {
	c.mu.RLock()
	elem, ok := c.items[query]
	c.mu.RUnlock()

	if ok {
		// Move to front (most recently used)
		c.mu.Lock()
		c.order.MoveToFront(elem)
		c.mu.Unlock()
		return elem.Value.(*cacheEntry).stmt, nil
	}

	// Prepare new statement
	stmt, err := c.db.Prepare(query)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if another goroutine added it
	if elem, ok := c.items[query]; ok {
		stmt.Close() // Close duplicate
		c.order.MoveToFront(elem)
		return elem.Value.(*cacheEntry).stmt, nil
	}

	// Evict if at capacity
	if c.order.Len() >= c.capacity {
		c.evict()
	}

	// Add to cache
	entry := &cacheEntry{key: query, stmt: stmt}
	elem = c.order.PushFront(entry)
	c.items[query] = elem

	return stmt, nil
}

// evict removes the least recently used statement.
func (c *StmtCache) evict() {
	elem := c.order.Back()
	if elem == nil {
		return
	}

	entry := elem.Value.(*cacheEntry)
	delete(c.items, entry.key)
	c.order.Remove(elem)
	entry.stmt.Close()
}

// Clear closes all cached statements and clears the cache.
func (c *StmtCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, elem := range c.items {
		entry := elem.Value.(*cacheEntry)
		entry.stmt.Close()
	}

	c.items = make(map[string]*list.Element)
	c.order = list.New()
}

// Size returns the current number of cached statements.
func (c *StmtCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Stats returns cache statistics.
type CacheStats struct {
	Size     int
	Capacity int
	Hits     int64
	Misses   int64
}

// StmtCacheWithStats adds hit/miss tracking to StmtCache.
type StmtCacheWithStats struct {
	*StmtCache
	mu     sync.RWMutex
	hits   int64
	misses int64
}

// NewStmtCacheWithStats creates a statement cache with statistics tracking.
func NewStmtCacheWithStats(db *sql.DB, capacity int) *StmtCacheWithStats {
	return &StmtCacheWithStats{
		StmtCache: NewStmtCache(db, capacity),
	}
}

// Get retrieves a statement, tracking hits and misses.
func (c *StmtCacheWithStats) Get(query string) (*sql.Stmt, error) {
	c.StmtCache.mu.RLock()
	_, ok := c.StmtCache.items[query]
	c.StmtCache.mu.RUnlock()

	c.mu.Lock()
	if ok {
		c.hits++
	} else {
		c.misses++
	}
	c.mu.Unlock()

	return c.StmtCache.Get(query)
}

// Stats returns the cache statistics.
func (c *StmtCacheWithStats) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CacheStats{
		Size:     c.Size(),
		Capacity: c.capacity,
		Hits:     c.hits,
		Misses:   c.misses,
	}
}

// HitRate returns the cache hit rate (0.0 to 1.0).
func (c *StmtCacheWithStats) HitRate() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	if total == 0 {
		return 0
	}
	return float64(c.hits) / float64(total)
}
