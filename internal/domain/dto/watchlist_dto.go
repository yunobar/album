package dto

import (
	"github.com/google/uuid"
)

type AddWatchlistItemRequest struct {
	ContentID uuid.UUID `json:"contentId" binding:"required"`
	Priority  string    `json:"priority" binding:"required,oneof=must high medium low"`
	Notes     string    `json:"notes"`
}

type UpdateWatchlistItemRequest struct {
	Priority string  `json:"priority" binding:"omitempty,oneof=must high medium low"`
	Notes    *string `json:"notes"`
}

type WatchlistItemResponse struct {
	BaseDTO
	Priority string          `json:"priority"`
	Notes    string          `json:"notes"`
	Content  ContentResponse `json:"content"`
}
