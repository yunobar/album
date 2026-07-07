package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/itsLeonB/ungerr"
	"github.com/stretchr/testify/assert"
	"github.com/yunobar/album/internal/core/logger"
)

func TestNewTurnstileClient_EmptyKey_ReturnsNoop(t *testing.T) {
	logger.Init("album-test")
	c := NewTurnstileClient("")
	err := c.Verify(context.Background(), "any-token")
	assert.NoError(t, err)
}

func TestTurnstileClient_Verify_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-secret", r.FormValue("secret"))
		assert.Equal(t, "valid-token", r.FormValue("response"))
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}))
	defer srv.Close()

	c := NewTurnstileClientWithURL("test-secret", srv.URL)
	err := c.Verify(context.Background(), "valid-token")
	assert.NoError(t, err)
}

func TestTurnstileClient_Verify_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": false})
	}))
	defer srv.Close()

	c := NewTurnstileClientWithURL("test-secret", srv.URL)
	err := c.Verify(context.Background(), "invalid-token")

	assert.Error(t, err)
	var appErr ungerr.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, "captcha verification failed", appErr.Details())
}

func TestTurnstileClient_Verify_NetworkError(t *testing.T) {
	c := NewTurnstileClientWithURL("test-secret", "http://localhost:1")
	err := c.Verify(context.Background(), "token")
	assert.Error(t, err)
}

func TestTurnstileClient_Verify_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewTurnstileClientWithURL("test-secret", srv.URL)
	err := c.Verify(context.Background(), "token")
	assert.Error(t, err)
}
