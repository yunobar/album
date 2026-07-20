package decisionmethod

import (
	"context"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/entity"
)

type majorityStrategy struct {
	base
	sessionVoteRepo crud.Repository[entity.SessionVote]
}

func (s *majorityStrategy) Tally(ctx context.Context, session entity.DecisionSession) (any, error) {
	spec := crud.Specification[entity.SessionVote]{}
	spec.Model.SessionID = session.ID
	votes, err := s.sessionVoteRepo.FindAll(ctx, spec)
	if err != nil {
		return dto.CountsTally{}, err
	}

	counts := make(map[string]int)
	for _, v := range votes {
		counts[v.ContentID.String()]++
	}
	return dto.CountsTally{Counts: counts}, nil
}

// Resolve: most votes wins. The spec's first tie-break
// ("fewest-recently-watched among tied titles") is a no-op under the
// current data model — see this task's brief for why — so ties go
// straight to seededPick.
// ponytail: no watch-history tie-break yet (Watch Ledger doesn't exist);
// wire in a real "times watched by this group" query once watch_events
// lands, ahead of the seededPick fallback below.
func (s *majorityStrategy) Resolve(ctx context.Context, session entity.DecisionSession) (uuid.UUID, error) {
	voteSpec := crud.Specification[entity.SessionVote]{}
	voteSpec.Model.SessionID = session.ID
	votes, err := s.sessionVoteRepo.FindAll(ctx, voteSpec)
	if err != nil {
		return uuid.Nil, err
	}

	candidateIDs := sessionCandidateIDs(session)
	counts := make(map[uuid.UUID]int, len(candidateIDs))
	for _, cid := range candidateIDs {
		counts[cid] = 0
	}
	for _, v := range votes {
		counts[v.ContentID]++
	}
	return seededPick(session.RandomSeed, tiedFor(candidateIDs, counts)), nil
}
