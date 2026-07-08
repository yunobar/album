package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/itsLeonB/go-crud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/yunobar/album/internal/domain/client"
	"github.com/yunobar/album/internal/domain/entity"
	"github.com/yunobar/album/internal/domain/service"
	"github.com/yunobar/album/internal/mocks"
	"github.com/yunobar/album/internal/testhelpers"
)

func TestContentSearch(t *testing.T) {
	testhelpers.RequireTestDB(t, testDB)

	t.Run("dedups repeated searches that resolve to the same tmdb id", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)

		mockTMDB := mocks.NewMockTMDBClient(t)
		currentContentService = service.NewContentService(
			crud.NewTransactor(testDB),
			crud.NewRepository[entity.Content](testDB),
			mockTMDB,
		)

		mockTMDB.EXPECT().Search(mock.Anything, "batman").Return([]client.TMDBResult{
			{SourceID: "123", ContentType: "movie", Title: "Batman", Metadata: json.RawMessage(`{"v":1}`)},
		}, nil).Once()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/content/search?q=batman", nil)
		testRouter.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var rows []entity.Content
		require.NoError(t, testDB.Find(&rows).Error)
		require.Len(t, rows, 1)
		firstID := rows[0].ID

		mockTMDB.EXPECT().Search(mock.Anything, "the batman").Return([]client.TMDBResult{
			{SourceID: "123", ContentType: "movie", Title: "Batman", Metadata: json.RawMessage(`{"v":2}`)},
		}, nil).Once()

		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest(http.MethodGet, "/api/v1/content/search?q=the+batman", nil)
		testRouter.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)

		require.NoError(t, testDB.Find(&rows).Error)
		require.Len(t, rows, 1)
		assert.Equal(t, firstID, rows[0].ID)
		assert.JSONEq(t, `{"v":2}`, string(rows[0].Metadata))
	})
}
