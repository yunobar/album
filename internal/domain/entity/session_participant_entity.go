package entity

import (
	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
)

type SessionParticipant struct {
	crud.BaseEntity
	SessionID uuid.UUID
	ProfileID uuid.UUID

	// Relationships
	Session DecisionSession
	Profile UserProfile
}

func (SessionParticipant) TableName() string {
	return "session_participants"
}
