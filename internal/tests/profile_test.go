package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/entity"
	"github.com/yunobar/album/internal/testhelpers"
)

func TestGetProfile(t *testing.T) {
	testhelpers.RequireTestDB(t, testDB)

	t.Run("returns user profile", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/profile", nil)
		testRouter.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Data dto.ProfileResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, testProfileID, resp.Data.ID)
		assert.Equal(t, "Test User", resp.Data.Name)
	})
}

func TestUpdateProfile(t *testing.T) {
	testhelpers.RequireTestDB(t, testDB)

	t.Run("updates name successfully", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPatch, "/api/v1/profile", strings.NewReader(`{"name":"Updated Name"}`))
		req.Header.Set("Content-Type", "application/json")
		testRouter.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Data dto.ProfileResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "Updated Name", resp.Data.Name)
	})

	t.Run("400 validation error name too short", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPatch, "/api/v1/profile", strings.NewReader(`{"name":"ab"}`))
		req.Header.Set("Content-Type", "application/json")
		testRouter.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})
}

func seedTestUser(t *testing.T) {
	t.Helper()
	testDB.Create(&entity.User{
		BaseEntity: crud.BaseEntity{ID: testUserID},
		Email:      "test@example.com",
		Profile: entity.UserProfile{
			BaseEntity: crud.BaseEntity{ID: testProfileID},
			UserID:     uuid.NullUUID{UUID: testUserID, Valid: true},
			Name:       "Test User",
			Avatar:     "avatar.png",
		},
	})
}
