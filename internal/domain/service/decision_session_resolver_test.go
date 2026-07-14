package service

import (
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/yunobar/album/internal/appconstant"
	"github.com/yunobar/album/internal/domain/entity"
)

func TestSeededPick(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	seed := int64(42)

	first := seededPick(seed, ids)
	second := seededPick(seed, ids)

	assert.Equal(t, first, second, "same seed + same tied set must be deterministic")
	assert.True(t, slices.Contains(ids, first))
}

func TestResolveMajority(t *testing.T) {
	t.Run("clear winner by vote count", func(t *testing.T) {
		a, b := uuid.New(), uuid.New()
		votes := []entity.SessionVote{
			{ContentID: a}, {ContentID: a}, {ContentID: b},
		}

		winner := resolveMajority([]uuid.UUID{a, b}, votes, 1)

		assert.Equal(t, a, winner)
	})

	t.Run("tie falls to a deterministic seeded pick", func(t *testing.T) {
		a, b := uuid.New(), uuid.New()
		votes := []entity.SessionVote{{ContentID: a}, {ContentID: b}}

		first := resolveMajority([]uuid.UUID{a, b}, votes, 7)
		second := resolveMajority([]uuid.UUID{a, b}, votes, 7)

		assert.Equal(t, first, second)
		assert.True(t, first == a || first == b)
	})

	t.Run("zero votes still returns a candidate", func(t *testing.T) {
		a, b := uuid.New(), uuid.New()

		winner := resolveMajority([]uuid.UUID{a, b}, nil, 3)

		assert.True(t, winner == a || winner == b)
	})
}

func TestResolvePriority(t *testing.T) {
	t.Run("clear winner by weighted total", func(t *testing.T) {
		a, b := uuid.New(), uuid.New()
		snapshots := []entity.SessionPrioritySnapshot{
			{ContentID: a, Priority: appconstant.PriorityMust},
			{ContentID: b, Priority: appconstant.PriorityLow},
		}

		winner := resolvePriority([]uuid.UUID{a, b}, snapshots, 1)

		assert.Equal(t, a, winner)
	})

	t.Run("tie on total broken by must-watch count", func(t *testing.T) {
		a, b := uuid.New(), uuid.New()
		// a: high(3) + medium(2) = 5, no must votes.
		// b: must(5) alone = 5, one must vote.
		snapshots := []entity.SessionPrioritySnapshot{
			{ContentID: a, Priority: appconstant.PriorityHigh},
			{ContentID: a, Priority: appconstant.PriorityMedium},
			{ContentID: b, Priority: appconstant.PriorityMust},
		}

		winner := resolvePriority([]uuid.UUID{a, b}, snapshots, 1)

		assert.Equal(t, b, winner, "b has 1 must-watch vote vs a's 0, despite equal totals")
	})

	t.Run("tie on total and must count falls to deterministic seeded pick", func(t *testing.T) {
		a, b := uuid.New(), uuid.New()
		snapshots := []entity.SessionPrioritySnapshot{
			{ContentID: a, Priority: appconstant.PriorityMust},
			{ContentID: b, Priority: appconstant.PriorityMust},
		}

		first := resolvePriority([]uuid.UUID{a, b}, snapshots, 9)
		second := resolvePriority([]uuid.UUID{a, b}, snapshots, 9)

		assert.Equal(t, first, second)
		assert.True(t, first == a || first == b)
	})

	t.Run("zero snapshots, all weight 0, still returns a candidate", func(t *testing.T) {
		a, b := uuid.New(), uuid.New()

		winner := resolvePriority([]uuid.UUID{a, b}, nil, 4)

		assert.True(t, winner == a || winner == b)
	})
}

func TestResolveRoundRobin(t *testing.T) {
	t.Run("chooser's vote wins", func(t *testing.T) {
		a, b := uuid.New(), uuid.New()
		chooserVote := &entity.SessionVote{ContentID: b}

		winner := resolveRoundRobin([]uuid.UUID{a, b}, chooserVote, 1)

		assert.Equal(t, b, winner)
	})

	t.Run("nil chooser vote falls to a deterministic seeded pick", func(t *testing.T) {
		a, b := uuid.New(), uuid.New()

		first := resolveRoundRobin([]uuid.UUID{a, b}, nil, 5)
		second := resolveRoundRobin([]uuid.UUID{a, b}, nil, 5)

		assert.Equal(t, first, second)
		assert.True(t, first == a || first == b)
	})
}

