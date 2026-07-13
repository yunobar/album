package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/entity"
	"github.com/yunobar/album/internal/testhelpers"
)

func TestWatchlistAdd(t *testing.T) {
	testhelpers.RequireTestDB(t, testDB)

	t.Run("adding the same content twice conflicts; delete then re-add succeeds", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		content := seedTestContent(t)

		body := fmt.Sprintf(`{"contentId":%q,"priority":"high"}`, content.ID)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/watchlist", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		testRouter.ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code)

		var rows []entity.WatchlistItem
		require.NoError(t, testDB.Find(&rows).Error)
		require.Len(t, rows, 1)

		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest(http.MethodPost, "/api/v1/watchlist", strings.NewReader(body))
		req2.Header.Set("Content-Type", "application/json")
		testRouter.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusConflict, w2.Code)

		require.NoError(t, testDB.Find(&rows).Error)
		require.Len(t, rows, 1, "duplicate add must not create a second row")

		w3 := httptest.NewRecorder()
		req3, _ := http.NewRequest(http.MethodDelete, "/api/v1/watchlist/"+content.ID.String(), nil)
		testRouter.ServeHTTP(w3, req3)
		require.Equal(t, http.StatusNoContent, w3.Code)

		w4 := httptest.NewRecorder()
		req4, _ := http.NewRequest(http.MethodPost, "/api/v1/watchlist", strings.NewReader(body))
		req4.Header.Set("Content-Type", "application/json")
		testRouter.ServeHTTP(w4, req4)
		assert.Equal(t, http.StatusCreated, w4.Code, "re-adding after delete is a fresh insert")
	})

	t.Run("rejects an unknown content_id", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)

		body := fmt.Sprintf(`{"contentId":%q,"priority":"high"}`, uuid.New())

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/watchlist", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		testRouter.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("404 when deleting a content_id not on the watchlist", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/watchlist/"+uuid.New().String(), nil)
		testRouter.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("another profile's update/delete against the same content_id 404s and leaves the row untouched", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		content := seedTestContent(t)

		require.NoError(t, testDB.Create(&entity.WatchlistItem{
			ProfileID: testProfileID,
			ContentID: content.ID,
			Priority:  "low",
			Notes:     "owner's notes",
			Status:    "active",
		}).Error)

		otherProfileID := uuid.New()

		updateReq, _ := http.NewRequest(http.MethodPatch, "/api/v1/watchlist/"+content.ID.String(), strings.NewReader(`{"priority":"must"}`))
		updateReq.Header.Set("Content-Type", "application/json")
		updateReq.Header.Set(testProfileIDHeader, otherProfileID.String())
		w := httptest.NewRecorder()
		testRouter.ServeHTTP(w, updateReq)
		assert.Equal(t, http.StatusNotFound, w.Code, "update must be scoped by profile_id, not just content_id")

		deleteReq, _ := http.NewRequest(http.MethodDelete, "/api/v1/watchlist/"+content.ID.String(), nil)
		deleteReq.Header.Set(testProfileIDHeader, otherProfileID.String())
		w2 := httptest.NewRecorder()
		testRouter.ServeHTTP(w2, deleteReq)
		assert.Equal(t, http.StatusNotFound, w2.Code, "delete must be scoped by profile_id, not just content_id")

		var persisted entity.WatchlistItem
		require.NoError(t, testDB.First(&persisted, "content_id = ?", content.ID).Error)
		assert.Equal(t, "low", persisted.Priority)
		assert.Equal(t, "owner's notes", persisted.Notes)
	})
}

func TestWatchlistList(t *testing.T) {
	testhelpers.RequireTestDB(t, testDB)

	t.Run("returns only active items joined to content", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		active := seedTestContent(t)
		watched := seedTestContent(t)

		testDB.Create(&entity.WatchlistItem{
			ProfileID: testProfileID,
			ContentID: active.ID,
			Priority:  "must",
			Status:    "active",
		})
		testDB.Create(&entity.WatchlistItem{
			ProfileID: testProfileID,
			ContentID: watched.ID,
			Priority:  "low",
			Status:    "watched",
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/watchlist", nil)
		testRouter.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Data []dto.WatchlistItemResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Len(t, resp.Data, 1)
		assert.Equal(t, active.ID, resp.Data[0].Content.ID)
	})
}

func TestWatchlistUpdate(t *testing.T) {
	testhelpers.RequireTestDB(t, testDB)

	t.Run("updates priority and notes, and persists the change", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		content := seedTestContent(t)

		require.NoError(t, testDB.Create(&entity.WatchlistItem{
			ProfileID: testProfileID,
			ContentID: content.ID,
			Priority:  "low",
			Notes:     "original notes",
			Status:    "active",
		}).Error)

		body := `{"priority":"must","notes":"updated notes"}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPatch, "/api/v1/watchlist/"+content.ID.String(), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		testRouter.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Data dto.WatchlistItemResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "must", resp.Data.Priority)
		assert.Equal(t, "updated notes", resp.Data.Notes)
		assert.Equal(t, content.ID, resp.Data.Content.ID)

		var persisted entity.WatchlistItem
		require.NoError(t, testDB.First(&persisted, "content_id = ?", content.ID).Error)
		assert.Equal(t, "must", persisted.Priority)
		assert.Equal(t, "updated notes", persisted.Notes)
	})

	t.Run("404 when the watchlist item for that contentId does not exist", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)

		body := `{"priority":"must"}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPatch, "/api/v1/watchlist/"+uuid.New().String(), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		testRouter.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("priority-only update leaves notes untouched", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		content := seedTestContent(t)

		require.NoError(t, testDB.Create(&entity.WatchlistItem{
			ProfileID: testProfileID,
			ContentID: content.ID,
			Priority:  "low",
			Notes:     "original notes",
			Status:    "active",
		}).Error)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPatch, "/api/v1/watchlist/"+content.ID.String(), strings.NewReader(`{"priority":"must"}`))
		req.Header.Set("Content-Type", "application/json")
		testRouter.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Data dto.WatchlistItemResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "must", resp.Data.Priority)
		assert.Equal(t, "original notes", resp.Data.Notes)

		var persisted entity.WatchlistItem
		require.NoError(t, testDB.First(&persisted, "content_id = ?", content.ID).Error)
		assert.Equal(t, "must", persisted.Priority)
		assert.Equal(t, "original notes", persisted.Notes)
	})

	t.Run("notes-only update leaves priority untouched", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		content := seedTestContent(t)

		require.NoError(t, testDB.Create(&entity.WatchlistItem{
			ProfileID: testProfileID,
			ContentID: content.ID,
			Priority:  "low",
			Notes:     "original notes",
			Status:    "active",
		}).Error)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPatch, "/api/v1/watchlist/"+content.ID.String(), strings.NewReader(`{"notes":"updated notes"}`))
		req.Header.Set("Content-Type", "application/json")
		testRouter.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Data dto.WatchlistItemResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "low", resp.Data.Priority)
		assert.Equal(t, "updated notes", resp.Data.Notes)

		var persisted entity.WatchlistItem
		require.NoError(t, testDB.First(&persisted, "content_id = ?", content.ID).Error)
		assert.Equal(t, "low", persisted.Priority)
		assert.Equal(t, "updated notes", persisted.Notes)
	})
}

func seedTestContent(t *testing.T) entity.Content {
	t.Helper()
	content := entity.Content{
		Source:      "tmdb",
		SourceID:    uuid.NewString(),
		ContentType: "movie",
		Title:       "Test Movie",
		Metadata:    json.RawMessage(`{}`),
	}
	require.NoError(t, testDB.Create(&content).Error)
	return content
}
