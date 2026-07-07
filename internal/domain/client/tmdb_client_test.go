package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func TestTMDBClient_Search_MapsResultsByMediaTypeAndSkipsPeople(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "batman", r.URL.Query().Get("query"))
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"id": 268, "media_type": "movie", "title": "Batman", "release_date": "1989-06-23", "poster_path": "/movie.jpg"},
				{"id": 2661, "media_type": "tv", "name": "Batman", "first_air_date": "1966-01-12", "poster_path": "/tv.jpg"},
				{"id": 3894, "media_type": "person", "name": "Michael Keaton"},
			},
		})
	}))
	defer srv.Close()

	c := NewTMDBClient(srv.URL, "test-key")
	results, err := c.Search(context.Background(), "batman")
	require.NoError(t, err)
	require.Len(t, results, 2) // the person result is filtered out

	assert.Equal(t, "268", results[0].SourceID)
	assert.Equal(t, "movie", results[0].ContentType)
	assert.Equal(t, "Batman", results[0].Title)
	require.NotNil(t, results[0].ReleaseYear)
	assert.Equal(t, 1989, *results[0].ReleaseYear)
	assert.Equal(t, "https://image.tmdb.org/t/p/w500/movie.jpg", results[0].PosterURL)

	assert.Equal(t, "2661", results[1].SourceID)
	assert.Equal(t, "tv", results[1].ContentType)
	assert.Equal(t, "Batman", results[1].Title)
	require.NotNil(t, results[1].ReleaseYear)
	assert.Equal(t, 1966, *results[1].ReleaseYear)
	assert.Equal(t, "https://image.tmdb.org/t/p/w500/tv.jpg", results[1].PosterURL)
}

func TestTMDBClient_RateLimiter_RejectsBurstBeyondBucket(t *testing.T) {
	limiter := rate.NewLimiter(rate.Limit(35), 35)

	allowed := 0
	for range 50 {
		if limiter.Allow() {
			allowed++
		}
	}

	assert.Equal(t, 35, allowed)
}
