package service

import (
	"math/rand"
	"sort"

	"github.com/google/uuid"
	"github.com/yunobar/album/internal/appconstant"
	"github.com/yunobar/album/internal/domain/entity"
)

// seededPick deterministically picks one candidate from candidateIDs using
// seed. Candidates are sorted by string ID first — Go map iteration order
// is randomized, and every caller here builds candidateIDs by ranging over
// a map, so without this sort the same seed could produce different
// results across runs, defeating the whole point of storing a reproducible
// seed.
func seededPick(seed int64, candidateIDs []uuid.UUID) uuid.UUID {
	sorted := append([]uuid.UUID(nil), candidateIDs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].String() < sorted[j].String() })
	r := rand.New(rand.NewSource(seed))
	return sorted[r.Intn(len(sorted))]
}

// tiedFor returns the subset of ids with the highest score.
func tiedFor(ids []uuid.UUID, scores map[uuid.UUID]int) []uuid.UUID {
	best := scores[ids[0]]
	for _, id := range ids {
		if scores[id] > best {
			best = scores[id]
		}
	}
	var tied []uuid.UUID
	for _, id := range ids {
		if scores[id] == best {
			tied = append(tied, id)
		}
	}
	return tied
}

// tiedForLowest is tiedFor's mirror, used by IRV's loser selection.
func tiedForLowest(ids []uuid.UUID, scores map[uuid.UUID]int) []uuid.UUID {
	lowest := scores[ids[0]]
	for _, id := range ids {
		if scores[id] < lowest {
			lowest = scores[id]
		}
	}
	var tied []uuid.UUID
	for _, id := range ids {
		if scores[id] == lowest {
			tied = append(tied, id)
		}
	}
	return tied
}

// resolveMajority: most votes wins. The spec's first tie-break
// ("fewest-recently-watched among tied titles") is a no-op under the
// current data model — see this task's brief for why — so ties go
// straight to seededPick.
// ponytail: no watch-history tie-break yet (Watch Ledger doesn't exist);
// wire in a real "times watched by this group" query once watch_events
// lands, ahead of the seededPick fallback below.
func resolveMajority(candidateIDs []uuid.UUID, votes []entity.SessionVote, seed int64) uuid.UUID {
	counts := make(map[uuid.UUID]int, len(candidateIDs))
	for _, cid := range candidateIDs {
		counts[cid] = 0
	}
	for _, v := range votes {
		counts[v.ContentID]++
	}
	return seededPick(seed, tiedFor(candidateIDs, counts))
}

// resolvePriority: highest weighted total wins; tie → highest single
// Must-Watch count; still tied → seeded random.
func resolvePriority(candidateIDs []uuid.UUID, snapshots []entity.SessionPrioritySnapshot, seed int64) uuid.UUID {
	totals := make(map[uuid.UUID]int, len(candidateIDs))
	mustCounts := make(map[uuid.UUID]int, len(candidateIDs))
	for _, cid := range candidateIDs {
		totals[cid] = 0
		mustCounts[cid] = 0
	}
	for _, s := range snapshots {
		totals[s.ContentID] += appconstant.PriorityWeights[s.Priority]
		if s.Priority == appconstant.PriorityMust {
			mustCounts[s.ContentID]++
		}
	}

	tied := tiedFor(candidateIDs, totals)
	if len(tied) == 1 {
		return tied[0]
	}
	return seededPick(seed, tiedFor(tied, mustCounts))
}

// resolveRoundRobin reads back the chooser's stored pick (a session_votes
// row, per this module's design — see Task 3). A nil chooserVote means the
// chooser never picked; still must be computable, so it falls back to
// seededPick rather than erroring.
func resolveRoundRobin(candidateIDs []uuid.UUID, chooserVote *entity.SessionVote, seed int64) uuid.UUID {
	if chooserVote != nil {
		return chooserVote.ContentID
	}
	return seededPick(seed, candidateIDs)
}

func resolveRandom(candidateIDs []uuid.UUID, seed int64) uuid.UUID {
	return seededPick(seed, candidateIDs)
}

func resolveRanked(candidateIDs []uuid.UUID, rankings []entity.SessionRanking, seed int64) uuid.UUID {
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
			return seededPick(seed, activeIDs)
		}

		for _, cid := range activeIDs {
			if float64(counts[cid]) > float64(countedBallots)/2 {
				return cid
			}
		}

		if len(active) == 1 {
			return activeIDs[0]
		}

		loser := irvLoser(activeIDs, counts, ballots, active, seed)
		delete(active, loser)
	}
}

// irvLoser eliminates the fewest-first-preference candidate. A tie is
// broken by counting each tied candidate's "transferable" ballots — those
// currently counting them first that rank another still-active candidate
// afterward — and eliminating whichever would transfer fewest votes
// forward (weakest remaining support). A further tie falls to seededPick.
func irvLoser(activeIDs []uuid.UUID, counts map[uuid.UUID]int, ballots [][]uuid.UUID, active map[uuid.UUID]bool, seed int64) uuid.UUID {
	tied := tiedForLowest(activeIDs, counts)
	if len(tied) == 1 {
		return tied[0]
	}

	tiedSet := make(map[uuid.UUID]bool, len(tied))
	for _, cid := range tied {
		tiedSet[cid] = true
	}

	transferable := make(map[uuid.UUID]int, len(tied))
	for _, cid := range tied {
		transferable[cid] = 0
	}
	for _, ballot := range ballots {
		firstActiveIdx := -1
		for i, cid := range ballot {
			if active[cid] {
				firstActiveIdx = i
				break
			}
		}
		if firstActiveIdx == -1 || !tiedSet[ballot[firstActiveIdx]] {
			continue
		}
		for _, cid := range ballot[firstActiveIdx+1:] {
			if active[cid] {
				transferable[ballot[firstActiveIdx]]++
				break
			}
		}
	}

	return seededPick(seed, tiedForLowest(tied, transferable))
}
