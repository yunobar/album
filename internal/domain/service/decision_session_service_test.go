package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

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
	*mocks.MockRepository[entity.SessionVote],
	*mocks.MockRepository[entity.SessionRanking],
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
	sessionVoteRepo := mocks.NewMockRepository[entity.SessionVote](t)
	sessionRankingRepo := mocks.NewMockRepository[entity.SessionRanking](t)

	svc := NewDecisionSessionService(
		transactor,
		decisionSessionRepo,
		sessionParticipantRepo,
		sessionCandidateRepo,
		groupMemberRepo,
		groupRepo,
		watchlistRepo,
		snapshotRepo,
		sessionVoteRepo,
		sessionRankingRepo,
	)

	return svc, decisionSessionRepo, sessionParticipantRepo, sessionCandidateRepo, groupMemberRepo, groupRepo, watchlistRepo, snapshotRepo, sessionVoteRepo, sessionRankingRepo
}

func TestDecisionSessionService_CapturePrioritySnapshots(t *testing.T) {
	sessionID := uuid.New()
	profileA := uuid.New()
	profileB := uuid.New()
	candidateOne := uuid.New()
	candidateTwo := uuid.New()
	offWatchlistCandidate := uuid.New()

	t.Run("writes a snapshot per on-watchlist candidate and skips off-watchlist ones", func(t *testing.T) {
		svc, _, _, _, _, _, watchlistRepo, snapshotRepo, _, _ := newTestDecisionSessionService(t)

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
		svc, _, _, _, _, _, watchlistRepo, snapshotRepo, _, _ := newTestDecisionSessionService(t)

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
		svc, _, _, _, _, _, watchlistRepo, _, _, _ := newTestDecisionSessionService(t)

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
		svc, _, _, _, _, _, watchlistRepo, snapshotRepo, _, _ := newTestDecisionSessionService(t)

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
		svc, _, _, _, groupMemberRepo, _, _, _, _, _ := newTestDecisionSessionService(t)

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
		svc, _, _, _, groupMemberRepo, _, _, _, _, _ := newTestDecisionSessionService(t)

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
		svc, _, _, _, groupMemberRepo, _, _, _, _, _ := newTestDecisionSessionService(t)

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

func TestDecisionSessionService_CastVote(t *testing.T) {
	sessionID := uuid.New()
	profileID := uuid.New()
	candidateOne := uuid.New()
	candidateTwo := uuid.New()

	votingMajoritySession := func() entity.DecisionSession {
		return entity.DecisionSession{
			ID:         sessionID,
			Method:     "majority",
			Status:     "voting",
			Candidates: []entity.SessionCandidate{{ContentID: candidateOne}, {ContentID: candidateTwo}},
		}
	}

	expectParticipant := func(participantRepo *mocks.MockRepository[entity.SessionParticipant]) {
		participantRepo.EXPECT().
			FindFirst(mock.Anything, mock.MatchedBy(func(spec crud.Specification[entity.SessionParticipant]) bool {
				return spec.Model.SessionID == sessionID && spec.Model.ProfileID == profileID
			})).
			Return(entity.SessionParticipant{BaseEntity: crud.BaseEntity{ID: uuid.New()}, SessionID: sessionID, ProfileID: profileID}, nil)
	}

	t.Run("rejects a non-majority session with 400", func(t *testing.T) {
		svc, decisionSessionRepo, sessionParticipantRepo, _, _, _, _, _, _, _ := newTestDecisionSessionService(t)
		expectParticipant(sessionParticipantRepo)

		session := votingMajoritySession()
		session.Method = "ranked"
		decisionSessionRepo.EXPECT().FindFirst(mock.Anything, mock.Anything).Return(session, nil)

		_, err := svc.CastVote(context.Background(), profileID, sessionID, dto.CastVoteRequest{ContentID: candidateOne})

		require.Error(t, err)
		var appErr ungerr.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, http.StatusBadRequest, appErr.HttpStatus())
	})

	t.Run("rejects a non-candidate contentId with 400", func(t *testing.T) {
		svc, decisionSessionRepo, sessionParticipantRepo, _, _, _, _, _, _, _ := newTestDecisionSessionService(t)
		expectParticipant(sessionParticipantRepo)
		decisionSessionRepo.EXPECT().FindFirst(mock.Anything, mock.Anything).Return(votingMajoritySession(), nil)

		_, err := svc.CastVote(context.Background(), profileID, sessionID, dto.CastVoteRequest{ContentID: uuid.New()})

		require.Error(t, err)
		var appErr ungerr.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, http.StatusBadRequest, appErr.HttpStatus())
	})

	t.Run("rejects a session that isn't open for voting with 409", func(t *testing.T) {
		svc, decisionSessionRepo, sessionParticipantRepo, _, _, _, _, _, _, _ := newTestDecisionSessionService(t)
		expectParticipant(sessionParticipantRepo)

		session := votingMajoritySession()
		session.Status = "completed"
		decisionSessionRepo.EXPECT().FindFirst(mock.Anything, mock.Anything).Return(session, nil)

		_, err := svc.CastVote(context.Background(), profileID, sessionID, dto.CastVoteRequest{ContentID: candidateOne})

		require.Error(t, err)
		var appErr ungerr.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, http.StatusConflict, appErr.HttpStatus())
	})

	t.Run("replaces a prior vote rather than stacking, and returns correct counts", func(t *testing.T) {
		svc, decisionSessionRepo, sessionParticipantRepo, _, _, _, _, _, sessionVoteRepo, _ := newTestDecisionSessionService(t)
		expectParticipant(sessionParticipantRepo)
		decisionSessionRepo.EXPECT().FindFirst(mock.Anything, mock.Anything).Return(votingMajoritySession(), nil)

		existingVote := entity.SessionVote{BaseEntity: crud.BaseEntity{ID: uuid.New()}, SessionID: sessionID, ProfileID: profileID, ContentID: candidateOne}
		sessionVoteRepo.EXPECT().
			FindFirst(mock.Anything, mock.MatchedBy(func(spec crud.Specification[entity.SessionVote]) bool {
				return spec.Model.SessionID == sessionID && spec.Model.ProfileID == profileID
			})).
			Return(existingVote, nil)

		sessionVoteRepo.EXPECT().
			Update(mock.Anything, mock.MatchedBy(func(v entity.SessionVote) bool {
				return v.ContentID == candidateTwo
			})).
			Return(entity.SessionVote{}, nil)
		sessionVoteRepo.AssertNotCalled(t, "Insert", mock.Anything, mock.Anything)

		sessionVoteRepo.EXPECT().
			FindAll(mock.Anything, mock.MatchedBy(func(spec crud.Specification[entity.SessionVote]) bool {
				return spec.Model.SessionID == sessionID
			})).
			Return([]entity.SessionVote{
				{SessionID: sessionID, ProfileID: profileID, ContentID: candidateTwo},
				{SessionID: sessionID, ProfileID: uuid.New(), ContentID: candidateTwo},
			}, nil)

		resp, err := svc.CastVote(context.Background(), profileID, sessionID, dto.CastVoteRequest{ContentID: candidateTwo})

		require.NoError(t, err)
		tally, ok := resp.Tally.(dto.CountsTally)
		require.True(t, ok)
		assert.Equal(t, 2, tally.Counts[candidateTwo.String()])
	})
}

