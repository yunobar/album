package service

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
	"github.com/itsLeonB/ungerr"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/yunobar/album/internal/appconstant"
	"github.com/yunobar/album/internal/core/logger"
	"github.com/yunobar/album/internal/core/otel"
	"github.com/yunobar/album/internal/core/pubsub"
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/entity"
	"github.com/yunobar/album/internal/domain/mapper"
	"github.com/yunobar/album/internal/domain/repository"
	"github.com/yunobar/album/internal/domain/service/decisionmethod"
	"go.opentelemetry.io/otel/trace"
)

// pgUniqueViolation is Postgres SQLSTATE 23505.
const pgUniqueViolation = "23505"

// oneLiveSessionPerGroupIndex is the partial unique index name from the
// one_live_session_per_group migration (ADR-0006) — checked by constraint
// name so an unrelated unique violation elsewhere doesn't get misreported as
// "a session is already live".
const oneLiveSessionPerGroupIndex = "one_live_session_per_group"

// isOneLiveSessionViolation reports whether err is the DB rejecting a second
// concurrent create for a group that already has a session in "voting" — the
// index, not an app-level read-then-check, is what makes this race-safe
// (ADR-0006). go-crud's Insert wraps driver errors in *ungerr.UnknownError,
// which has no Unwrap(), so errors.As must run on the unwrapped cause rather
// than the returned err directly.
func isOneLiveSessionViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(ungerr.Unwrap(err), &pgErr) {
		return false
	}
	return pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == oneLiveSessionPerGroupIndex
}

type DecisionSessionService interface {
	Create(ctx context.Context, profileID, groupID uuid.UUID, request dto.CreateSessionRequest) (dto.SessionResponse, error)
	Get(ctx context.Context, profileID, sessionID uuid.UUID) (dto.SessionResponse, error)
	CastVote(ctx context.Context, profileID, sessionID uuid.UUID, request dto.CastVoteRequest) (dto.TallyResponse, error)
	SubmitRanking(ctx context.Context, profileID, sessionID uuid.UUID, request dto.SubmitRankingRequest) (dto.TallyResponse, error)
	// Select is Round Robin's chooser pick — reuses CastVoteRequest's
	// {contentId} shape and the session_votes table (see this file's
	// upsertVote doc comment).
	Select(ctx context.Context, profileID, sessionID uuid.UUID, request dto.CastVoteRequest) (dto.TallyResponse, error)
	// Finalize dispatches to the session's method resolver, sets the
	// winner/status/finalizedAt atomically, and — for round_robin — advances
	// groups.round_robin_pointer. The session row is loaded with ForUpdate
	// inside the transaction, so its status=="voting" guard also serializes
	// concurrent Finalize calls on the same session: only the first commits,
	// the second sees status=="completed" and 409s instead of racing to
	// silently overwrite the winner.
	Finalize(ctx context.Context, profileID, sessionID uuid.UUID) (dto.SessionResponse, error)
	// VerifyParticipant is requireParticipant exported for callers outside
	// this service (the WS handler, which must reject non-participants
	// before upgrading the connection).
	VerifyParticipant(ctx context.Context, profileID, sessionID uuid.UUID) error
}

type decisionSessionServiceImpl struct {
	transactor                  crud.Transactor
	decisionSessionRepo         crud.Repository[entity.DecisionSession]
	sessionParticipantRepo      crud.Repository[entity.SessionParticipant]
	sessionCandidateRepo        crud.Repository[entity.SessionCandidate]
	groupMemberRepo             crud.Repository[entity.GroupMember]
	groupRepo                   repository.GroupRepository
	sessionPrioritySnapshotRepo crud.Repository[entity.SessionPrioritySnapshot]
	sessionVoteRepo             crud.Repository[entity.SessionVote]
	sessionRankingRepo          crud.Repository[entity.SessionRanking]
	publisher                   pubsub.Publisher
	strategies                  map[string]decisionmethod.Strategy
}

