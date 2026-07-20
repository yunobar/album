package decisionmethod

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/yunobar/album/internal/domain/entity"
)

func TestResolveRandom(t *testing.T) {
	t.Run("same seed produces the same winner across repeated calls", func(t *testing.T) {
		session := entity.DecisionSession{
			Candidates: []entity.SessionCandidate{{ContentID: uuid.New()}, {ContentID: uuid.New()}, {ContentID: uuid.New()}},
			RandomSeed: 123,
		}
		strat := &randomStrategy{}

		first, err := strat.Resolve(context.Background(), session)
		assert.NoError(t, err)
		second, err := strat.Resolve(context.Background(), session)
		assert.NoError(t, err)

		assert.Equal(t, first, second)
	})

	t.Run("distinct seeds don't all collapse onto the same candidate", func(t *testing.T) {
		session := entity.DecisionSession{
			Candidates: []entity.SessionCandidate{
				{ContentID: uuid.New()}, {ContentID: uuid.New()}, {ContentID: uuid.New()}, {ContentID: uuid.New()},
			},
		}
		strat := &randomStrategy{}

		results := make(map[uuid.UUID]bool)
		for seed := range int64(50) {
			session.RandomSeed = seed
			winner, err := strat.Resolve(context.Background(), session)
			assert.NoError(t, err)
			results[winner] = true
		}

		assert.Greater(t, len(results), 1, "50 distinct seeds should not always pick the same candidate")
	})
}
