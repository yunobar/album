package service

import (
	"context"

	"github.com/itsLeonB/go-crud"
	"github.com/itsLeonB/ungerr"
	"github.com/yunobar/album/internal/appconstant"
	"github.com/yunobar/album/internal/core/otel"
	"github.com/yunobar/album/internal/domain/client"
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/entity"
	"github.com/yunobar/album/internal/domain/mapper"
	"gorm.io/gorm/clause"
)

type ContentService interface {
	Search(ctx context.Context, query string) ([]dto.ContentResponse, error)
}

type contentServiceImpl struct {
	transactor  crud.Transactor
	contentRepo crud.Repository[entity.Content]
	tmdb        client.TMDBClient
}

func NewContentService(
	transactor crud.Transactor,
	contentRepo crud.Repository[entity.Content],
	tmdb client.TMDBClient,
) ContentService {
	return &contentServiceImpl{transactor, contentRepo, tmdb}
}

func (cs *contentServiceImpl) Search(ctx context.Context, query string) ([]dto.ContentResponse, error) {
	ctx, span := otel.Tracer.Start(ctx, "ContentService.Search")
	defer span.End()

	results, err := cs.tmdb.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	contents := make([]entity.Content, len(results))
	for i, r := range results {
		contents[i] = entity.Content{
			Source:      "tmdb",
			SourceID:    r.SourceID,
			ContentType: r.ContentType,
			Title:       r.Title,
			ReleaseYear: r.ReleaseYear,
			PosterURL:   r.PosterURL,
			Metadata:    r.Metadata,
		}
	}

	var saved []entity.Content
	err = cs.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		var err error
		saved, err = cs.upsertMany(ctx, contents)
		return err
	})
	if err != nil {
		return nil, err
	}

	responses := make([]dto.ContentResponse, len(saved))
	for i, c := range saved {
		responses[i] = mapper.ContentToResponse(c)
	}

	return responses, nil
}

// upsertMany batches all results into a single INSERT ... ON CONFLICT (source, source_id, content_type)
// DO UPDATE, dropping past crud.Repository[T] (which has no upsert primitive) via
// GetGormInstance. created_at is left untouched on conflict so re-searching a title
// never resets its original insert time.
func (cs *contentServiceImpl) upsertMany(ctx context.Context, contents []entity.Content) ([]entity.Content, error) {
	if len(contents) == 0 {
		return nil, nil
	}

	db, err := cs.contentRepo.GetGormInstance(ctx)
	if err != nil {
		return nil, err
	}

	// TMDB ids are only unique within a media type (a movie and a tv show can share
	// the same numeric id), so content_type is part of the conflict target, not just
	// an updated column.
	err = db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "source"}, {Name: "source_id"}, {Name: "content_type"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"title", "release_year", "poster_url", "metadata", "updated_at",
		}),
	}).Create(&contents).Error
	if err != nil {
		return nil, ungerr.Wrap(err, appconstant.ErrDataInsert)
	}

	return contents, nil
}
