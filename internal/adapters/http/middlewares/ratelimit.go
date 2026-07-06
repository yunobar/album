package middlewares

import (
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func WithRateKey(keyFromCtx string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Header.Del("X-Rate-Key")
		if val, exists := c.Get(keyFromCtx); exists {
			c.Request.Header.Set("X-Rate-Key", fmt.Sprint(val))
		}
		c.Next()
	}
}

type ValueLimiter struct {
	mu      sync.Mutex
	entries map[string]*limiterEntry
	limit   rate.Limit
	burst   int
	ttl     time.Duration
	stop    chan struct{}
}

type limiterEntry struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

func NewValueLimiter(limit rate.Limit, burst int, ttl time.Duration) *ValueLimiter {
	vl := &ValueLimiter{
		entries: make(map[string]*limiterEntry),
		limit:   limit,
		burst:   burst,
		ttl:     ttl,
		stop:    make(chan struct{}),
	}
	go vl.cleanup()
	return vl
}

func (vl *ValueLimiter) Stop() {
	close(vl.stop)
}

func (vl *ValueLimiter) Allow(key string) bool {
	vl.mu.Lock()
	defer vl.mu.Unlock()

	e, exists := vl.entries[key]
	if !exists {
		e = &limiterEntry{limiter: rate.NewLimiter(vl.limit, vl.burst)}
		vl.entries[key] = e
	}
	e.lastAccess = time.Now()
	return e.limiter.Allow()
}

func (vl *ValueLimiter) cleanup() {
	ticker := time.NewTicker(vl.ttl)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			vl.mu.Lock()
			now := time.Now()
			for k, e := range vl.entries {
				if now.Sub(e.lastAccess) > vl.ttl {
					delete(vl.entries, k)
				}
			}
			vl.mu.Unlock()
		case <-vl.stop:
			return
		}
	}
}
