package cache

import (
	"sync"
	"time"

	"github.com/yunobar/album/internal/core/logger"
)

type Cache[T any] interface {
	Get(key string, fallbackFunc func(string) (T, bool)) (T, bool)
	Delete(key string)
	Shutdown() error
}

type cacheEntry[T any] struct {
	value     T
	expiresAt time.Time
}

type inmemoryCache[T any] struct {
	data   *sync.Map
	stopCh chan struct{}
	wg     sync.WaitGroup
	expiry time.Duration
}

func NewInMemoryCache[T any](expiry time.Duration) Cache[T] {
	cache := &inmemoryCache[T]{
		data:   new(sync.Map),
		stopCh: make(chan struct{}),
		expiry: expiry,
	}
	cache.startCleanup()
	return cache
}

func (c *inmemoryCache[T]) Get(key string, fallbackFunc func(string) (T, bool)) (T, bool) {
	var zero T
	val, exists := c.getValue(key)
	if exists {
		return val, true
	}
	if fallbackFunc == nil {
		return zero, false
	}
	fallbackVal, ok := fallbackFunc(key)
	if !ok {
		return zero, false
	}
	entry := cacheEntry[T]{
		value:     fallbackVal,
		expiresAt: time.Now().Add(c.expiry),
	}
	c.data.Store(key, entry)
	return fallbackVal, true
}

func (c *inmemoryCache[T]) Delete(key string) {
	c.data.Delete(key)
}

func (c *inmemoryCache[T]) Shutdown() error {
	close(c.stopCh)
	c.wg.Wait()
	return nil
}

func (c *inmemoryCache[T]) getValue(key string) (T, bool) {
	var zero T
	value, loaded := c.data.Load(key)
	if !loaded {
		return zero, false
	}

	entry, ok := value.(cacheEntry[T])
	if !ok {
		logger.Errorf("cache value is not cacheEntry, instead: %T", value)
		c.data.Delete(key)
		return zero, false
	}

	if time.Now().After(entry.expiresAt) {
		c.data.Delete(key)
		return zero, false
	}

	return entry.value, true
}

func (c *inmemoryCache[T]) startCleanup() {
	c.wg.Go(func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.cleanup()
			case <-c.stopCh:
				return
			}
		}
	})
}

func (c *inmemoryCache[T]) cleanup() {
	now := time.Now()
	c.data.Range(func(key, value any) bool {
		entry, ok := value.(cacheEntry[T])
		if !ok {
			c.data.Delete(key)
			return true
		}
		if now.After(entry.expiresAt) {
			c.data.Delete(key)
		}
		return true
	})
}
