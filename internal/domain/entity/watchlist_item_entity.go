package entity

import (
	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
)

type WatchlistItem struct {
	crud.BaseEntity
	ProfileID uuid.UUID
	ContentID uuid.UUID
	Priority  string
	Notes     string
	Status    string

	// Relationships
	Content Content
}

func (WatchlistItem) TableName() string {
	return "watchlist_items"
}
