package decisionmethod

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/yunobar/album/internal/domain/entity"
	"github.com/yunobar/album/internal/mocks"
)

func TestResolveRanked(t *testing.T) {
	newStrategy := func(t *testing.T, rankings []entity.SessionRanking) *rankedStrategy {
		t.Helper()
		repo := mocks.NewMockRepository[entity.SessionRanking](t)
		repo.EXPECT().FindAll(mock.Anything, mock.Anything).Return(rankings, nil)
		return &rankedStrategy{sessionRankingRepo: repo}
	}

	t.Run("outright first-round majority wins immediately", func(t *testing.T) {
		a, b := uuid.New(), uuid.New()
		rankings := rankingsFor(map[uuid.UUID][]uuid.UUID{
			uuid.New(): {a, b},
			uuid.New(): {a, b},
			uuid.New(): {b, a},
		})
		session := entity.DecisionSession{
			Candidates: []entity.SessionCandidate{{ContentID: a}, {ContentID: b}},
			RandomSeed: 1,
		}

		winner, err := newStrategy(t, rankings).Resolve(context.Background(), session)

		assert.NoError(t, err)
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
		session := entity.DecisionSession{
			Candidates: []entity.SessionCandidate{{ContentID: a}, {ContentID: b}, {ContentID: c}},
			RandomSeed: 1,
		}

		winner, err := newStrategy(t, rankings).Resolve(context.Background(), session)

		assert.NoError(t, err)
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
		session := entity.DecisionSession{
			Candidates: []entity.SessionCandidate{{ContentID: a}, {ContentID: b}, {ContentID: c}},
			RandomSeed: 1,
		}

		winner, err := newStrategy(t, rankings).Resolve(context.Background(), session)

		assert.NoError(t, err)
		assert.Equal(t, a, winner)
	})

	t.Run("zero rankings still returns a candidate", func(t *testing.T) {
		a, b, c := uuid.New(), uuid.New(), uuid.New()
		session := entity.DecisionSession{
			Candidates: []entity.SessionCandidate{{ContentID: a}, {ContentID: b}, {ContentID: c}},
			RandomSeed: 2,
		}

		winner, err := newStrategy(t, nil).Resolve(context.Background(), session)

		assert.NoError(t, err)
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