func NewDecisionSessionService(
	transactor crud.Transactor,
	decisionSessionRepo crud.Repository[entity.DecisionSession],
	sessionParticipantRepo crud.Repository[entity.SessionParticipant],
	sessionCandidateRepo crud.Repository[entity.SessionCandidate],
	groupMemberRepo crud.Repository[entity.GroupMember],
	groupRepo repository.GroupRepository,
	watchlistRepo crud.Repository[entity.WatchlistItem],
	sessionPrioritySnapshotRepo crud.Repository[entity.SessionPrioritySnapshot],
	sessionVoteRepo crud.Repository[entity.SessionVote],
	sessionRankingRepo crud.Repository[entity.SessionRanking],
	publisher pubsub.Publisher,
) DecisionSessionService {
	return &decisionSessionServiceImpl{
		transactor,
		decisionSessionRepo,
		sessionParticipantRepo,
		sessionCandidateRepo,
		groupMemberRepo,
		groupRepo,
		sessionPrioritySnapshotRepo,
		sessionVoteRepo,
		sessionRankingRepo,
		publisher,
		decisionmethod.NewRegistry(
			groupRepo,
			groupMemberRepo,
			watchlistRepo,
			sessionVoteRepo,
			sessionRankingRepo,
			sessionPrioritySnapshotRepo,
		),
	}
}

func (dss *decisionSessionServiceImpl) Create(ctx context.Context, profileID, groupID uuid.UUID, request dto.CreateSessionRequest) (dto.SessionResponse, error) {
	ctx, span := otel.Tracer.Start(ctx, "DecisionSessionService.Create")
	defer span.End()

	var sessionID uuid.UUID

	err := dss.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		memberSpec := crud.Specification[entity.GroupMember]{}
		memberSpec.Model.GroupID = groupID
		members, err := dss.groupMemberRepo.FindAll(ctx, memberSpec)
		if err != nil {
			return err
		}

		memberIDs := make(map[uuid.UUID]struct{}, len(members))
		isCallerMember := false
		for _, m := range members {
			memberIDs[m.ProfileID] = struct{}{}
			if m.ProfileID == profileID {
				isCallerMember = true
			}
		}
		if !isCallerMember {
			return ungerr.NotFoundError(fmt.Sprintf("group %s is not found", groupID))
		}

		// Cheap pre-check for the common case (a friendly 409 without
		// tripping the unique index). Not a substitute for the index below —
		// two concurrent creates can both pass this check before either
		// commits, so isOneLiveSessionViolation still guards the actual
		// race (ADR-0006).
		liveSpec := crud.Specification[entity.DecisionSession]{}
		liveSpec.Model.GroupID = groupID
		liveSpec.Model.Status = appconstant.SessionStatusVoting
		existing, err := dss.decisionSessionRepo.FindFirst(ctx, liveSpec)
		if err != nil {
			return err
		}
		if existing.ID != uuid.Nil {
			return ungerr.ConflictError(fmt.Sprintf("group %s already has a session in voting", groupID))
		}

		isCallerParticipant := false
		seenParticipants := make(map[uuid.UUID]struct{}, len(request.ParticipantIDs))
		uniqueParticipantIDs := make([]uuid.UUID, 0, len(request.ParticipantIDs))
		for _, pid := range request.ParticipantIDs {
			if _, ok := memberIDs[pid]; !ok {
				return ungerr.BadRequestError(fmt.Sprintf("participant %s is not a member of group %s", pid, groupID))
			}
			if pid == profileID {
				isCallerParticipant = true
			}
			if _, dup := seenParticipants[pid]; dup {
				continue
			}
			seenParticipants[pid] = struct{}{}
			uniqueParticipantIDs = append(uniqueParticipantIDs, pid)
		}
		// The caller must be able to see the session they just created —
		// Get (and every other session-scoped read/write) is strictly
		// participant-scoped, non-negotiably, so a creator who excludes
		// themselves would otherwise get a 201 whose own follow-up Get
		// reload 404s. Enforcing this at validation time keeps that
		// invariant uniform instead of special-casing the post-create load.
		if !isCallerParticipant {
			return ungerr.BadRequestError(fmt.Sprintf("caller %s must be included in participantIds", profileID))
		}
		request.ParticipantIDs = uniqueParticipantIDs

		validCandidates, err := dss.mergedContentIDs(ctx, groupID)
		if err != nil {
			return err
		}
		seenCandidates := make(map[uuid.UUID]struct{}, len(request.CandidateContentIDs))
		uniqueCandidateIDs := make([]uuid.UUID, 0, len(request.CandidateContentIDs))
		for _, cid := range request.CandidateContentIDs {
			if _, ok := validCandidates[cid]; !ok {
				return ungerr.BadRequestError(fmt.Sprintf("content %s is not on the group's merged watchlist", cid))
			}
			if _, dup := seenCandidates[cid]; dup {
				continue
			}
			seenCandidates[cid] = struct{}{}
			uniqueCandidateIDs = append(uniqueCandidateIDs, cid)
		}
		request.CandidateContentIDs = uniqueCandidateIDs

		seed, err := generateRandomSeed()
		if err != nil {
			return err
		}

		session, err := dss.decisionSessionRepo.Insert(ctx, entity.DecisionSession{
			GroupID:    groupID,
			Method:     mapper.MethodToDB(request.Method),
			Status:     appconstant.SessionStatusVoting,
			RandomSeed: seed,
		})
		if err != nil {
			if isOneLiveSessionViolation(err) {
				return ungerr.ConflictError(fmt.Sprintf("group %s already has a session in voting", groupID))
			}
			return err
		}

		participants := make([]entity.SessionParticipant, len(request.ParticipantIDs))
		for i, pid := range request.ParticipantIDs {
			participants[i] = entity.SessionParticipant{SessionID: session.ID, ProfileID: pid}
		}
		if _, err := dss.sessionParticipantRepo.InsertMany(ctx, participants); err != nil {
			return err
		}

		candidates := make([]entity.SessionCandidate, len(request.CandidateContentIDs))
		for i, cid := range request.CandidateContentIDs {
			candidates[i] = entity.SessionCandidate{SessionID: session.ID, ContentID: cid}
		}
		if _, err := dss.sessionCandidateRepo.InsertMany(ctx, candidates); err != nil {
			return err
		}

		if err := dss.strategies[session.Method].OnCreate(ctx, session, request.ParticipantIDs, request.CandidateContentIDs); err != nil {
			return err
		}

		sessionID = session.ID
		return nil
	})
	if err != nil {
		return dto.SessionResponse{}, err
	}

	// InsertMany doesn't return rows with Profile/Content preloaded (no join
	// happened), and the mapper needs those — reload via Get after commit
	// instead of hydrating them here by hand.
	return dss.Get(ctx, profileID, sessionID)
}

