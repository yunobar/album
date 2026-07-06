package dto

import (
	"time"

	"github.com/google/uuid"
)

type BaseDTO struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
