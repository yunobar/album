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
	if session.Method == "round_robin" {
		chooser, err = dss.currentChooser(ctx, session)
		if err != nil {
			return dto.SessionResponse{}, err
		}
	}

	return mapper.SessionToResponse(session, chooser), nil
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
