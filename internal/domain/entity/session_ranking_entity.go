package entity

import (
	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
)

type SessionRanking struct {
	crud.BaseEntity
	SessionID uuid.UUID
	ProfileID uuid.UUID
	ContentID uuid.UUID
	Rank      int

	// Relationships
	Session DecisionSession
	Profile UserProfile
	Content Content
}

func (SessionRanking) TableName() string {
	return "session_rankings"
}
