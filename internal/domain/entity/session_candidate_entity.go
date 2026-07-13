package entity

import (
	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
)

type SessionCandidate struct {
	crud.BaseEntity
	SessionID uuid.UUID
	ContentID uuid.UUID

	// Relationships
	Session DecisionSession
	Content Content
}

func (SessionCandidate) TableName() string {
	return "session_candidates"
}
