package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/yunobar/album/internal/domain/repository"
)

func TestMergeRows(t *testing.T) {
	movieID := uuid.New()
	showID := uuid.New()
	aliceID := uuid.New()
	bobID := uuid.New()
	year := 2020

	t.Run("empty input yields empty output", func(t *testing.T) {
		got := mergeRows(nil)

		assert.Empty(t, got)
	})

	t.Run("dedups rows for the same content across members", func(t *testing.T) {
		rows := []repository.MergedWatchlistRow{
			{
				ContentID:   movieID,
				ProfileID:   aliceID,
				Priority:    "high",
				ContentType: "movie",
				Title:       "Movie A",
				ReleaseYear: &year,
				PosterURL:   "poster-a.png",
			},
			{
				ContentID:   movieID,
				ProfileID:   bobID,
				Priority:    "low",
				ContentType: "movie",
				Title:       "Movie A",
				ReleaseYear: &year,
				PosterURL:   "poster-a.png",
			},
		}

		got := mergeRows(rows)

		assert.Len(t, got, 1)
		item := got[0]
		assert.Equal(t, movieID, item.Content.ID)
		assert.Equal(t, "Movie A", item.Content.Title)
		assert.Equal(t, "movie", item.Content.ContentType)
		assert.Equal(t, &year, item.Content.ReleaseYear)
		assert.Equal(t, "poster-a.png", item.Content.PosterUrl)
		assert.Equal(t, 2, item.InterestedCount)
		assert.ElementsMatch(t, []uuid.UUID{aliceID, bobID}, item.Members)
		assert.Equal(t, map[string]string{
			aliceID.String(): "high",
			bobID.String():   "low",
		}, item.Priorities)
	})

	t.Run("orders by interested count desc then title asc on ties", func(t *testing.T) {
		rows := []repository.MergedWatchlistRow{
			// "Zeta" has 1 interested member.
			{ContentID: showID, ProfileID: aliceID, Priority: "high", ContentType: "tv", Title: "Zeta"},
			// "Movie A" has 2 interested members, so it should sort first
			// despite "Movie A" < "Zeta" alphabetically.
			{ContentID: movieID, ProfileID: aliceID, Priority: "high", ContentType: "movie", Title: "Movie A"},
			{ContentID: movieID, ProfileID: bobID, Priority: "low", ContentType: "movie", Title: "Movie A"},
		}

		got := mergeRows(rows)

		assert.Len(t, got, 2)
		assert.Equal(t, movieID, got[0].Content.ID)
		assert.Equal(t, showID, got[1].Content.ID)
	})

	t.Run("breaks ties on interested count by title ascending", func(t *testing.T) {
		rows := []repository.MergedWatchlistRow{
			{ContentID: showID, ProfileID: aliceID, Priority: "high", ContentType: "tv", Title: "Zeta"},
			{ContentID: movieID, ProfileID: bobID, Priority: "low", ContentType: "movie", Title: "Alpha"},
		}

		got := mergeRows(rows)

		assert.Len(t, got, 2)
		assert.Equal(t, "Alpha", got[0].Content.Title)
		assert.Equal(t, "Zeta", got[1].Content.Title)
	})
}