// mergedContentIDs is the candidate universe for a session — the DISTINCT
// content actively watchlisted by any group member — as a lookup set for
// candidate validation. The query lives in GroupRepository.
func (dss *decisionSessionServiceImpl) mergedContentIDs(ctx context.Context, groupID uuid.UUID) (map[uuid.UUID]struct{}, error) {
	ids, err := dss.groupRepo.FindMergedContentIDs(ctx, groupID)
	if err != nil {
		return nil, err
	}

	set := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set, nil
}

// generateRandomSeed is crypto/rand-backed, not math/rand, per the Global
// Constraints reproducibility note (the seed is persisted and later reused
// by the Random resolver, so it must not be predictable).
func generateRandomSeed() (int64, error) {
	buf := make([]byte, 8)
	if _, err := cryptorand.Read(buf); err != nil {
		return 0, ungerr.Wrap(err, "error generating random seed")
	}
	return int64(binary.BigEndian.Uint64(buf)), nil
}

func (dss *decisionSessionServiceImpl) Get(ctx context.Context, profileID, sessionID uuid.UUID) (dto.SessionResponse, error) {
	ctx, span := otel.Tracer.Start(ctx, "DecisionSessionService.Get")
	defer span.End()

	// A separate, cheap lookup before the main FindFirst — go-crud's
	// PreloadRelations can't express "only preload if a matching
	// participant row exists for X", and this keeps the 404-for-non-
	// participant path from touching the bigger preloaded query.
	if err := dss.requireParticipant(ctx, sessionID, profileID); err != nil {
		return dto.SessionResponse{}, err
	}

	sessionSpec := crud.Specification[entity.DecisionSession]{
		PreloadRelations: []string{"Participants.Profile", "Candidates.Content"},
	}
	sessionSpec.Model.ID = sessionID
	session, err := dss.decisionSessionRepo.FindFirst(ctx, sessionSpec)
	if err != nil {
		return dto.SessionResponse{}, err
	}
	if session.ID == uuid.Nil {
		return dto.SessionResponse{}, ungerr.NotFoundError(fmt.Sprintf("session %s is not found", sessionID))
	}

	strat := dss.strategies[session.Method]

	chooser, err := strat.Chooser(ctx, session)
	if err != nil {
		return dto.SessionResponse{}, err
	}

	tally, err := strat.Tally(ctx, session)
	if err != nil {
		return dto.SessionResponse{}, err
	}

	return mapper.SessionToResponse(session, chooser, tally), nil
}

