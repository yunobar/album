package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/itsLeonB/ezutil/v2"
	"github.com/itsLeonB/go-crud"
	"github.com/itsLeonB/ungerr"
	"github.com/yunobar/album/internal/appconstant"
	"github.com/yunobar/album/internal/core/otel"
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/entity"
	"github.com/yunobar/album/internal/domain/mapper"
)

type WatchlistService interface {
	List(ctx context.Context, profileID uuid.UUID) ([]dto.WatchlistItemResponse, error)
	Add(ctx context.Context, profileID uuid.UUID, request dto.AddWatchlistItemRequest) (dto.WatchlistItemResponse, error)
	Update(ctx context.Context, profileID, contentID uuid.UUID, request dto.UpdateWatchlistItemRequest) (dto.WatchlistItemResponse, error)
	Remove(ctx context.Context, profileID, contentID uuid.UUID) error
}

type watchlistServiceImpl struct {
	transactor    crud.Transactor
	watchlistRepo crud.Repository[entity.WatchlistItem]
	contentRepo   crud.Repository[entity.Content]
}

func NewWatchlistService(
	transactor crud.Transactor,
	watchlistRepo crud.Repository[entity.WatchlistItem],
	contentRepo crud.Repository[entity.Content],
) WatchlistService {
	return &watchlistServiceImpl{transactor, watchlistRepo, contentRepo}
}

func (ws *watchlistServiceImpl) List(ctx context.Context, profileID uuid.UUID) ([]dto.WatchlistItemResponse, error) {
	ctx, span := otel.Tracer.Start(ctx, "WatchlistService.List")
	defer span.End()

	spec := crud.Specification[entity.WatchlistItem]{
		PreloadRelations: []string{"Content"},
	}
	spec.Model.ProfileID = profileID
	spec.Model.Status = appconstant.WatchlistStatusActive

	items, err := ws.watchlistRepo.FindAll(ctx, spec)
	if err != nil {
		return nil, err
	}

	return ezutil.MapSlice(items, mapper.WatchlistItemToResponse), nil
}

func (ws *watchlistServiceImpl) Add(ctx context.Context, profileID uuid.UUID, request dto.AddWatchlistItemRequest) (dto.WatchlistItemResponse, error) {
	ctx, span := otel.Tracer.Start(ctx, "WatchlistService.Add")
	defer span.End()

	var response dto.WatchlistItemResponse

	err := ws.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		spec := crud.Specification[entity.WatchlistItem]{}
		spec.Model.ProfileID = profileID
		spec.Model.ContentID = request.ContentID
		existing, err := ws.watchlistRepo.FindFirst(ctx, spec)
		if err != nil {
			return err
		}
		if !existing.IsZero() {
			return ungerr.ConflictError(fmt.Sprintf("content %s is already on the watchlist", request.ContentID))
		}

		contentSpec := crud.Specification[entity.Content]{}
		contentSpec.Model.ID = request.ContentID
		content, err := ws.contentRepo.FindFirst(ctx, contentSpec)
		if err != nil {
			return err
		}
		if content.IsZero() {
			return ungerr.NotFoundError(fmt.Sprintf("content %s is not found", request.ContentID))
		}

		inserted, err := ws.watchlistRepo.Insert(ctx, entity.WatchlistItem{
			ProfileID: profileID,
			ContentID: request.ContentID,
			Priority:  request.Priority,
			Notes:     request.Notes,
			Status:    appconstant.WatchlistStatusActive,
		})
		if err != nil {
			return err
		}

		inserted.Content = content
		response = mapper.WatchlistItemToResponse(inserted)
		return nil
	})

	return response, err
}

func (ws *watchlistServiceImpl) Update(ctx context.Context, profileID, contentID uuid.UUID, request dto.UpdateWatchlistItemRequest) (dto.WatchlistItemResponse, error) {
	ctx, span := otel.Tracer.Start(ctx, "WatchlistService.Update")
	defer span.End()

	var response dto.WatchlistItemResponse

	err := ws.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		spec := crud.Specification[entity.WatchlistItem]{
			ForUpdate:        true,
			PreloadRelations: []string{"Content"},
		}
		spec.Model.ProfileID = profileID
		spec.Model.ContentID = contentID
		item, err := ws.watchlistRepo.FindFirst(ctx, spec)
		if err != nil {
			return err
		}
		if item.IsZero() {
			return ungerr.NotFoundError(fmt.Sprintf("watchlist item for content %s is not found", contentID))
		}

		if request.Priority != "" {
			item.Priority = request.Priority
		}
		if request.Notes != nil {
			item.Notes = *request.Notes
		}

		updated, err := ws.watchlistRepo.Update(ctx, item)
		if err != nil {
			return err
		}

		response = mapper.WatchlistItemToResponse(updated)
		return nil
	})

	return response, err
}

func (ws *watchlistServiceImpl) Remove(ctx context.Context, profileID, contentID uuid.UUID) error {
	ctx, span := otel.Tracer.Start(ctx, "WatchlistService.Remove")
	defer span.End()

	spec := crud.Specification[entity.WatchlistItem]{}
	spec.Model.ProfileID = profileID
	spec.Model.ContentID = contentID
	item, err := ws.watchlistRepo.FindFirst(ctx, spec)
	if err != nil {
		return err
	}
	if item.IsZero() {
		return ungerr.NotFoundError(fmt.Sprintf("watchlist item for content %s is not found", contentID))
	}

	return ws.watchlistRepo.Delete(ctx, item)
}