func TestDecisionSessionService_SubmitRanking(t *testing.T) {
	sessionID := uuid.New()
	profileID := uuid.New()
	candidateOne := uuid.New()
	candidateTwo := uuid.New()

	votingRankedSession := func() entity.DecisionSession {
		return entity.DecisionSession{
			ID:         sessionID,
			Method:     "ranked",
			Status:     "voting",
			Candidates: []entity.SessionCandidate{{ContentID: candidateOne}, {ContentID: candidateTwo}},
		}
	}

	expectParticipant := func(participantRepo *mocks.MockRepository[entity.SessionParticipant]) {
		participantRepo.EXPECT().
			FindFirst(mock.Anything, mock.MatchedBy(func(spec crud.Specification[entity.SessionParticipant]) bool {
				return spec.Model.SessionID == sessionID && spec.Model.ProfileID == profileID
			})).
			Return(entity.SessionParticipant{BaseEntity: crud.BaseEntity{ID: uuid.New()}, SessionID: sessionID, ProfileID: profileID}, nil)
	}

	t.Run("rejects a duplicate candidate in the ranking with 400", func(t *testing.T) {
		svc, decisionSessionRepo, sessionParticipantRepo, _, _, _, _, _, _, _ := newTestDecisionSessionService(t)
		expectParticipant(sessionParticipantRepo)
		decisionSessionRepo.EXPECT().FindFirst(mock.Anything, mock.Anything).Return(votingRankedSession(), nil)

		_, err := svc.SubmitRanking(context.Background(), profileID, sessionID, dto.SubmitRankingRequest{
			Ranking: []uuid.UUID{candidateOne, candidateOne},
		})

		require.Error(t, err)
		var appErr ungerr.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, http.StatusBadRequest, appErr.HttpStatus())
	})

	t.Run("rejects a non-candidate entry with 400", func(t *testing.T) {
		svc, decisionSessionRepo, sessionParticipantRepo, _, _, _, _, _, _, _ := newTestDecisionSessionService(t)
		expectParticipant(sessionParticipantRepo)
		decisionSessionRepo.EXPECT().FindFirst(mock.Anything, mock.Anything).Return(votingRankedSession(), nil)

		_, err := svc.SubmitRanking(context.Background(), profileID, sessionID, dto.SubmitRankingRequest{
			Ranking: []uuid.UUID{candidateOne, uuid.New()},
		})

		require.Error(t, err)
		var appErr ungerr.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, http.StatusBadRequest, appErr.HttpStatus())
	})

	t.Run("replaces the full prior ballot rather than accumulating", func(t *testing.T) {
		svc, decisionSessionRepo, sessionParticipantRepo, _, _, _, _, _, _, sessionRankingRepo := newTestDecisionSessionService(t)
		expectParticipant(sessionParticipantRepo)
		decisionSessionRepo.EXPECT().FindFirst(mock.Anything, mock.Anything).Return(votingRankedSession(), nil)

		priorRankings := []entity.SessionRanking{
			{SessionID: sessionID, ProfileID: profileID, ContentID: candidateOne, Rank: 1},
			{SessionID: sessionID, ProfileID: profileID, ContentID: candidateTwo, Rank: 2},
		}
		sessionRankingRepo.EXPECT().
			FindAll(mock.Anything, mock.MatchedBy(func(spec crud.Specification[entity.SessionRanking]) bool {
				return spec.Model.SessionID == sessionID && spec.Model.ProfileID == profileID
			})).
			Return(priorRankings, nil)

		sessionRankingRepo.EXPECT().
			DeleteMany(mock.Anything, priorRankings).
			Return(nil)

		sessionRankingRepo.EXPECT().
			InsertMany(mock.Anything, []entity.SessionRanking{
				{SessionID: sessionID, ProfileID: profileID, ContentID: candidateTwo, Rank: 1},
				{SessionID: sessionID, ProfileID: profileID, ContentID: candidateOne, Rank: 2},
			}).
			Return(nil, nil)

		sessionRankingRepo.EXPECT().
			FindAll(mock.Anything, mock.MatchedBy(func(spec crud.Specification[entity.SessionRanking]) bool {
				return spec.Model.SessionID == sessionID && spec.Model.ProfileID == uuid.Nil
			})).
			Return([]entity.SessionRanking{
				{SessionID: sessionID, ProfileID: profileID, ContentID: candidateTwo, Rank: 1},
			}, nil)

		resp, err := svc.SubmitRanking(context.Background(), profileID, sessionID, dto.SubmitRankingRequest{
			Ranking: []uuid.UUID{candidateTwo, candidateOne},
		})

		require.NoError(t, err)
		tally, ok := resp.Tally.(dto.RankedTally)
		require.True(t, ok)
		assert.Equal(t, 1, tally.Counts[candidateTwo.String()])
	})
}

