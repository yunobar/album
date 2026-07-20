package decisionmethod

import (
	"context"
	"sort"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/entity"
)

type rankedStrategy struct {
	base
	sessionRankingRepo crud.Repository[entity.SessionRanking]
}

// Tally is a raw first-preference snapshot, not an IRV simulation — see
// this task's "Design decisions" note on why round/eliminations are
// finalize-only (Task 4).
func (s *rankedStrategy) Tally(ctx context.Context, session entity.DecisionSession) (any, error) {
	spec := crud.Specification[entity.SessionRanking]{}
	spec.Model.SessionID = session.ID
	rankings, err := s.sessionRankingRepo.FindAll(ctx, spec)
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

func (s *rankedStrategy) Resolve(ctx context.Context, session entity.DecisionSession) (uuid.UUID, error) {
	rankingSpec := crud.Specification[entity.SessionRanking]{}
	rankingSpec.Model.SessionID = session.ID
	rankings, err := s.sessionRankingRepo.FindAll(ctx, rankingSpec)
	if err != nil {
		return uuid.Nil, err
	}

	candidateIDs := sessionCandidateIDs(session)
	seed := session.RandomSeed

	ballotsByProfile := make(map[uuid.UUID][]entity.SessionRanking)
	for _, r := range rankings {
		ballotsByProfile[r.ProfileID] = append(ballotsByProfile[r.ProfileID], r)
	}

	ballots := make([][]uuid.UUID, 0, len(ballotsByProfile))
	for _, rs := range ballotsByProfile {
		sort.Slice(rs, func(i, j int) bool { return rs[i].Rank < rs[j].Rank })
		ordered := make([]uuid.UUID, len(rs))
		for i, r := range rs {
			ordered[i] = r.ContentID
		}
		ballots = append(ballots, ordered)
	}

	active := make(map[uuid.UUID]bool, len(candidateIDs))
	for _, cid := range candidateIDs {
		active[cid] = true
	}

	for {
		counts := make(map[uuid.UUID]int)
		for cid := range active {
			counts[cid] = 0
		}
		countedBallots := 0
		for _, ballot := range ballots {
			for _, cid := range ballot {
				if active[cid] {
					counts[cid]++
					countedBallots++
					break
				}
			}
		}

		activeIDs := make([]uuid.UUID, 0, len(active))
		for cid := range active {
			activeIDs = append(activeIDs, cid)
		}

		// Always computable — zero counted ballots (no ranked input at all,
		// or every ballot exhausted against the current active set) skips
		// straight to seeded random rather than dividing by zero below.
		if countedBallots == 0 {
			return seededPick(seed, activeIDs), nil
		}

		for _, cid := range activeIDs {
			if float64(counts[cid]) > float64(countedBallots)/2 {
				return cid, nil
			}
		}

		if len(active) == 1 {
			return activeIDs[0], nil
		}

		loser := irvLoser(activeIDs, counts, ballots, active, seed)
		delete(active, loser)
	}
}
