package main

import (
	"sync"
	"time"
)

type structuredL1Cache struct {
	mu         sync.Mutex
	maxEntries int
	ttlSeconds int
	entries    map[string]structuredL1CacheEntry
	order      []string
}

type structuredL1CacheEntry struct {
	entry     StructuredCacheEntry
	expiresAt time.Time
}

func newStructuredL1Cache(maxEntries int, ttlSeconds int) *structuredL1Cache {
	if maxEntries <= 0 || ttlSeconds <= 0 {
		return nil
	}
	return &structuredL1Cache{
		maxEntries: maxEntries,
		ttlSeconds: ttlSeconds,
		entries:    map[string]structuredL1CacheEntry{},
	}
}

func (c *structuredL1Cache) Get(key string) (StructuredCacheEntry, bool) {
	if c == nil || key == "" {
		return StructuredCacheEntry{}, false
	}

	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.entries[key]
	if !ok {
		return StructuredCacheEntry{}, false
	}
	if c.entryExpiredLocked(item, now) {
		c.deleteLocked(key)
		return StructuredCacheEntry{}, false
	}
	return item.entry, true
}

func (c *structuredL1Cache) Put(key string, entry StructuredCacheEntry) {
	if c == nil || key == "" || c.maxEntries <= 0 || c.ttlSeconds <= 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()

	if _, exists := c.entries[key]; !exists {
		c.order = append(c.order, key)
	}
	c.entries[key] = structuredL1CacheEntry{
		entry:     entry,
		expiresAt: now.Add(time.Duration(c.ttlSeconds) * time.Second),
	}
	c.evictLocked()
}

func (c *structuredL1Cache) Delete(key string) {
	if c == nil || key == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deleteLocked(key)
}

func (c *structuredL1Cache) evictLocked() {
	for len(c.entries) > c.maxEntries && len(c.order) > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.entries, oldest)
	}
}

func (c *structuredL1Cache) entryExpiredLocked(entry structuredL1CacheEntry, now time.Time) bool {
	if !entry.expiresAt.IsZero() && !now.Before(entry.expiresAt) {
		return true
	}
	return false
}

func (c *structuredL1Cache) deleteLocked(key string) {
	delete(c.entries, key)
	for idx, orderedKey := range c.order {
		if orderedKey == key {
			c.order = append(c.order[:idx], c.order[idx+1:]...)
			return
		}
	}
}
