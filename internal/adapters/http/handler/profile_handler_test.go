package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/mocks"
)

func TestProfileHandler_HandleProfile(t *testing.T) {
	profileID := uuid.New()
	svc := mocks.NewMockProfileService(t)
	h := NewProfileHandler(svc)

	svc.On("GetByID", mock.Anything, profileID).
		Return(dto.ProfileResponse{BaseDTO: dto.BaseDTO{ID: profileID}, Name: "Alice"}, nil)

	r := gin.New()
	r.GET("/profile", injectProfileID(profileID), h.HandleProfile())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/profile", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Alice")
}

func TestProfileHandler_HandleUpdate(t *testing.T) {
	profileID := uuid.New()

	t.Run("200 updates name", func(t *testing.T) {
		svc := mocks.NewMockProfileService(t)
		h := NewProfileHandler(svc)

		svc.On("Update", mock.Anything, mock.Anything).
			Return(dto.ProfileResponse{BaseDTO: dto.BaseDTO{ID: profileID}, Name: "Bob"}, nil)

		r := gin.New()
		r.PATCH("/profile", injectProfileID(profileID), h.HandleUpdate())

		body := `{"name":"Bob"}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPatch, "/profile", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Contains(t, string(resp["data"]), "Bob")
	})

	t.Run("422 validation error name too short", func(t *testing.T) {
		svc := mocks.NewMockProfileService(t)
		h := NewProfileHandler(svc)

		r := gin.New()
		r.Use(testErrorMiddleware())
		r.PATCH("/profile", injectProfileID(profileID), h.HandleUpdate())

		body := `{"name":"ab"}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPatch, "/profile", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})
}
