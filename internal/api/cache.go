package api

import (
	"sync"
	"time"
)

// StatsCache provides in-memory caching for statistics with TTL
type StatsCache struct {
	mu        sync.RWMutex
	data      interface{}
	expiresAt time.Time
	ttl       time.Duration
}

// NewStatsCache creates a new stats cache with specified TTL
func NewStatsCache(ttl time.Duration) *StatsCache {
	return &StatsCache{
		ttl: ttl,
	}
}

// Get retrieves cached stats if not expired
func (c *StatsCache) Get() (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if time.Now().Before(c.expiresAt) && c.data != nil {
		return c.data, true
	}
	return nil, false
}

// Set stores stats in cache with expiration
func (c *StatsCache) Set(data interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = data
	c.expiresAt = time.Now().Add(c.ttl)
}

// Invalidate clears the cache
func (c *StatsCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = nil
	c.expiresAt = time.Time{}
}
