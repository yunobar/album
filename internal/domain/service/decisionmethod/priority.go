package decisionmethod

import (
	"context"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
	"github.com/yunobar/album/internal/appconstant"
	"github.com/yunobar/album/internal/domain/entity"
)

type priorityStrategy struct {
	base
	watchlistRepo crud.Repository[entity.WatchlistItem]
	snapshotRepo  crud.Repository[entity.SessionPrioritySnapshot]
}

// OnCreate freezes each participant's current watchlist priority for every
// candidate they have actively watchlisted, at session-creation time. A
// participant with no active watchlist_items row for a candidate gets no
// snapshot row — Resolve treats a missing row as weight 0.
func (s *priorityStrategy) OnCreate(ctx context.Context, session entity.DecisionSession, participantIDs, candidateIDs []uuid.UUID) error {
	candidateSet := make(map[uuid.UUID]struct{}, len(candidateIDs))
	for _, id := range candidateIDs {
		candidateSet[id] = struct{}{}
	}

	var snapshots []entity.SessionPrioritySnapshot

	for _, profileID := range participantIDs {
		spec := crud.Specification[entity.WatchlistItem]{}
		spec.Model.ProfileID = profileID
		spec.Model.Status = appconstant.WatchlistStatusActive

		items, err := s.watchlistRepo.FindAll(ctx, spec)
		if err != nil {
			return err
		}

		for _, item := range items {
			if _, ok := candidateSet[item.ContentID]; !ok {
				continue
			}

			snapshots = append(snapshots, entity.SessionPrioritySnapshot{
				SessionID: session.ID,
				ProfileID: profileID,
				ContentID: item.ContentID,
				Priority:  item.Priority,
			})
		}
	}

	if len(snapshots) == 0 {
		return nil
	}

	_, err := s.snapshotRepo.InsertMany(ctx, snapshots)
	return err
}

// Resolve: highest weighted total wins; tie → highest single Must-Watch
// count; still tied → seeded random.
func (s *priorityStrategy) Resolve(ctx context.Context, session entity.DecisionSession) (uuid.UUID, error) {
	snapshotSpec := crud.Specification[entity.SessionPrioritySnapshot]{}
	snapshotSpec.Model.SessionID = session.ID
	snapshots, err := s.snapshotRepo.FindAll(ctx, snapshotSpec)
	if err != nil {
		return uuid.Nil, err
	}

	candidateIDs := sessionCandidateIDs(session)
	totals := make(map[uuid.UUID]int, len(candidateIDs))
	mustCounts := make(map[uuid.UUID]int, len(candidateIDs))
	for _, cid := range candidateIDs {
		totals[cid] = 0
		mustCounts[cid] = 0
	}
	for _, snap := range snapshots {
		totals[snap.ContentID] += appconstant.PriorityWeights[snap.Priority]
		if snap.Priority == appconstant.PriorityMust {
			mustCounts[snap.ContentID]++
		}
	}

	tied := tiedFor(candidateIDs, totals)
	if len(tied) == 1 {
		return tied[0], nil
	}
	return seededPick(session.RandomSeed, tiedFor(tied, mustCounts)), nil
}
