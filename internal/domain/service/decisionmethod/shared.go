package decisionmethod

import (
	"math/rand"
	"sort"

	"github.com/google/uuid"
	"github.com/yunobar/album/internal/domain/entity"
)

// sessionCandidateIDs extracts the session's candidate content IDs — every
// strategy's Resolve needs this same slice built from session.Candidates.
func sessionCandidateIDs(session entity.DecisionSession) []uuid.UUID {
	ids := make([]uuid.UUID, len(session.Candidates))
	for i, c := range session.Candidates {
		ids[i] = c.ContentID
	}
	return ids
}

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

// chooserFromGroup picks the current chooser from an already-loaded
// group's RoundRobinPointer. Pure — callers are responsible for fetching
// group and members with whatever locking their context requires.
func chooserFromGroup(group entity.Group, members []entity.GroupMember, participants []entity.SessionParticipant) *uuid.UUID {
	sort.Slice(members, func(i, j int) bool {
		return members[i].ID.String() < members[j].ID.String()
	})

	participantIDs := make(map[uuid.UUID]struct{}, len(participants))
	for _, p := range participants {
		participantIDs[p.ProfileID] = struct{}{}
	}

	var ordered []uuid.UUID
	for _, m := range members {
		if _, ok := participantIDs[m.ProfileID]; ok {
			ordered = append(ordered, m.ProfileID)
		}
	}
	if len(ordered) == 0 {
		return nil
	}

	chooser := ordered[group.RoundRobinPointer%len(ordered)]
	return &chooser
}
