package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kroma-labs/sentinel-go/httpserver"
	sentinelGin "github.com/kroma-labs/sentinel-go/httpserver/adapters/gin"
	"github.com/stretchr/testify/assert"
	"github.com/yunobar/album/internal/appconstant"
	"golang.org/x/time/rate"
)

func TestPerIPRateLimit(t *testing.T) {
	r := gin.New()
	r.POST("/auth/password-reset",
		sentinelGin.RateLimit(httpserver.RateLimitConfig{
			Limit:   rate.Limit(3.0 / 900),
			Burst:   3,
			KeyFunc: httpserver.KeyFuncByIP(),
		}),
		func(c *gin.Context) { c.Status(http.StatusCreated) },
	)

	for i := range 3 {
		req := httptest.NewRequest(http.MethodPost, "/auth/password-reset", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code, "request %d should pass", i+1)
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/password-reset", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// Different IP should still pass
	req = httptest.NewRequest(http.MethodPost, "/auth/password-reset", nil)
	req.RemoteAddr = "5.6.7.8:1234"
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestPerUserRateLimit(t *testing.T) {
	r := gin.New()
	// Simulate auth middleware setting profile ID
	r.Use(func(c *gin.Context) {
		c.Set(appconstant.ContextProfileID.String(), "user-123")
		c.Next()
	})

	profiles := r.Group("/profiles")
	profiles.Use(
		WithRateKey(appconstant.ContextProfileID.String()),
		sentinelGin.RateLimit(httpserver.RateLimitConfig{
			Limit:   rate.Limit(10.0 / 60),
			Burst:   3,
			KeyFunc: httpserver.KeyFuncByHeader("X-Rate-Key"),
		}),
	)
	profiles.GET("", func(c *gin.Context) { c.Status(http.StatusOK) })

	for i := range 3 {
		req := httptest.NewRequest(http.MethodGet, "/profiles", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "request %d should pass", i+1)
	}

	req := httptest.NewRequest(http.MethodGet, "/profiles", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestWithRateKey_StripsClientHeader(t *testing.T) {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("profile_id", "real-user")
		c.Next()
	})
	r.Use(WithRateKey("profile_id"))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, c.Request.Header.Get("X-Rate-Key"))
	})

	// Client sends a spoofed header — middleware should overwrite it
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Rate-Key", "spoofed-value")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, "real-user", w.Body.String())
}

func TestAuthGroupRateLimit(t *testing.T) {
	r := gin.New()
	auth := r.Group("/auth")
	auth.Use(sentinelGin.RateLimit(httpserver.RateLimitConfig{
		Limit:   rate.Limit(20.0 / 60),
		Burst:   5,
		KeyFunc: httpserver.KeyFuncByIP(),
	}))
	auth.POST("/login", func(c *gin.Context) { c.Status(http.StatusOK) })
	auth.POST("/register", func(c *gin.Context) { c.Status(http.StatusCreated) })

	for i := range 5 {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
		req.RemoteAddr = "10.0.0.1:9999"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "request %d should pass", i+1)
	}

	// 6th request from same IP should be rate limited
	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.RemoteAddr = "10.0.0.1:9999"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// Different endpoint, same IP — still limited (shared bucket per IP)
	req = httptest.NewRequest(http.MethodPost, "/auth/register", nil)
	req.RemoteAddr = "10.0.0.1:9999"
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}
