package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/itsLeonB/ungerr"
	"github.com/stretchr/testify/assert"
)

func TestNewTurnstileService_EmptyKey_ReturnsNoop(t *testing.T) {
	svc := NewTurnstileService("")
	err := svc.Verify(context.Background(), "any-token")
	assert.NoError(t, err)
}

func TestTurnstileService_Verify_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-secret", r.FormValue("secret"))
		assert.Equal(t, "valid-token", r.FormValue("response"))
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}))
	defer srv.Close()

	svc := NewTurnstileServiceWithURL("test-secret", srv.URL)
	err := svc.Verify(context.Background(), "valid-token")
	assert.NoError(t, err)
}

func TestTurnstileService_Verify_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": false})
	}))
	defer srv.Close()

	svc := NewTurnstileServiceWithURL("test-secret", srv.URL)
	err := svc.Verify(context.Background(), "invalid-token")

	assert.Error(t, err)
	var appErr ungerr.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, "captcha verification failed", appErr.Details())
}

func TestTurnstileService_Verify_NetworkError(t *testing.T) {
	svc := NewTurnstileServiceWithURL("test-secret", "http://localhost:1")
	err := svc.Verify(context.Background(), "token")
	assert.Error(t, err)
}

func TestTurnstileService_Verify_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	svc := NewTurnstileServiceWithURL("test-secret", srv.URL)
	err := svc.Verify(context.Background(), "token")
	assert.Error(t, err)
}