func TestDecisionSessionService_Select(t *testing.T) {
	sessionID := uuid.New()
	groupID := uuid.New()
	chooserProfileID := uuid.New()
	otherProfileID := uuid.New()
	candidateOne := uuid.New()

	votingRoundRobinSession := func() entity.DecisionSession {
		return entity.DecisionSession{
			ID:         sessionID,
			GroupID:    groupID,
			Method:     "round_robin",
			Status:     "voting",
			Candidates: []entity.SessionCandidate{{ContentID: candidateOne}},
			Participants: []entity.SessionParticipant{
				{SessionID: sessionID, ProfileID: chooserProfileID},
			},
		}
	}

	expectParticipant := func(participantRepo *mocks.MockRepository[entity.SessionParticipant], callerID uuid.UUID) {
		participantRepo.EXPECT().
			FindFirst(mock.Anything, mock.MatchedBy(func(spec crud.Specification[entity.SessionParticipant]) bool {
				return spec.Model.SessionID == sessionID && spec.Model.ProfileID == callerID
			})).
			Return(entity.SessionParticipant{BaseEntity: crud.BaseEntity{ID: uuid.New()}, SessionID: sessionID, ProfileID: callerID}, nil)
	}

	expectChooser := func(groupMemberRepo *mocks.MockRepository[entity.GroupMember], groupRepo *mocks.MockRepository[entity.Group]) {
		groupMemberRepo.EXPECT().
			FindAll(mock.Anything, mock.MatchedBy(func(spec crud.Specification[entity.GroupMember]) bool {
				return spec.Model.GroupID == groupID
			})).
			Return([]entity.GroupMember{{ProfileID: chooserProfileID}}, nil)
		groupRepo.EXPECT().
			FindFirst(mock.Anything, mock.Anything).
			Return(entity.Group{BaseEntity: crud.BaseEntity{ID: groupID}, RoundRobinPointer: 0}, nil)
	}

	t.Run("rejects wrong method (including random) with 400", func(t *testing.T) {
		svc, decisionSessionRepo, sessionParticipantRepo, _, _, _, _, _, _, _ := newTestDecisionSessionService(t)
		expectParticipant(sessionParticipantRepo, chooserProfileID)

		session := votingRoundRobinSession()
		session.Method = "random"
		decisionSessionRepo.EXPECT().FindFirst(mock.Anything, mock.Anything).Return(session, nil)

		_, err := svc.Select(context.Background(), chooserProfileID, sessionID, dto.CastVoteRequest{ContentID: candidateOne})

		require.Error(t, err)
		var appErr ungerr.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, http.StatusBadRequest, appErr.HttpStatus())
	})

	t.Run("rejects a non-chooser with 403", func(t *testing.T) {
		svc, decisionSessionRepo, sessionParticipantRepo, _, groupMemberRepo, groupRepo, _, _, _, _ := newTestDecisionSessionService(t)
		expectParticipant(sessionParticipantRepo, otherProfileID)
		decisionSessionRepo.EXPECT().FindFirst(mock.Anything, mock.Anything).Return(votingRoundRobinSession(), nil)
		expectChooser(groupMemberRepo, groupRepo)

		_, err := svc.Select(context.Background(), otherProfileID, sessionID, dto.CastVoteRequest{ContentID: candidateOne})

		require.Error(t, err)
		var appErr ungerr.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, http.StatusForbidden, appErr.HttpStatus())
	})

	t.Run("accepts the correct chooser's pick", func(t *testing.T) {
		svc, decisionSessionRepo, sessionParticipantRepo, _, groupMemberRepo, groupRepo, _, _, sessionVoteRepo, _ := newTestDecisionSessionService(t)
		expectParticipant(sessionParticipantRepo, chooserProfileID)
		decisionSessionRepo.EXPECT().FindFirst(mock.Anything, mock.Anything).Return(votingRoundRobinSession(), nil)
		expectChooser(groupMemberRepo, groupRepo)

		// upsertVote's lookup (empty → Insert), then selectionTally's
		// read-back of the persisted vote — two distinct FindFirst calls on
		// identical args, sequenced with .Once() since testify's mock
		// otherwise always resolves to the first registered match.
		sessionVoteFindFirstMatcher := mock.MatchedBy(func(spec crud.Specification[entity.SessionVote]) bool {
			return spec.Model.SessionID == sessionID && spec.Model.ProfileID == chooserProfileID
		})
		sessionVoteRepo.EXPECT().
			FindFirst(mock.Anything, sessionVoteFindFirstMatcher).
			Return(entity.SessionVote{}, nil).
			Once()
		sessionVoteRepo.EXPECT().
			Insert(mock.Anything, entity.SessionVote{SessionID: sessionID, ProfileID: chooserProfileID, ContentID: candidateOne}).
			Return(entity.SessionVote{}, nil)
		sessionVoteRepo.EXPECT().
			FindFirst(mock.Anything, sessionVoteFindFirstMatcher).
			Return(entity.SessionVote{BaseEntity: crud.BaseEntity{ID: uuid.New()}, SessionID: sessionID, ProfileID: chooserProfileID, ContentID: candidateOne}, nil).
			Once()

		// selectionTally re-derives the chooser via the same (unlimited)
		// expectChooser expectations registered above.

		resp, err := svc.Select(context.Background(), chooserProfileID, sessionID, dto.CastVoteRequest{ContentID: candidateOne})

		require.NoError(t, err)
		tally, ok := resp.Tally.(dto.SelectionTally)
		require.True(t, ok)
		require.NotNil(t, tally.SelectedContentID)
		assert.Equal(t, candidateOne, *tally.SelectedContentID)
	})
}

