package decisionmethod

import (
	"context"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/entity"
)

type roundRobinStrategy struct {
	base
	groupRepo       crud.Repository[entity.Group]
	groupMemberRepo crud.Repository[entity.GroupMember]
	sessionVoteRepo crud.Repository[entity.SessionVote]
}

// Chooser duplicates group_service.go's membersOf join-order idiom rather
// than exporting it from GroupService — deliberate, matching this
// codebase's established pattern of each service depending only on repos,
// never on other services.
func (s *roundRobinStrategy) Chooser(ctx context.Context, session entity.DecisionSession) (*uuid.UUID, error) {
	memberSpec := crud.Specification[entity.GroupMember]{}
	memberSpec.Model.GroupID = session.GroupID
	members, err := s.groupMemberRepo.FindAll(ctx, memberSpec)
	if err != nil {
		return nil, err
	}

	groupSpec := crud.Specification[entity.Group]{}
	groupSpec.Model.ID = session.GroupID
	group, err := s.groupRepo.FindFirst(ctx, groupSpec)
	if err != nil {
		return nil, err
	}

	return chooserFromGroup(group, members, session.Participants), nil
}

func (s *roundRobinStrategy) Tally(ctx context.Context, session entity.DecisionSession) (any, error) {
	chooser, err := s.Chooser(ctx, session)
	if err != nil {
		return dto.SelectionTally{}, err
	}
	if chooser == nil {
		return dto.SelectionTally{}, nil
	}

	spec := crud.Specification[entity.SessionVote]{}
	spec.Model.SessionID = session.ID
	spec.Model.ProfileID = *chooser
	vote, err := s.sessionVoteRepo.FindFirst(ctx, spec)
	if err != nil {
		return dto.SelectionTally{}, err
	}
	if vote.IsZero() {
		return dto.SelectionTally{}, nil
	}
	return dto.SelectionTally{SelectedContentID: &vote.ContentID}, nil
}

// Resolve reads back the chooser's stored pick and advances the group's
// round_robin_pointer. Must run inside Finalize's transaction with the
// session already locked — the ForUpdate group fetch here is what
// serializes this read-increment-write against any other concurrent
// Finalize touching the same group's pointer (lost-update race), and
// against a sibling round_robin session in the same group computing a
// stale chooser from an unlocked read.
func (s *roundRobinStrategy) Resolve(ctx context.Context, session entity.DecisionSession) (uuid.UUID, error) {
	memberSpec := crud.Specification[entity.GroupMember]{}
	memberSpec.Model.GroupID = session.GroupID
	members, err := s.groupMemberRepo.FindAll(ctx, memberSpec)
	if err != nil {
		return uuid.Nil, err
	}

	groupSpec := crud.Specification[entity.Group]{ForUpdate: true}
	groupSpec.Model.ID = session.GroupID
	lockedGroup, err := s.groupRepo.FindFirst(ctx, groupSpec)
	if err != nil {
		return uuid.Nil, err
	}

	chooser := chooserFromGroup(lockedGroup, members, session.Participants)
	var chooserVote *entity.SessionVote
	if chooser != nil {
		voteSpec := crud.Specification[entity.SessionVote]{}
		voteSpec.Model.SessionID = session.ID
		voteSpec.Model.ProfileID = *chooser
		v, err := s.sessionVoteRepo.FindFirst(ctx, voteSpec)
		if err != nil {
			return uuid.Nil, err
		}
		if !v.IsZero() {
			chooserVote = &v
		}
	}

	// The chooser's stored pick wins (a session_votes row, per this
	// module's design — see Task 3). A nil chooserVote means the chooser
	// never picked; still must be computable, so it falls back to
	// seededPick rather than erroring.
	var winnerID uuid.UUID
	if chooserVote != nil {
		winnerID = chooserVote.ContentID
	} else {
		winnerID = seededPick(session.RandomSeed, sessionCandidateIDs(session))
	}

	lockedGroup.RoundRobinPointer++
	if _, err := s.groupRepo.Update(ctx, lockedGroup); err != nil {
		return uuid.Nil, err
	}

	return winnerID, nil
}
