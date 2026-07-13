package service

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
	"github.com/itsLeonB/ungerr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/yunobar/album/internal/appconstant"
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/entity"
	"github.com/yunobar/album/internal/mocks"
)

// newTestDecisionSessionService wires a decisionSessionServiceImpl with all-
// mock repos, wrapping WithinTransaction to just invoke the closure — the
// same shape used everywhere else in this codebase's mocked service tests.
func newTestDecisionSessionService(t *testing.T) (
	DecisionSessionService,
	*mocks.MockRepository[entity.DecisionSession],
	*mocks.MockRepository[entity.SessionParticipant],
	*mocks.MockRepository[entity.SessionCandidate],
	*mocks.MockRepository[entity.GroupMember],
	*mocks.MockRepository[entity.Group],
	*mocks.MockRepository[entity.WatchlistItem],
	*mocks.MockRepository[entity.SessionPrioritySnapshot],
) {
	t.Helper()

	transactor := mocks.NewMockTransactor(t)
	transactor.EXPECT().
		WithinTransaction(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		Maybe()

	decisionSessionRepo := mocks.NewMockRepository[entity.DecisionSession](t)
	sessionParticipantRepo := mocks.NewMockRepository[entity.SessionParticipant](t)
	sessionCandidateRepo := mocks.NewMockRepository[entity.SessionCandidate](t)
	groupMemberRepo := mocks.NewMockRepository[entity.GroupMember](t)
	groupRepo := mocks.NewMockRepository[entity.Group](t)
	watchlistRepo := mocks.NewMockRepository[entity.WatchlistItem](t)
	snapshotRepo := mocks.NewMockRepository[entity.SessionPrioritySnapshot](t)

	svc := NewDecisionSessionService(
		transactor,
		decisionSessionRepo,
		sessionParticipantRepo,
		sessionCandidateRepo,
		groupMemberRepo,
		groupRepo,
		watchlistRepo,
		snapshotRepo,
	)

	return svc, decisionSessionRepo, sessionParticipantRepo, sessionCandidateRepo, groupMemberRepo, groupRepo, watchlistRepo, snapshotRepo
}

func TestDecisionSessionService_CapturePrioritySnapshots(t *testing.T) {
	sessionID := uuid.New()
	profileA := uuid.New()
	profileB := uuid.New()
	candidateOne := uuid.New()
	candidateTwo := uuid.New()
	offWatchlistCandidate := uuid.New()

	t.Run("writes a snapshot per on-watchlist candidate and skips off-watchlist ones", func(t *testing.T) {
		svc, _, _, _, _, _, watchlistRepo, snapshotRepo := newTestDecisionSessionService(t)

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
		svc, _, _, _, _, _, watchlistRepo, snapshotRepo := newTestDecisionSessionService(t)

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
		svc, _, _, _, _, _, watchlistRepo, _ := newTestDecisionSessionService(t)

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
		svc, _, _, _, _, _, watchlistRepo, snapshotRepo := newTestDecisionSessionService(t)

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

// Create's candidate-validation branch (mergedContentIDs's raw SQL against
// group_members/watchlist_items) isn't exercised here — go-crud's
// GetGormInstance can't be usefully stubbed without a real *gorm.DB, the
// same reason GroupService.GetMergedWatchlist (structurally identical raw
// SQL) has no mocked-repo test either. That branch, plus the full
// Create-then-Get success shape, is covered in
// internal/tests/decision_session_test.go against the real test DB.
func TestDecisionSessionService_Create(t *testing.T) {
	groupID := uuid.New()

	t.Run("rejects a caller who is not a member of the group", func(t *testing.T) {
		svc, _, _, _, groupMemberRepo, _, _, _ := newTestDecisionSessionService(t)

		caller := uuid.New()
		otherMember := uuid.New()

		groupMemberRepo.EXPECT().
			FindAll(mock.Anything, mock.MatchedBy(func(spec crud.Specification[entity.GroupMember]) bool {
				return spec.Model.GroupID == groupID
			})).
			Return([]entity.GroupMember{{ProfileID: otherMember}}, nil)

		_, err := svc.Create(context.Background(), caller, groupID, dto.CreateSessionRequest{
			Method:              "majority",
			ParticipantIDs:      []uuid.UUID{otherMember},
			CandidateContentIDs: []uuid.UUID{uuid.New()},
		})

		require.Error(t, err)
		var appErr ungerr.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, http.StatusNotFound, appErr.HttpStatus())
	})

	t.Run("rejects a participantId that isn't a group member", func(t *testing.T) {
		svc, _, _, _, groupMemberRepo, _, _, _ := newTestDecisionSessionService(t)

		caller := uuid.New()
		outsider := uuid.New()

		groupMemberRepo.EXPECT().
			FindAll(mock.Anything, mock.MatchedBy(func(spec crud.Specification[entity.GroupMember]) bool {
				return spec.Model.GroupID == groupID
			})).
			Return([]entity.GroupMember{{ProfileID: caller}}, nil)

		_, err := svc.Create(context.Background(), caller, groupID, dto.CreateSessionRequest{
			Method:              "majority",
			ParticipantIDs:      []uuid.UUID{caller, outsider},
			CandidateContentIDs: []uuid.UUID{uuid.New()},
		})

		require.Error(t, err)
		var appErr ungerr.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, http.StatusBadRequest, appErr.HttpStatus())
	})

	t.Run("propagates the membership lookup error", func(t *testing.T) {
		svc, _, _, _, groupMemberRepo, _, _, _ := newTestDecisionSessionService(t)

		wantErr := errors.New("db error")
		groupMemberRepo.EXPECT().FindAll(mock.Anything, mock.Anything).Return(nil, wantErr)

		_, err := svc.Create(context.Background(), uuid.New(), groupID, dto.CreateSessionRequest{
			Method:              "majority",
			ParticipantIDs:      []uuid.UUID{uuid.New()},
			CandidateContentIDs: []uuid.UUID{uuid.New()},
		})

		assert.ErrorIs(t, err, wantErr)
	})
}

// generateRandomSeed is crypto/rand-backed — this is the one part of
// Create's happy path that's a pure function and worth unit-testing in
// isolation, since the rest of the happy path needs the real DB (see note
// above).
func TestGenerateRandomSeed(t *testing.T) {
	seed1, err := generateRandomSeed()
	require.NoError(t, err)
	seed2, err := generateRandomSeed()
	require.NoError(t, err)

	assert.NotZero(t, seed1)
	assert.NotZero(t, seed2)
	assert.NotEqual(t, seed1, seed2, "a weak but real check the generator isn't a constant")
}
