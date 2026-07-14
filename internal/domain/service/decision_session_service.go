package service

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
	"github.com/itsLeonB/ungerr"
	"github.com/yunobar/album/internal/appconstant"
	"github.com/yunobar/album/internal/core/otel"
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/entity"
	"github.com/yunobar/album/internal/domain/mapper"
)

type DecisionSessionService interface {
	Create(ctx context.Context, profileID, groupID uuid.UUID, request dto.CreateSessionRequest) (dto.SessionResponse, error)
	Get(ctx context.Context, profileID, sessionID uuid.UUID) (dto.SessionResponse, error)
	// CapturePrioritySnapshots freezes each participant's current watchlist
	// priority for every candidate they have actively watchlisted, at
	// session-creation time. A participant with no active watchlist_items
	// row for a candidate gets no snapshot row — the Priority-Based
	// resolver (Task 4) treats a missing row as weight 0.
	CapturePrioritySnapshots(ctx context.Context, sessionID uuid.UUID, participantIDs, candidateIDs []uuid.UUID) error
	CastVote(ctx context.Context, profileID, sessionID uuid.UUID, request dto.CastVoteRequest) (dto.TallyResponse, error)
	SubmitRanking(ctx context.Context, profileID, sessionID uuid.UUID, request dto.SubmitRankingRequest) (dto.TallyResponse, error)
	// Select is Round Robin's chooser pick — reuses CastVoteRequest's
	// {contentId} shape and the session_votes table (see this file's
	// upsertVote doc comment).
	Select(ctx context.Context, profileID, sessionID uuid.UUID, request dto.CastVoteRequest) (dto.TallyResponse, error)
}

type decisionSessionServiceImpl struct {
	transactor                  crud.Transactor
	decisionSessionRepo         crud.Repository[entity.DecisionSession]
	sessionParticipantRepo      crud.Repository[entity.SessionParticipant]
	sessionCandidateRepo        crud.Repository[entity.SessionCandidate]
	groupMemberRepo             crud.Repository[entity.GroupMember]
	groupRepo                   crud.Repository[entity.Group]
	watchlistRepo               crud.Repository[entity.WatchlistItem]
	sessionPrioritySnapshotRepo crud.Repository[entity.SessionPrioritySnapshot]
	sessionVoteRepo             crud.Repository[entity.SessionVote]
	sessionRankingRepo          crud.Repository[entity.SessionRanking]
}