func TestResolveRandom(t *testing.T) {
	t.Run("same seed produces the same winner across repeated calls", func(t *testing.T) {
		ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

		first := resolveRandom(ids, 123)
		second := resolveRandom(ids, 123)

		assert.Equal(t, first, second)
	})

	t.Run("distinct seeds don't all collapse onto the same candidate", func(t *testing.T) {
		ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()}

		results := make(map[uuid.UUID]bool)
		for seed := int64(0); seed < 50; seed++ {
			results[resolveRandom(ids, seed)] = true
		}

		assert.Greater(t, len(results), 1, "50 distinct seeds should not always pick the same candidate")
	})
}

func TestResolveRanked(t *testing.T) {
	t.Run("outright first-round majority wins immediately", func(t *testing.T) {
		a, b := uuid.New(), uuid.New()
		rankings := rankingsFor(map[uuid.UUID][]uuid.UUID{
			uuid.New(): {a, b},
			uuid.New(): {a, b},
			uuid.New(): {b, a},
		})

		winner := resolveRanked([]uuid.UUID{a, b}, rankings, 1)

		assert.Equal(t, a, winner)
	})

	t.Run("multi-round elimination: lowest first-preference candidate is eliminated and its votes transfer", func(t *testing.T) {
		a, b, c := uuid.New(), uuid.New(), uuid.New()
		// Round 1 first choices: A=2 (ballots 1,2), B=1 (ballot 3), C=2
		// (ballots 4,5). Total=5, threshold=2.5 — nobody exceeds it, so B
		// (the unique lowest) is eliminated.
		// Round 2 (active A,C): ballot 3's B-vote transfers to A (its next
		// active preference) — A=3, C=2, threshold=2.5 — A wins.
		rankings := rankingsFor(map[uuid.UUID][]uuid.UUID{
			uuid.New(): {a, b, c},
			uuid.New(): {a, c, b},
			uuid.New(): {b, a, c},
			uuid.New(): {c, b, a},
			uuid.New(): {c, a, b},
		})

		winner := resolveRanked([]uuid.UUID{a, b, c}, rankings, 1)

		assert.Equal(t, a, winner, "B's elimination transfers ballot 3's vote to A, tipping A over 50%")
	})

	t.Run("exhausted ballots drop out of the denominator and change the majority threshold", func(t *testing.T) {
		a, b, c := uuid.New(), uuid.New(), uuid.New()
		// Round 1: A=3, B=2, C=1 (6 ballots total, threshold=3) — A's 3
		// does NOT exceed 3, so no winner yet. C (unique lowest) is
		// eliminated.
		// Round 2 (active A,B): the C-only ballot has no active
		// preference left and drops out entirely — countedBallots=5, not
		// 6. threshold=2.5. A=3 > 2.5 — A wins.
		// A buggy implementation that kept dividing by the original 6
		// would compute threshold=3 and never find A a winner (3 is not
		// > 3), diverging from this expected result.
		rankings := rankingsFor(map[uuid.UUID][]uuid.UUID{
			uuid.New(): {c},
			uuid.New(): {a, b},
			uuid.New(): {a, b},
			uuid.New(): {a, b},
			uuid.New(): {b, a},
			uuid.New(): {b, a},
		})

		winner := resolveRanked([]uuid.UUID{a, b, c}, rankings, 1)

		assert.Equal(t, a, winner)
	})

	t.Run("zero rankings still returns a candidate", func(t *testing.T) {
		a, b, c := uuid.New(), uuid.New(), uuid.New()

		winner := resolveRanked([]uuid.UUID{a, b, c}, nil, 2)

		assert.True(t, slices.Contains([]uuid.UUID{a, b, c}, winner))
	})
}

// rankingsFor builds entity.SessionRanking rows from a per-profile ordered
// preference list — index 0 is rank 1, etc. Each map key is used only as a
// distinct profileID; the key's own value is irrelevant.
func rankingsFor(byProfile map[uuid.UUID][]uuid.UUID) []entity.SessionRanking {
	var out []entity.SessionRanking
	for profileID, ordered := range byProfile {
		for i, cid := range ordered {
			out = append(out, entity.SessionRanking{ProfileID: profileID, ContentID: cid, Rank: i + 1})
		}
	}
	return out
}
