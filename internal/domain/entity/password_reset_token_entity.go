package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
)

type PasswordResetToken struct {
	crud.BaseEntity
	UserID       uuid.UUID
	Selector     string
	VerifierHash string
	ExpiresAt    time.Time
}

func (prt PasswordResetToken) IsValid() bool {
	return !prt.IsZero() && prt.ExpiresAt.After(time.Now())
}
