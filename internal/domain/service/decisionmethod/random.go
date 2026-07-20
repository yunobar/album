package decisionmethod

import (
	"context"

	"github.com/google/uuid"
	"github.com/yunobar/album/internal/domain/entity"
)

type randomStrategy struct {
	base
}

func (s *randomStrategy) Resolve(ctx context.Context, session entity.DecisionSession) (uuid.UUID, error) {
	return seededPick(session.RandomSeed, sessionCandidateIDs(session)), nil
}
