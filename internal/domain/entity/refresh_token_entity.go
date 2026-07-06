package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
)

type RefreshToken struct {
	crud.BaseEntity
	SessionID uuid.UUID
	TokenHash string
	ExpiresAt time.Time
}
