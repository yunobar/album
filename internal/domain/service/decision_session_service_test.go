package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/yunobar/album/internal/appconstant"
	"github.com/yunobar/album/internal/domain/entity"
	"github.com/yunobar/album/internal/mocks"
)

func TestDecisionSessionService_CapturePrioritySnapshots(t *testing.T) {
	sessionID := uuid.New()
	profileA := uuid.New()
	profileB := uuid.New()
	candidateOne := uuid.New()
	candidateTwo := uuid.New()
	offWatchlistCandidate := uuid.New()

	t.Run("writes a snapshot per on-watchlist candidate and skips off-watchlist ones", func(t *testing.T) {
		watchlistRepo := mocks.NewMockRepository[entity.WatchlistItem](t)
		snapshotRepo := mocks.NewMockRepository[entity.SessionPrioritySnapshot](t)
		svc := NewDecisionSessionService(watchlistRepo, snapshotRepo)

		// profileA has both candidates actively watchlisted.
		watchlistRepo.EXPECT().
			FindAll(mock.Anything, mock.MatchedBy(func(spec crud.Specification[entity.WatchlistItem]) bool {
				return spec.Model.ProfileID == profileA && spec.Model.Status == appconstant.WatchlistStatusActive
			})).
			Return([]entity.WatchlistItem{
				{ProfileID: profileA, ContentID: candidateOne, Priority: "high"},
				{ProfileID: profileA, ContentID: candidateTwo, Priority: "must"},
			}, nil)

		// profileB is missing candidateTwo, and has an unrelated item that
		// isn't in the session's candidate set.
		watchlistRepo.EXPECT().
			FindAll(mock.Anything, mock.MatchedBy(func(spec crud.Specification[entity.WatchlistItem]) bool {
				return spec.Model.ProfileID == profileB && spec.Model.Status == appconstant.WatchlistStatusActive
			})).
			Return([]entity.WatchlistItem{
				{ProfileID: profileB, ContentID: candidateOne, Priority: "low"},
				{ProfileID: profileB, ContentID: offWatchlistCandidate, Priority: "medium"},
			}, nil)

		snapshotRepo.EXPECT().
			InsertMany(mock.Anything, []entity.SessionPrioritySnapshot{
				{SessionID: sessionID, ProfileID: profileA, ContentID: candidateOne, Priority: "high"},
				{SessionID: sessionID, ProfileID: profileA, ContentID: candidateTwo, Priority: "must"},
				{SessionID: sessionID, ProfileID: profileB, ContentID: candidateOne, Priority: "low"},
			}).
			Return(nil, nil)

		err := svc.CapturePrioritySnapshots(
			context.Background(), sessionID,
			[]uuid.UUID{profileA, profileB},
			[]uuid.UUID{candidateOne, candidateTwo},
		)

		assert.NoError(t, err)
	})

	t.Run("writes no rows and does not call InsertMany when nothing matches", func(t *testing.T) {
		watchlistRepo := mocks.NewMockRepository[entity.WatchlistItem](t)
		snapshotRepo := mocks.NewMockRepository[entity.SessionPrioritySnapshot](t)
		svc := NewDecisionSessionService(watchlistRepo, snapshotRepo)

		watchlistRepo.EXPECT().
			FindAll(mock.Anything, mock.Anything).
			Return([]entity.WatchlistItem{}, nil)

		err := svc.CapturePrioritySnapshots(
			context.Background(), sessionID,
			[]uuid.UUID{profileA},
			[]uuid.UUID{candidateOne},
		)

		assert.NoError(t, err)
		snapshotRepo.AssertNotCalled(t, "InsertMany", mock.Anything, mock.Anything)
	})

	t.Run("propagates FindAll error", func(t *testing.T) {
		watchlistRepo := mocks.NewMockRepository[entity.WatchlistItem](t)
		snapshotRepo := mocks.NewMockRepository[entity.SessionPrioritySnapshot](t)
		svc := NewDecisionSessionService(watchlistRepo, snapshotRepo)

		wantErr := errors.New("db error")
		watchlistRepo.EXPECT().
			FindAll(mock.Anything, mock.Anything).
			Return(nil, wantErr)

		err := svc.CapturePrioritySnapshots(
			context.Background(), sessionID,
			[]uuid.UUID{profileA},
			[]uuid.UUID{candidateOne},
		)

		assert.ErrorIs(t, err, wantErr)
	})

	t.Run("propagates InsertMany error", func(t *testing.T) {
		watchlistRepo := mocks.NewMockRepository[entity.WatchlistItem](t)
		snapshotRepo := mocks.NewMockRepository[entity.SessionPrioritySnapshot](t)
		svc := NewDecisionSessionService(watchlistRepo, snapshotRepo)

		watchlistRepo.EXPECT().
			FindAll(mock.Anything, mock.Anything).
			Return([]entity.WatchlistItem{
				{ProfileID: profileA, ContentID: candidateOne, Priority: "high"},
			}, nil)

		wantErr := errors.New("insert error")
		snapshotRepo.EXPECT().
			InsertMany(mock.Anything, mock.Anything).
			Return(nil, wantErr)

		err := svc.CapturePrioritySnapshots(
			context.Background(), sessionID,
			[]uuid.UUID{profileA},
			[]uuid.UUID{candidateOne},
		)

		assert.ErrorIs(t, err, wantErr)
	})
}
