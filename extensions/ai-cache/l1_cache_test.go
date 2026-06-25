package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStructuredL1CacheAllowsRepeatedReadsBeforeTTL(t *testing.T) {
	cache := newStructuredL1Cache(2, 1)
	require.NotNil(t, cache)

	entry := StructuredCacheEntry{
		Version:            StructuredCacheEntryVersion,
		Answer:             StructuredCacheAnswer{Role: "assistant", Content: "cached answer"},
		Scope:              StructuredCacheScope{Type: "global"},
		RequestFingerprint: RequestFingerprint{Hash: "req"},
		SoftExpiresAt:      time.Now().Add(time.Minute).Unix(),
		HardExpiresAt:      time.Now().Add(2 * time.Minute).Unix(),
	}

	cache.Put("entry-key", entry)

	first, ok := cache.Get("entry-key")
	require.True(t, ok)
	require.Equal(t, "cached answer", first.Answer.Content)

	second, ok := cache.Get("entry-key")
	require.True(t, ok)
	require.Equal(t, "cached answer", second.Answer.Content)
}

func TestStructuredL1CacheExpiresEntriesAfterTTL(t *testing.T) {
	cache := newStructuredL1Cache(2, 1)
	require.NotNil(t, cache)

	entry := StructuredCacheEntry{
		Version:            StructuredCacheEntryVersion,
		Answer:             StructuredCacheAnswer{Role: "assistant", Content: "cached answer"},
		Scope:              StructuredCacheScope{Type: "global"},
		RequestFingerprint: RequestFingerprint{Hash: "req"},
		SoftExpiresAt:      time.Now().Add(time.Minute).Unix(),
		HardExpiresAt:      time.Now().Add(2 * time.Minute).Unix(),
	}

	cache.Put("entry-key", entry)
	time.Sleep(1100 * time.Millisecond)

	_, ok := cache.Get("entry-key")
	require.False(t, ok)
}
