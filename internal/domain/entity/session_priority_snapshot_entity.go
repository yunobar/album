package entity

import (
	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
)

type SessionPrioritySnapshot struct {
	crud.BaseEntity
	SessionID uuid.UUID
	ProfileID uuid.UUID
	ContentID uuid.UUID
	Priority  string

	// Relationships
	Session DecisionSession
	Profile UserProfile
	Content Content
}

func (SessionPrioritySnapshot) TableName() string {
	return "session_priority_snapshots"
}