// requireParticipant returns the same NotFoundError whether the session
// doesn't exist or the caller isn't a participant — the non-disclosure
// rule every session-scoped endpoint follows.
func (dss *decisionSessionServiceImpl) requireParticipant(ctx context.Context, sessionID, profileID uuid.UUID) error {
	spec := crud.Specification[entity.SessionParticipant]{}
	spec.Model.SessionID = sessionID
	spec.Model.ProfileID = profileID
	participation, err := dss.sessionParticipantRepo.FindFirst(ctx, spec)
	if err != nil {
		return err
	}
	if participation.IsZero() {
		return ungerr.NotFoundError(fmt.Sprintf("session %s is not found", sessionID))
	}
	return nil
}

// VerifyParticipant is requireParticipant exported for callers outside this
// service (the WS handler, which must reject non-participants before
// upgrading the connection).
func (dss *decisionSessionServiceImpl) VerifyParticipant(ctx context.Context, profileID, sessionID uuid.UUID) error {
	return dss.requireParticipant(ctx, sessionID, profileID)
}

// lockVotingSession loads a session with ForUpdate and verifies it's still
// "voting" (409 otherwise). Must be called from inside a
// transactor.WithinTransaction closure — the row lock only holds for that
// transaction's lifetime, so calling this standalone gives no protection
// against a concurrent Finalize interleaving between this check and the
// caller's write (the exact TOCTOU class Finalize itself was fixed for;
// every mutating endpoint needs the same guarantee, not just Finalize).
func (dss *decisionSessionServiceImpl) lockVotingSession(ctx context.Context, sessionID uuid.UUID, preloads []string) (entity.DecisionSession, error) {
	spec := crud.Specification[entity.DecisionSession]{PreloadRelations: preloads, ForUpdate: true}
	spec.Model.ID = sessionID
	session, err := dss.decisionSessionRepo.FindFirst(ctx, spec)
	if err != nil {
		return entity.DecisionSession{}, err
	}
	if session.ID == uuid.Nil {
		return entity.DecisionSession{}, ungerr.NotFoundError(fmt.Sprintf("session %s is not found", sessionID))
	}
	if session.Status != appconstant.SessionStatusVoting {
		return entity.DecisionSession{}, ungerr.ConflictError(fmt.Sprintf("session %s is not open for voting", sessionID))
	}

	return session, nil
}

func containsCandidate(candidates []entity.SessionCandidate, contentID uuid.UUID) bool {
	for _, c := range candidates {
		if c.ContentID == contentID {
			return true
		}
	}
	return false
}

// upsertVote backs both CastVote (Majority) and Select (Round Robin) — one
// row per (session_id, profile_id), replaced on resubmit.
func (dss *decisionSessionServiceImpl) upsertVote(ctx context.Context, sessionID, profileID, contentID uuid.UUID) error {
	spec := crud.Specification[entity.SessionVote]{}
	spec.Model.SessionID = sessionID
	spec.Model.ProfileID = profileID
	existing, err := dss.sessionVoteRepo.FindFirst(ctx, spec)
	if err != nil {
		return err
	}

	if existing.IsZero() {
		_, err = dss.sessionVoteRepo.Insert(ctx, entity.SessionVote{SessionID: sessionID, ProfileID: profileID, ContentID: contentID})
		return err
	}

	existing.ContentID = contentID
	_, err = dss.sessionVoteRepo.Update(ctx, existing)
	return err
}

func (dss *decisionSessionServiceImpl) CastVote(ctx context.Context, profileID, sessionID uuid.UUID, request dto.CastVoteRequest) (dto.TallyResponse, error) {
	ctx, span := otel.Tracer.Start(ctx, "DecisionSessionService.CastVote")
	defer span.End()

	if err := dss.requireParticipant(ctx, sessionID, profileID); err != nil {
		return dto.TallyResponse{}, err
	}

	// Session lock + status check + the write all happen inside one
	// transaction, so a vote can't sneak in after a concurrent Finalize has
	// already locked and completed this session — the same TOCTOU class
	// Finalize itself was fixed for.
	var session entity.DecisionSession
	err := dss.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		var err error
		session, err = dss.lockVotingSession(ctx, sessionID, []string{"Candidates"})
		if err != nil {
			return err
		}
		if session.Method != appconstant.SessionMethodMajority {
			return ungerr.BadRequestError(fmt.Sprintf("session %s is not a majority session", sessionID))
		}
		if !containsCandidate(session.Candidates, request.ContentID) {
			return ungerr.BadRequestError(fmt.Sprintf("content %s is not a session candidate", request.ContentID))
		}
		return dss.upsertVote(ctx, sessionID, profileID, request.ContentID)
	})
	if err != nil {
		return dto.TallyResponse{}, err
	}

	tally, err := dss.strategies[session.Method].Tally(ctx, session)
	if err != nil {
		return dto.TallyResponse{}, err
	}
	dss.publishTally(ctx, sessionID, tally)
	return dto.TallyResponse{Tally: tally}, nil
}

