package entity

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
)

type Session struct {
	crud.BaseEntity
	UserID     uuid.UUID
	DeviceID   sql.NullString
	LastUsedAt time.Time
}
