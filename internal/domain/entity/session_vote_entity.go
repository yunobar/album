package entity

import (
	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
)

type SessionVote struct {
	crud.BaseEntity
	SessionID uuid.UUID
	ProfileID uuid.UUID
	ContentID uuid.UUID

	// Relationships
	Session DecisionSession
	Profile UserProfile
	Content Content
}

func (SessionVote) TableName() string {
	return "session_votes"
}