func (dss *decisionSessionServiceImpl) SubmitRanking(ctx context.Context, profileID, sessionID uuid.UUID, request dto.SubmitRankingRequest) (dto.TallyResponse, error) {
	ctx, span := otel.Tracer.Start(ctx, "DecisionSessionService.SubmitRanking")
	defer span.End()

	if err := dss.requireParticipant(ctx, sessionID, profileID); err != nil {
		return dto.TallyResponse{}, err
	}

	// Session lock + status check + validation + the delete-then-reinsert
	// all happen inside one transaction: the lock prevents a ballot sneaking
	// in after a concurrent Finalize completes (same TOCTOU class Finalize
	// itself was fixed for), and it's the same transaction the delete-then-
	// reinsert already needed for its own atomicity (a successful DeleteMany
	// followed by a failed InsertMany must not leave the ballot at zero rows).
	var session entity.DecisionSession
	err := dss.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		var err error
		session, err = dss.lockVotingSession(ctx, sessionID, []string{"Candidates"})
		if err != nil {
			return err
		}
		if session.Method != appconstant.SessionMethodRanked {
			return ungerr.BadRequestError(fmt.Sprintf("session %s is not a ranked session", sessionID))
		}

		seen := make(map[uuid.UUID]struct{}, len(request.Ranking))
		for _, cid := range request.Ranking {
			if _, dup := seen[cid]; dup {
				return ungerr.BadRequestError(fmt.Sprintf("ranking has duplicate content %s", cid))
			}
			seen[cid] = struct{}{}
			if !containsCandidate(session.Candidates, cid) {
				return ungerr.BadRequestError(fmt.Sprintf("content %s is not a session candidate", cid))
			}
		}

		existingSpec := crud.Specification[entity.SessionRanking]{}
		existingSpec.Model.SessionID = sessionID
		existingSpec.Model.ProfileID = profileID
		existing, err := dss.sessionRankingRepo.FindAll(ctx, existingSpec)
		if err != nil {
			return err
		}
		if len(existing) > 0 {
			if err := dss.sessionRankingRepo.DeleteMany(ctx, existing); err != nil {
				return err
			}
		}

		newRankings := make([]entity.SessionRanking, len(request.Ranking))
		for i, cid := range request.Ranking {
			newRankings[i] = entity.SessionRanking{SessionID: sessionID, ProfileID: profileID, ContentID: cid, Rank: i + 1}
		}
		_, err = dss.sessionRankingRepo.InsertMany(ctx, newRankings)
		return err
	})
	if err != nil {
		return dto.TallyResponse{}, err
	}

	tally, err := dss.strategies[session.Method].Tally(ctx, session)
	if err != nil {
		return dto.TallyResponse{}, err
	}
	dss.publishTally(ctx, sessionID, tally)
	return dto.TallyResponse{Tally: tally}, nil
}

func (dss *decisionSessionServiceImpl) Select(ctx context.Context, profileID, sessionID uuid.UUID, request dto.CastVoteRequest) (dto.TallyResponse, error) {
	ctx, span := otel.Tracer.Start(ctx, "DecisionSessionService.Select")
	defer span.End()

	if err := dss.requireParticipant(ctx, sessionID, profileID); err != nil {
		return dto.TallyResponse{}, err
	}

	// Session lock + status check + chooser authorization + the write all
	// happen inside one transaction, so a pick can't sneak in after a
	// concurrent Finalize has already locked and completed this session —
	// the same TOCTOU class Finalize itself was fixed for.
	var session entity.DecisionSession
	var strat decisionmethod.Strategy
	err := dss.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		var err error
		session, err = dss.lockVotingSession(ctx, sessionID, []string{"Candidates", "Participants"})
		if err != nil {
			return err
		}
		if session.Method != appconstant.SessionMethodRoundRobin {
			return ungerr.BadRequestError(fmt.Sprintf("session %s is not a round robin session", sessionID))
		}

		strat = dss.strategies[session.Method]

		chooser, err := strat.Chooser(ctx, session)
		if err != nil {
			return err
		}
		if chooser == nil || *chooser != profileID {
			return ungerr.ForbiddenError(fmt.Sprintf("profile %s is not the current chooser for session %s", profileID, sessionID))
		}
		if !containsCandidate(session.Candidates, request.ContentID) {
			return ungerr.BadRequestError(fmt.Sprintf("content %s is not a session candidate", request.ContentID))
		}
		return dss.upsertVote(ctx, sessionID, profileID, request.ContentID)
	})
	if err != nil {
		return dto.TallyResponse{}, err
	}

	tally, err := strat.Tally(ctx, session)
	if err != nil {
		return dto.TallyResponse{}, err
	}
	dss.publishTally(ctx, sessionID, tally)
	return dto.TallyResponse{Tally: tally}, nil
}

