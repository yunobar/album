package entity

import (
	"time"

	"github.com/google/uuid"
)

type DecisionSession struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey;default:uuidv7()"`
	GroupID         uuid.UUID
	Method          string
	Status          string
	WinnerContentID *uuid.UUID
	RandomSeed      int64
	CreatedAt       time.Time
	FinalizedAt     *time.Time

	// Relationships
	Group             Group
	Participants      []SessionParticipant      `gorm:"foreignKey:SessionID"`
	Candidates        []SessionCandidate        `gorm:"foreignKey:SessionID"`
	Votes             []SessionVote             `gorm:"foreignKey:SessionID"`
	Rankings          []SessionRanking          `gorm:"foreignKey:SessionID"`
	PrioritySnapshots []SessionPrioritySnapshot `gorm:"foreignKey:SessionID"`
}

func (DecisionSession) TableName() string {
	return "decision_sessions"
}
