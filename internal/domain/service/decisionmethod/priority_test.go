package decisionmethod

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

func TestResolvePriority(t *testing.T) {
	newStrategy := func(t *testing.T, snapshots []entity.SessionPrioritySnapshot) *priorityStrategy {
		t.Helper()
		repo := mocks.NewMockRepository[entity.SessionPrioritySnapshot](t)
		repo.EXPECT().FindAll(mock.Anything, mock.Anything).Return(snapshots, nil)
		return &priorityStrategy{snapshotRepo: repo}
	}

	t.Run("clear winner by weighted total", func(t *testing.T) {
		a, b := uuid.New(), uuid.New()
		snapshots := []entity.SessionPrioritySnapshot{
			{ContentID: a, Priority: appconstant.PriorityMust},
			{ContentID: b, Priority: appconstant.PriorityLow},
		}
		session := entity.DecisionSession{
			Candidates: []entity.SessionCandidate{{ContentID: a}, {ContentID: b}},
			RandomSeed: 1,
		}

		winner, err := newStrategy(t, snapshots).Resolve(context.Background(), session)

		assert.NoError(t, err)
		assert.Equal(t, a, winner)
	})

	t.Run("tie on total broken by must-watch count", func(t *testing.T) {
		a, b := uuid.New(), uuid.New()
		// a: high(3) + medium(2) = 5, no must votes.
		// b: must(5) alone = 5, one must vote.
		snapshots := []entity.SessionPrioritySnapshot{
			{ContentID: a, Priority: appconstant.PriorityHigh},
			{ContentID: a, Priority: appconstant.PriorityMedium},
			{ContentID: b, Priority: appconstant.PriorityMust},
		}
		session := entity.DecisionSession{
			Candidates: []entity.SessionCandidate{{ContentID: a}, {ContentID: b}},
			RandomSeed: 1,
		}

		winner, err := newStrategy(t, snapshots).Resolve(context.Background(), session)

		assert.NoError(t, err)
		assert.Equal(t, b, winner, "b has 1 must-watch vote vs a's 0, despite equal totals")
	})

	t.Run("tie on total and must count falls to deterministic seeded pick", func(t *testing.T) {
		a, b := uuid.New(), uuid.New()
		snapshots := []entity.SessionPrioritySnapshot{
			{ContentID: a, Priority: appconstant.PriorityMust},
			{ContentID: b, Priority: appconstant.PriorityMust},
		}
		session := entity.DecisionSession{
			Candidates: []entity.SessionCandidate{{ContentID: a}, {ContentID: b}},
			RandomSeed: 9,
		}

		first, err := newStrategy(t, snapshots).Resolve(context.Background(), session)
		assert.NoError(t, err)
		second, err := newStrategy(t, snapshots).Resolve(context.Background(), session)
		assert.NoError(t, err)

		assert.Equal(t, first, second)
		assert.True(t, first == a || first == b)
	})

	t.Run("zero snapshots, all weight 0, still returns a candidate", func(t *testing.T) {
		a, b := uuid.New(), uuid.New()
		session := entity.DecisionSession{
			Candidates: []entity.SessionCandidate{{ContentID: a}, {ContentID: b}},
			RandomSeed: 4,
		}

		winner, err := newStrategy(t, nil).Resolve(context.Background(), session)

		assert.NoError(t, err)
		assert.True(t, winner == a || winner == b)
	})
}

func TestPriorityStrategy_OnCreate(t *testing.T) {
	sessionID := uuid.New()
	profileA := uuid.New()
	profileB := uuid.New()
	candidateOne := uuid.New()
	candidateTwo := uuid.New()
	offWatchlistCandidate := uuid.New()

	newStrategy := func(t *testing.T) (*priorityStrategy, *mocks.MockRepository[entity.WatchlistItem], *mocks.MockRepository[entity.SessionPrioritySnapshot]) {
		t.Helper()
		watchlistRepo := mocks.NewMockRepository[entity.WatchlistItem](t)
		snapshotRepo := mocks.NewMockRepository[entity.SessionPrioritySnapshot](t)
		return &priorityStrategy{watchlistRepo: watchlistRepo, snapshotRepo: snapshotRepo}, watchlistRepo, snapshotRepo
	}

	t.Run("writes a snapshot per on-watchlist candidate and skips off-watchlist ones", func(t *testing.T) {
		strat, watchlistRepo, snapshotRepo := newStrategy(t)

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

		err := strat.OnCreate(
			context.Background(), entity.DecisionSession{ID: sessionID},
			[]uuid.UUID{profileA, profileB},
			[]uuid.UUID{candidateOne, candidateTwo},
		)

		assert.NoError(t, err)
	})

	t.Run("writes no rows and does not call InsertMany when nothing matches", func(t *testing.T) {
		strat, watchlistRepo, snapshotRepo := newStrategy(t)

		watchlistRepo.EXPECT().
			FindAll(mock.Anything, mock.Anything).
			Return([]entity.WatchlistItem{}, nil)

		err := strat.OnCreate(
			context.Background(), entity.DecisionSession{ID: sessionID},
			[]uuid.UUID{profileA},
			[]uuid.UUID{candidateOne},
		)

		assert.NoError(t, err)
		snapshotRepo.AssertNotCalled(t, "InsertMany", mock.Anything, mock.Anything)
	})

	t.Run("propagates FindAll error", func(t *testing.T) {
		strat, watchlistRepo, _ := newStrategy(t)

		wantErr := errors.New("db error")
		watchlistRepo.EXPECT().
			FindAll(mock.Anything, mock.Anything).
			Return(nil, wantErr)

		err := strat.OnCreate(
			context.Background(), entity.DecisionSession{ID: sessionID},
			[]uuid.UUID{profileA},
			[]uuid.UUID{candidateOne},
		)

		assert.ErrorIs(t, err, wantErr)
	})

	t.Run("propagates InsertMany error", func(t *testing.T) {
		strat, watchlistRepo, snapshotRepo := newStrategy(t)

		watchlistRepo.EXPECT().
			FindAll(mock.Anything, mock.Anything).
			Return([]entity.WatchlistItem{
				{ProfileID: profileA, ContentID: candidateOne, Priority: "high"},
			}, nil)

		wantErr := errors.New("insert error")
		snapshotRepo.EXPECT().
			InsertMany(mock.Anything, mock.Anything).
			Return(nil, wantErr)

		err := strat.OnCreate(
			context.Background(), entity.DecisionSession{ID: sessionID},
			[]uuid.UUID{profileA},
			[]uuid.UUID{candidateOne},
		)

		assert.ErrorIs(t, err, wantErr)
	})
}
