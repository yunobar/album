package decisionmethod

import (
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestSeededPick(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	seed := int64(42)

	first := seededPick(seed, ids)
	second := seededPick(seed, ids)

	assert.Equal(t, first, second, "same seed + same tied set must be deterministic")
	assert.True(t, slices.Contains(ids, first))
}