// publishTally is the shared best-effort publish behind CastVote,
// SubmitRanking, and Select — a publish failure (marshal error or transport
// error) must never fail the request: the vote/ranking/select itself
// already succeeded and was persisted, and a client that misses the live
// update re-fetches GET /sessions/:id per the API contract's reconnect
// story. The failure is still surfaced, not silently dropped: logged, and
// recorded on the caller's span without raising its status to Error (the
// request itself didn't fail — only the best-effort live push did).
func (dss *decisionSessionServiceImpl) publishTally(ctx context.Context, sessionID uuid.UUID, tally any) {
	span := trace.SpanFromContext(ctx)

	data, err := json.Marshal(dto.LiveTallyMessage{Type: "tally", Tally: tally})
	if err != nil {
		logger.Errorf("error marshaling tally for session %s: %v", sessionID, err)
		span.RecordError(err)
		return
	}
	if err := dss.publisher.Publish(pubsub.LiveSubject(sessionID), data); err != nil {
		logger.Errorf("error publishing tally for session %s: %v", sessionID, err)
		span.RecordError(err)
	}
}

func (dss *decisionSessionServiceImpl) Finalize(ctx context.Context, profileID, sessionID uuid.UUID) (dto.SessionResponse, error) {
	ctx, span := otel.Tracer.Start(ctx, "DecisionSessionService.Finalize")
	defer span.End()

	// The participant-existence check has no race (a participant row's
	// existence doesn't change during finalize), so it stays outside the
	// transaction — mirrors loadVotingSession's own separation.
	if err := dss.requireParticipant(ctx, sessionID, profileID); err != nil {
		return dto.SessionResponse{}, err
	}

	now := time.Now()
	var winnerID uuid.UUID

	err := dss.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		// Locked here (ForUpdate), so a concurrent Finalize on the same
		// session blocks on this same guard instead of racing this one to
		// compute-then-overwrite the winner (TOCTOU double-finalize).
		session, err := dss.lockVotingSession(ctx, sessionID, []string{"Participants.Profile", "Candidates.Content"})
		if err != nil {
			return err
		}

		// Dispatches to the session's method strategy — round_robin's
		// Resolve additionally advances groups.round_robin_pointer, using
		// the same ForUpdate-locked group fetch for both the chooser
		// computation and the pointer increment, inside this same
		// transaction (see decisionmethod.roundRobinStrategy.Resolve).
		winnerID, err = dss.strategies[session.Method].Resolve(ctx, session)
		if err != nil {
			return err
		}

		session.WinnerContentID = &winnerID
		session.Status = appconstant.SessionStatusCompleted
		session.FinalizedAt = &now
		if _, err := dss.decisionSessionRepo.Update(ctx, session); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return dto.SessionResponse{}, err
	}

	// Published after the transaction commits successfully, using the
	// already-known winnerID/now — never publish a winner that didn't
	// actually get persisted. A publish failure is logged and recorded on
	// this span (status stays as-is, not Error — Finalize itself already
	// succeeded) rather than silently dropped.
	data, err := json.Marshal(dto.LiveWinnerMessage{
		Type:            "winner",
		WinnerContentID: winnerID,
		FinalizedAt:     now,
	})
	if err != nil {
		logger.Errorf("error marshaling winner message for session %s: %v", sessionID, err)
		span.RecordError(err)
	} else if err := dss.publisher.Publish(pubsub.LiveSubject(sessionID), data); err != nil {
		logger.Errorf("error publishing winner for session %s: %v", sessionID, err)
		span.RecordError(err)
	}

	return dss.Get(ctx, profileID, sessionID)
}