func TestDecisionSessionService_Finalize(t *testing.T) {
	sessionID := uuid.New()
	groupID := uuid.New()
	profileID := uuid.New()
	candidateOne := uuid.New()
	candidateTwo := uuid.New()

	votingMajoritySession := func() entity.DecisionSession {
		return entity.DecisionSession{
			ID:         sessionID,
			GroupID:    groupID,
			Method:     "majority",
			Status:     "voting",
			RandomSeed: 1,
			Candidates: []entity.SessionCandidate{{ContentID: candidateOne}, {ContentID: candidateTwo}},
		}
	}

	expectParticipant := func(participantRepo *mocks.MockRepository[entity.SessionParticipant]) {
		participantRepo.EXPECT().
			FindFirst(mock.Anything, mock.MatchedBy(func(spec crud.Specification[entity.SessionParticipant]) bool {
				return spec.Model.SessionID == sessionID && spec.Model.ProfileID == profileID
			})).
			Return(entity.SessionParticipant{BaseEntity: crud.BaseEntity{ID: uuid.New()}, SessionID: sessionID, ProfileID: profileID}, nil)
	}

	t.Run("rejects a non-participant with 404", func(t *testing.T) {
		svc, _, sessionParticipantRepo, _, _, _, _, _, _, _ := newTestDecisionSessionService(t)
		sessionParticipantRepo.EXPECT().
			FindFirst(mock.Anything, mock.Anything).
			Return(entity.SessionParticipant{}, nil)

		_, err := svc.Finalize(context.Background(), profileID, sessionID)

		require.Error(t, err)
		var appErr ungerr.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, http.StatusNotFound, appErr.HttpStatus())
	})

	t.Run("rejects a session that isn't open for voting with 409", func(t *testing.T) {
		svc, decisionSessionRepo, sessionParticipantRepo, _, _, _, _, _, _, _ := newTestDecisionSessionService(t)
		expectParticipant(sessionParticipantRepo)

		session := votingMajoritySession()
		session.Status = "completed"
		decisionSessionRepo.EXPECT().FindFirst(mock.Anything, mock.Anything).Return(session, nil)

		_, err := svc.Finalize(context.Background(), profileID, sessionID)

		require.Error(t, err)
		var appErr ungerr.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, http.StatusConflict, appErr.HttpStatus())
	})

	t.Run("majority: dispatches to sessionVoteRepo (not sessionRankingRepo), sets status/winner/finalizedAt, never touches groupRepo", func(t *testing.T) {
		svc, decisionSessionRepo, sessionParticipantRepo, _, _, groupRepo, _, _, sessionVoteRepo, sessionRankingRepo := newTestDecisionSessionService(t)
		expectParticipant(sessionParticipantRepo)

		votingSession := votingMajoritySession()
		decisionSessionRepo.EXPECT().FindFirst(mock.Anything, mock.Anything).Return(votingSession, nil).Once()

		sessionVoteRepo.EXPECT().
			FindAll(mock.Anything, mock.MatchedBy(func(spec crud.Specification[entity.SessionVote]) bool {
				return spec.Model.SessionID == sessionID
			})).
			Return([]entity.SessionVote{
				{SessionID: sessionID, ProfileID: uuid.New(), ContentID: candidateOne},
				{SessionID: sessionID, ProfileID: uuid.New(), ContentID: candidateOne},
				{SessionID: sessionID, ProfileID: uuid.New(), ContentID: candidateTwo},
			}, nil)

		decisionSessionRepo.EXPECT().
			Update(mock.Anything, mock.MatchedBy(func(s entity.DecisionSession) bool {
				return s.Status == "completed" &&
					s.WinnerContentID != nil && *s.WinnerContentID == candidateOne &&
					s.FinalizedAt != nil
			})).
			Return(entity.DecisionSession{}, nil)

		completedSession := votingSession
		completedSession.Status = "completed"
		winner := candidateOne
		completedSession.WinnerContentID = &winner
		now := time.Now()
		completedSession.FinalizedAt = &now
		decisionSessionRepo.EXPECT().FindFirst(mock.Anything, mock.Anything).Return(completedSession, nil).Once()

		resp, err := svc.Finalize(context.Background(), profileID, sessionID)

		require.NoError(t, err)
		assert.Equal(t, "completed", resp.Status)
		require.NotNil(t, resp.WinnerContentID)
		assert.Equal(t, candidateOne, *resp.WinnerContentID)
		assert.NotNil(t, resp.FinalizedAt)

		sessionRankingRepo.AssertNotCalled(t, "FindAll", mock.Anything, mock.Anything)
		groupRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	})

	t.Run("ranked: dispatches to sessionRankingRepo, not sessionVoteRepo", func(t *testing.T) {
		svc, decisionSessionRepo, sessionParticipantRepo, _, _, _, _, _, sessionVoteRepo, sessionRankingRepo := newTestDecisionSessionService(t)
		expectParticipant(sessionParticipantRepo)

		session := votingMajoritySession()
		session.Method = "ranked"
		decisionSessionRepo.EXPECT().FindFirst(mock.Anything, mock.Anything).Return(session, nil).Once()

		sessionRankingRepo.EXPECT().
			FindAll(mock.Anything, mock.MatchedBy(func(spec crud.Specification[entity.SessionRanking]) bool {
				return spec.Model.SessionID == sessionID
			})).
			Return([]entity.SessionRanking{
				{SessionID: sessionID, ProfileID: uuid.New(), ContentID: candidateOne, Rank: 1},
			}, nil)

		decisionSessionRepo.EXPECT().
			Update(mock.Anything, mock.MatchedBy(func(s entity.DecisionSession) bool {
				return s.Status == "completed" && s.WinnerContentID != nil
			})).
			Return(entity.DecisionSession{}, nil)

		completedSession := session
		completedSession.Status = "completed"
		winner := candidateOne
		completedSession.WinnerContentID = &winner
		decisionSessionRepo.EXPECT().FindFirst(mock.Anything, mock.Anything).Return(completedSession, nil).Once()

		_, err := svc.Finalize(context.Background(), profileID, sessionID)

		require.NoError(t, err)
		sessionVoteRepo.AssertNotCalled(t, "FindAll", mock.Anything, mock.Anything)
	})

	t.Run("round_robin: advances groups.round_robin_pointer", func(t *testing.T) {
		svc, decisionSessionRepo, sessionParticipantRepo, _, groupMemberRepo, groupRepo, _, _, sessionVoteRepo, _ := newTestDecisionSessionService(t)
		expectParticipant(sessionParticipantRepo)

		session := votingMajoritySession()
		session.Method = "round_robin"
		session.Participants = []entity.SessionParticipant{{SessionID: sessionID, ProfileID: profileID}}
		decisionSessionRepo.EXPECT().FindFirst(mock.Anything, mock.Anything).Return(session, nil).Once()

		groupMemberRepo.EXPECT().
			FindAll(mock.Anything, mock.MatchedBy(func(spec crud.Specification[entity.GroupMember]) bool {
				return spec.Model.GroupID == groupID
			})).
			Return([]entity.GroupMember{{ProfileID: profileID}}, nil)
		groupRepo.EXPECT().
			FindFirst(mock.Anything, mock.Anything).
			Return(entity.Group{BaseEntity: crud.BaseEntity{ID: groupID}, RoundRobinPointer: 0}, nil)

		sessionVoteRepo.EXPECT().
			FindFirst(mock.Anything, mock.MatchedBy(func(spec crud.Specification[entity.SessionVote]) bool {
				return spec.Model.SessionID == sessionID && spec.Model.ProfileID == profileID
			})).
			Return(entity.SessionVote{BaseEntity: crud.BaseEntity{ID: uuid.New()}, SessionID: sessionID, ProfileID: profileID, ContentID: candidateOne}, nil)

		decisionSessionRepo.EXPECT().
			Update(mock.Anything, mock.MatchedBy(func(s entity.DecisionSession) bool {
				return s.Status == "completed"
			})).
			Return(entity.DecisionSession{}, nil)

		groupRepo.EXPECT().
			Update(mock.Anything, mock.MatchedBy(func(g entity.Group) bool {
				return g.RoundRobinPointer == 1
			})).
			Return(entity.Group{}, nil)

		completedSession := session
		completedSession.Status = "completed"
		winner := candidateOne
		completedSession.WinnerContentID = &winner
		decisionSessionRepo.EXPECT().FindFirst(mock.Anything, mock.Anything).Return(completedSession, nil).Once()

		_, err := svc.Finalize(context.Background(), profileID, sessionID)

		require.NoError(t, err)
	})
}