func NewDecisionSessionService(
	transactor crud.Transactor,
	decisionSessionRepo crud.Repository[entity.DecisionSession],
	sessionParticipantRepo crud.Repository[entity.SessionParticipant],
	sessionCandidateRepo crud.Repository[entity.SessionCandidate],
	groupMemberRepo crud.Repository[entity.GroupMember],
	groupRepo crud.Repository[entity.Group],
	watchlistRepo crud.Repository[entity.WatchlistItem],
	sessionPrioritySnapshotRepo crud.Repository[entity.SessionPrioritySnapshot],
	sessionVoteRepo crud.Repository[entity.SessionVote],
	sessionRankingRepo crud.Repository[entity.SessionRanking],
) DecisionSessionService {
	return &decisionSessionServiceImpl{
		transactor,
		decisionSessionRepo,
		sessionParticipantRepo,
		sessionCandidateRepo,
		groupMemberRepo,
		groupRepo,
		watchlistRepo,
		sessionPrioritySnapshotRepo,
		sessionVoteRepo,
		sessionRankingRepo,
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

		for _, pid := range request.ParticipantIDs {
			if _, ok := memberIDs[pid]; !ok {
				return ungerr.BadRequestError(fmt.Sprintf("participant %s is not a member of group %s", pid, groupID))
			}
		}

		validCandidates, err := dss.mergedContentIDs(ctx, groupID)
		if err != nil {
			return err
		}
		for _, cid := range request.CandidateContentIDs {
			if _, ok := validCandidates[cid]; !ok {
				return ungerr.BadRequestError(fmt.Sprintf("content %s is not on the group's merged watchlist", cid))
			}
		}

		seed, err := generateRandomSeed()
		if err != nil {
			return err
		}

		session, err := dss.decisionSessionRepo.Insert(ctx, entity.DecisionSession{
			GroupID:    groupID,
			Method:     mapper.MethodToDB(request.Method),
			Status:     "voting",
			RandomSeed: seed,
		})
		if err != nil {
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

		if session.Method == "priority" {
			if err := dss.CapturePrioritySnapshots(ctx, session.ID, request.ParticipantIDs, request.CandidateContentIDs); err != nil {
				return err
			}
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

// mergedContentIDs is the subset of GetMergedWatchlist's raw SQL needed here
// — just the distinct content IDs, no aggregation.
func (dss *decisionSessionServiceImpl) mergedContentIDs(ctx context.Context, groupID uuid.UUID) (map[uuid.UUID]struct{}, error) {
	db, err := dss.watchlistRepo.GetGormInstance(ctx)
	if err != nil {
		return nil, err
	}

	var ids []uuid.UUID
	err = db.Raw(`
		SELECT DISTINCT wi.content_id
		FROM group_members gm
		JOIN watchlist_items wi ON wi.profile_id = gm.profile_id AND wi.status = ?
		WHERE gm.group_id = ?
	`, appconstant.WatchlistStatusActive, groupID).Scan(&ids).Error
	if err != nil {
		return nil, ungerr.Wrap(err, "error querying merged watchlist content ids")
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
	participantSpec := crud.Specification[entity.SessionParticipant]{}
	participantSpec.Model.SessionID = sessionID
	participantSpec.Model.ProfileID = profileID
	participation, err := dss.sessionParticipantRepo.FindFirst(ctx, participantSpec)
	if err != nil {
		return dto.SessionResponse{}, err
	}
	if participation.IsZero() {
		return dto.SessionResponse{}, ungerr.NotFoundError(fmt.Sprintf("session %s is not found", sessionID))
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

	var chooser *uuid.UUID
	var tally any

	switch session.Method {
	case "majority":
		t, err := dss.majorityTally(ctx, session.ID)
		if err != nil {
			return dto.SessionResponse{}, err
		}
		tally = t
	case "ranked":
		t, err := dss.rankedTally(ctx, session)
		if err != nil {
			return dto.SessionResponse{}, err
		}
		tally = t
	case "round_robin":
		chooser, err = dss.currentChooser(ctx, session)
		if err != nil {
			return dto.SessionResponse{}, err
		}
		t, err := dss.selectionTally(ctx, session)
		if err != nil {
			return dto.SessionResponse{}, err
		}
		tally = t
	}

	return mapper.SessionToResponse(session, chooser, tally), nil
}

// currentChooser is Round Robin only — Random has no chooser concept and
// this is never called for method == "random". Duplicates
// group_service.go's membersOf join-order idiom rather than exporting it
// from GroupService — deliberate, matching this codebase's established
// pattern of each service depending only on repos, never on other services.
func (dss *decisionSessionServiceImpl) currentChooser(ctx context.Context, session entity.DecisionSession) (*uuid.UUID, error) {
	memberSpec := crud.Specification[entity.GroupMember]{}
	memberSpec.Model.GroupID = session.GroupID
	members, err := dss.groupMemberRepo.FindAll(ctx, memberSpec)
	if err != nil {
		return nil, err
	}
	sort.Slice(members, func(i, j int) bool {
		return members[i].ID.String() < members[j].ID.String()
	})

	participantIDs := make(map[uuid.UUID]struct{}, len(session.Participants))
	for _, p := range session.Participants {
		participantIDs[p.ProfileID] = struct{}{}
	}

	var ordered []uuid.UUID
	for _, m := range members {
		if _, ok := participantIDs[m.ProfileID]; ok {
			ordered = append(ordered, m.ProfileID)
		}
	}
	if len(ordered) == 0 {
		return nil, nil
	}

	groupSpec := crud.Specification[entity.Group]{}
	groupSpec.Model.ID = session.GroupID
	group, err := dss.groupRepo.FindFirst(ctx, groupSpec)
	if err != nil {
		return nil, err
	}

	chooser := ordered[group.RoundRobinPointer%len(ordered)]
	return &chooser, nil
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

// loadVotingSession is the shared guard for every mutating input endpoint:
// participant check (404, non-disclosure) → session load with the given
// preloads → status check (409 if not "voting"). Method-specific checks
// (right endpoint for this session's method, payload validity) happen in
// each caller after this returns.
func (dss *decisionSessionServiceImpl) loadVotingSession(ctx context.Context, sessionID, profileID uuid.UUID, preloads []string) (entity.DecisionSession, error) {
	participantSpec := crud.Specification[entity.SessionParticipant]{}
	participantSpec.Model.SessionID = sessionID
	participantSpec.Model.ProfileID = profileID
	participation, err := dss.sessionParticipantRepo.FindFirst(ctx, participantSpec)
	if err != nil {
		return entity.DecisionSession{}, err
	}
	if participation.IsZero() {
		return entity.DecisionSession{}, ungerr.NotFoundError(fmt.Sprintf("session %s is not found", sessionID))
	}

	spec := crud.Specification[entity.DecisionSession]{PreloadRelations: preloads}
	spec.Model.ID = sessionID
	session, err := dss.decisionSessionRepo.FindFirst(ctx, spec)
	if err != nil {
		return entity.DecisionSession{}, err
	}
	if session.ID == uuid.Nil {
		return entity.DecisionSession{}, ungerr.NotFoundError(fmt.Sprintf("session %s is not found", sessionID))
	}
	if session.Status != "voting" {
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

func (dss *decisionSessionServiceImpl) majorityTally(ctx context.Context, sessionID uuid.UUID) (dto.CountsTally, error) {
	spec := crud.Specification[entity.SessionVote]{}
	spec.Model.SessionID = sessionID
	votes, err := dss.sessionVoteRepo.FindAll(ctx, spec)
	if err != nil {
		return dto.CountsTally{}, err
	}

	counts := make(map[string]int)
	for _, v := range votes {
		counts[v.ContentID.String()]++
	}
	return dto.CountsTally{Counts: counts}, nil
}

// rankedTally is a raw first-preference snapshot, not an IRV simulation —
// see this task's "Design decisions" note on why round/eliminations are
// finalize-only (Task 4).
func (dss *decisionSessionServiceImpl) rankedTally(ctx context.Context, session entity.DecisionSession) (dto.RankedTally, error) {
	spec := crud.Specification[entity.SessionRanking]{}
	spec.Model.SessionID = session.ID
	rankings, err := dss.sessionRankingRepo.FindAll(ctx, spec)
	if err != nil {
		return dto.RankedTally{}, err
	}

	counts := make(map[string]int)
	for _, r := range rankings {
		if r.Rank == 1 {
			counts[r.ContentID.String()]++
		}
	}

	activeCandidateIDs := make([]uuid.UUID, len(session.Candidates))
	for i, c := range session.Candidates {
		activeCandidateIDs[i] = c.ContentID
	}

	return dto.RankedTally{
		Round:                  1,
		ActiveCandidateIDs:     activeCandidateIDs,
		EliminatedCandidateIDs: []uuid.UUID{},
		Counts:                 counts,
	}, nil
}

func (dss *decisionSessionServiceImpl) selectionTally(ctx context.Context, session entity.DecisionSession) (dto.SelectionTally, error) {
	chooser, err := dss.currentChooser(ctx, session)
	if err != nil {
		return dto.SelectionTally{}, err
	}
	if chooser == nil {
		return dto.SelectionTally{}, nil
	}

	spec := crud.Specification[entity.SessionVote]{}
	spec.Model.SessionID = session.ID
	spec.Model.ProfileID = *chooser
	vote, err := dss.sessionVoteRepo.FindFirst(ctx, spec)
	if err != nil {
		return dto.SelectionTally{}, err
	}
	if vote.IsZero() {
		return dto.SelectionTally{}, nil
	}
	return dto.SelectionTally{SelectedContentID: &vote.ContentID}, nil
}

func (dss *decisionSessionServiceImpl) CastVote(ctx context.Context, profileID, sessionID uuid.UUID, request dto.CastVoteRequest) (dto.TallyResponse, error) {
	ctx, span := otel.Tracer.Start(ctx, "DecisionSessionService.CastVote")
	defer span.End()

	session, err := dss.loadVotingSession(ctx, sessionID, profileID, []string{"Candidates"})
	if err != nil {
		return dto.TallyResponse{}, err
	}
	if session.Method != "majority" {
		return dto.TallyResponse{}, ungerr.BadRequestError(fmt.Sprintf("session %s is not a majority session", sessionID))
	}
	if !containsCandidate(session.Candidates, request.ContentID) {
		return dto.TallyResponse{}, ungerr.BadRequestError(fmt.Sprintf("content %s is not a session candidate", request.ContentID))
	}
	if err := dss.upsertVote(ctx, sessionID, profileID, request.ContentID); err != nil {
		return dto.TallyResponse{}, err
	}

	tally, err := dss.majorityTally(ctx, sessionID)
	if err != nil {
		return dto.TallyResponse{}, err
	}
	return dto.TallyResponse{Tally: tally}, nil
}

func (dss *decisionSessionServiceImpl) SubmitRanking(ctx context.Context, profileID, sessionID uuid.UUID, request dto.SubmitRankingRequest) (dto.TallyResponse, error) {
	ctx, span := otel.Tracer.Start(ctx, "DecisionSessionService.SubmitRanking")
	defer span.End()

	session, err := dss.loadVotingSession(ctx, sessionID, profileID, []string{"Candidates"})
	if err != nil {
		return dto.TallyResponse{}, err
	}
	if session.Method != "ranked" {
		return dto.TallyResponse{}, ungerr.BadRequestError(fmt.Sprintf("session %s is not a ranked session", sessionID))
	}

	seen := make(map[uuid.UUID]struct{}, len(request.Ranking))
	for _, cid := range request.Ranking {
		if _, dup := seen[cid]; dup {
			return dto.TallyResponse{}, ungerr.BadRequestError(fmt.Sprintf("ranking has duplicate content %s", cid))
		}
		seen[cid] = struct{}{}
		if !containsCandidate(session.Candidates, cid) {
			return dto.TallyResponse{}, ungerr.BadRequestError(fmt.Sprintf("content %s is not a session candidate", cid))
		}
	}

	existingSpec := crud.Specification[entity.SessionRanking]{}
	existingSpec.Model.SessionID = sessionID
	existingSpec.Model.ProfileID = profileID
	existing, err := dss.sessionRankingRepo.FindAll(ctx, existingSpec)
	if err != nil {
		return dto.TallyResponse{}, err
	}
	if len(existing) > 0 {
		if err := dss.sessionRankingRepo.DeleteMany(ctx, existing); err != nil {
			return dto.TallyResponse{}, err
		}
	}

	newRankings := make([]entity.SessionRanking, len(request.Ranking))
	for i, cid := range request.Ranking {
		newRankings[i] = entity.SessionRanking{SessionID: sessionID, ProfileID: profileID, ContentID: cid, Rank: i + 1}
	}
	if _, err := dss.sessionRankingRepo.InsertMany(ctx, newRankings); err != nil {
		return dto.TallyResponse{}, err
	}

	tally, err := dss.rankedTally(ctx, session)
	if err != nil {
		return dto.TallyResponse{}, err
	}
	return dto.TallyResponse{Tally: tally}, nil
}

func (dss *decisionSessionServiceImpl) Select(ctx context.Context, profileID, sessionID uuid.UUID, request dto.CastVoteRequest) (dto.TallyResponse, error) {
	ctx, span := otel.Tracer.Start(ctx, "DecisionSessionService.Select")
	defer span.End()

	session, err := dss.loadVotingSession(ctx, sessionID, profileID, []string{"Candidates", "Participants"})
	if err != nil {
		return dto.TallyResponse{}, err
	}
	if session.Method != "round_robin" {
		return dto.TallyResponse{}, ungerr.BadRequestError(fmt.Sprintf("session %s is not a round robin session", sessionID))
	}

	chooser, err := dss.currentChooser(ctx, session)
	if err != nil {
		return dto.TallyResponse{}, err
	}
	if chooser == nil || *chooser != profileID {
		return dto.TallyResponse{}, ungerr.ForbiddenError(fmt.Sprintf("profile %s is not the current chooser for session %s", profileID, sessionID))
	}
	if !containsCandidate(session.Candidates, request.ContentID) {
		return dto.TallyResponse{}, ungerr.BadRequestError(fmt.Sprintf("content %s is not a session candidate", request.ContentID))
	}
	if err := dss.upsertVote(ctx, sessionID, profileID, request.ContentID); err != nil {
		return dto.TallyResponse{}, err
	}

	tally, err := dss.selectionTally(ctx, session)
	if err != nil {
		return dto.TallyResponse{}, err
	}
	return dto.TallyResponse{Tally: tally}, nil
}
