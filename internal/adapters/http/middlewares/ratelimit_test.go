package middlewares

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

func TestValueLimiter_Allow(t *testing.T) {
	limiter := NewValueLimiter(rate.Limit(1.0/3600), 3, time.Hour)
	defer limiter.Stop()

	assert.True(t, limiter.Allow("test@x.com"))
	assert.True(t, limiter.Allow("test@x.com"))
	assert.True(t, limiter.Allow("test@x.com"))
	assert.False(t, limiter.Allow("test@x.com"))

	// Different key should have its own bucket
	assert.True(t, limiter.Allow("other@x.com"))
}

func TestValueLimiter_Eviction(t *testing.T) {
	limiter := NewValueLimiter(rate.Limit(1.0/3600), 1, 50*time.Millisecond)
	defer limiter.Stop()

	limiter.Allow("evict@x.com")

	time.Sleep(100 * time.Millisecond)

	limiter.mu.Lock()
	_, exists := limiter.entries["evict@x.com"]
	limiter.mu.Unlock()
	assert.False(t, exists)
}
