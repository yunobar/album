package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/itsLeonB/ungerr"
	"github.com/yunobar/album/internal/appconstant"
	"golang.org/x/time/rate"
)

type TMDBResult struct {
	SourceID    string
	ContentType string
	Title       string
	ReleaseYear *int
	PosterURL   string
	Metadata    json.RawMessage
}

type TMDBClient interface {
	Search(ctx context.Context, query string) ([]TMDBResult, error)
}

const tmdbPosterBaseURL = "https://image.tmdb.org/t/p/w500"

type tmdbClient struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	limiter    *rate.Limiter
}

func NewTMDBClient(baseURL, apiKey string) TMDBClient {
	return &tmdbClient{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		baseURL:    baseURL,
		apiKey:     apiKey,
		// ponytail: single-process bucket, move to a shared/Redis limiter if the app scales to multiple instances (TMDB's cap is per API key, not per instance)
		limiter: rate.NewLimiter(rate.Limit(35), 35),
	}
}

func (c *tmdbClient) Search(ctx context.Context, query string) ([]TMDBResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := c.limiter.Wait(ctx); err != nil {
		return nil, ungerr.Wrap(err, appconstant.ErrTMDBSearchFailed)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/search/multi", nil)
	if err != nil {
		return nil, ungerr.Wrap(err, appconstant.ErrTMDBSearchFailed)
	}
	q := req.URL.Query()
	q.Set("query", query)
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, ungerr.Wrap(err, appconstant.ErrTMDBSearchFailed)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, ungerr.Wrap(fmt.Errorf("unexpected status code: %d", resp.StatusCode), appconstant.ErrTMDBSearchFailed)
	}

	var body tmdbSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, ungerr.Wrap(err, appconstant.ErrTMDBSearchFailed)
	}

	return mapTMDBResults(body), nil
}

type tmdbSearchResponse struct {
	Results []json.RawMessage `json:"results"`
}

type tmdbSearchItem struct {
	ID           int    `json:"id"`
	MediaType    string `json:"media_type"`
	Title        string `json:"title"`
	Name         string `json:"name"`
	ReleaseDate  string `json:"release_date"`
	FirstAirDate string `json:"first_air_date"`
	PosterPath   string `json:"poster_path"`
}

func mapTMDBResults(body tmdbSearchResponse) []TMDBResult {
	results := make([]TMDBResult, 0, len(body.Results))

	for _, raw := range body.Results {
		var item tmdbSearchItem
		if err := json.Unmarshal(raw, &item); err != nil {
			continue
		}

		// /search/multi also returns media_type "person" (cast/crew, not content);
		// it has no title/release date/poster_path, so it isn't catalog content.
		if item.MediaType == "person" {
			continue
		}

		title := item.Title
		releaseDate := item.ReleaseDate
		if item.MediaType == "tv" {
			title = item.Name
			releaseDate = item.FirstAirDate
		}

		var posterURL string
		if item.PosterPath != "" {
			posterURL = tmdbPosterBaseURL + item.PosterPath
		}

		results = append(results, TMDBResult{
			SourceID:    strconv.Itoa(item.ID),
			ContentType: item.MediaType,
			Title:       title,
			ReleaseYear: parseReleaseYear(releaseDate),
			PosterURL:   posterURL,
			Metadata:    raw,
		})
	}

	return results
}

func parseReleaseYear(date string) *int {
	if len(date) < 4 {
		return nil
	}
	year, err := strconv.Atoi(date[:4])
	if err != nil {
		return nil
	}
	return &year
}
