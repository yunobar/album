package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
	"github.com/yunobar/album/internal/appconstant"
	"github.com/yunobar/album/internal/core/otel"
	"github.com/yunobar/album/internal/domain/entity"
)

type DecisionSessionService interface {
	// CapturePrioritySnapshots freezes each participant's current watchlist
	// priority for every candidate they have actively watchlisted, at
	// session-creation time. A participant with no active watchlist_items
	// row for a candidate gets no snapshot row — the Priority-Based
	// resolver (Task 4) treats a missing row as weight 0.
	CapturePrioritySnapshots(ctx context.Context, sessionID uuid.UUID, participantIDs, candidateIDs []uuid.UUID) error
}

type decisionSessionServiceImpl struct {
	watchlistRepo               crud.Repository[entity.WatchlistItem]
	sessionPrioritySnapshotRepo crud.Repository[entity.SessionPrioritySnapshot]
}

func NewDecisionSessionService(
	watchlistRepo crud.Repository[entity.WatchlistItem],
	sessionPrioritySnapshotRepo crud.Repository[entity.SessionPrioritySnapshot],
) DecisionSessionService {
	return &decisionSessionServiceImpl{watchlistRepo, sessionPrioritySnapshotRepo}
}

func (dss *decisionSessionServiceImpl) CapturePrioritySnapshots(ctx context.Context, sessionID uuid.UUID, participantIDs, candidateIDs []uuid.UUID) error {
	ctx, span := otel.Tracer.Start(ctx, "DecisionSessionService.CapturePrioritySnapshots")
	defer span.End()

	candidateSet := make(map[uuid.UUID]struct{}, len(candidateIDs))
	for _, id := range candidateIDs {
		candidateSet[id] = struct{}{}
	}

	var snapshots []entity.SessionPrioritySnapshot

	for _, profileID := range participantIDs {
		spec := crud.Specification[entity.WatchlistItem]{}
		spec.Model.ProfileID = profileID
		spec.Model.Status = appconstant.WatchlistStatusActive

		items, err := dss.watchlistRepo.FindAll(ctx, spec)
		if err != nil {
			return err
		}

		for _, item := range items {
			if _, ok := candidateSet[item.ContentID]; !ok {
				continue
			}

			snapshots = append(snapshots, entity.SessionPrioritySnapshot{
				SessionID: sessionID,
				ProfileID: profileID,
				ContentID: item.ContentID,
				Priority:  item.Priority,
			})
		}
	}

	if len(snapshots) == 0 {
		return nil
	}

	_, err := dss.sessionPrioritySnapshotRepo.InsertMany(ctx, snapshots)
	return err
}
