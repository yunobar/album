package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCache_Get_MissWithFallback(t *testing.T) {
	c := NewInMemoryCache[string](1 * time.Minute)
	defer func() { _ = c.Shutdown() }()

	val, ok := c.Get("key1", func(k string) (string, bool) {
		return "value1", true
	})

	assert.True(t, ok)
	assert.Equal(t, "value1", val)
}

func TestCache_Get_Hit(t *testing.T) {
	c := NewInMemoryCache[string](1 * time.Minute)
	defer func() { _ = c.Shutdown() }()

	// Fill cache
	c.Get("key1", func(k string) (string, bool) {
		return "value1", true
	})

	// Hit — fallback should NOT be called
	called := false
	val, ok := c.Get("key1", func(k string) (string, bool) {
		called = true
		return "other", true
	})

	assert.True(t, ok)
	assert.Equal(t, "value1", val)
	assert.False(t, called)
}

func TestCache_Get_MissWithNilFallback(t *testing.T) {
	c := NewInMemoryCache[string](1 * time.Minute)
	defer func() { _ = c.Shutdown() }()

	val, ok := c.Get("key1", nil)

	assert.False(t, ok)
	assert.Equal(t, "", val)
}

func TestCache_Get_Expired(t *testing.T) {
	c := NewInMemoryCache[string](1 * time.Millisecond)
	defer func() { _ = c.Shutdown() }()

	// Fill cache
	c.Get("key1", func(k string) (string, bool) {
		return "old", true
	})

	// Wait for expiry
	time.Sleep(5 * time.Millisecond)

	// Should miss and call fallback
	val, ok := c.Get("key1", func(k string) (string, bool) {
		return "new", true
	})

	assert.True(t, ok)
	assert.Equal(t, "new", val)
}

func TestCache_Delete(t *testing.T) {
	c := NewInMemoryCache[string](1 * time.Minute)
	defer func() { _ = c.Shutdown() }()

	c.Get("key1", func(k string) (string, bool) {
		return "value1", true
	})

	c.Delete("key1")

	// Next Get is a miss
	val, ok := c.Get("key1", func(k string) (string, bool) {
		return "refilled", true
	})

	assert.True(t, ok)
	assert.Equal(t, "refilled", val)
}

func TestCache_Shutdown(t *testing.T) {
	c := NewInMemoryCache[string](1 * time.Minute)
	err := c.Shutdown()
	assert.NoError(t, err)
}
