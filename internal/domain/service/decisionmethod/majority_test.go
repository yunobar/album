package decisionmethod

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/yunobar/album/internal/domain/entity"
	"github.com/yunobar/album/internal/mocks"
)

func TestResolveMajority(t *testing.T) {
	newStrategy := func(t *testing.T, votes []entity.SessionVote) *majorityStrategy {
		t.Helper()
		repo := mocks.NewMockRepository[entity.SessionVote](t)
		repo.EXPECT().FindAll(mock.Anything, mock.Anything).Return(votes, nil)
		return &majorityStrategy{sessionVoteRepo: repo}
	}

	t.Run("clear winner by vote count", func(t *testing.T) {
		a, b := uuid.New(), uuid.New()
		votes := []entity.SessionVote{
			{ContentID: a}, {ContentID: a}, {ContentID: b},
		}
		session := entity.DecisionSession{
			Candidates: []entity.SessionCandidate{{ContentID: a}, {ContentID: b}},
			RandomSeed: 1,
		}
		strat := newStrategy(t, votes)

		winner, err := strat.Resolve(context.Background(), session)

		assert.NoError(t, err)
		assert.Equal(t, a, winner)
	})

	t.Run("tie falls to a deterministic seeded pick", func(t *testing.T) {
		a, b := uuid.New(), uuid.New()
		votes := []entity.SessionVote{{ContentID: a}, {ContentID: b}}
		session := entity.DecisionSession{
			Candidates: []entity.SessionCandidate{{ContentID: a}, {ContentID: b}},
			RandomSeed: 7,
		}

		first, err := newStrategy(t, votes).Resolve(context.Background(), session)
		assert.NoError(t, err)
		second, err := newStrategy(t, votes).Resolve(context.Background(), session)
		assert.NoError(t, err)

		assert.Equal(t, first, second)
		assert.True(t, first == a || first == b)
	})

	t.Run("zero votes still returns a candidate", func(t *testing.T) {
		a, b := uuid.New(), uuid.New()
		session := entity.DecisionSession{
			Candidates: []entity.SessionCandidate{{ContentID: a}, {ContentID: b}},
			RandomSeed: 3,
		}
		strat := newStrategy(t, nil)

		winner, err := strat.Resolve(context.Background(), session)

		assert.NoError(t, err)
		assert.True(t, winner == a || winner == b)
	})
}
