package entity

import (
	"encoding/json"

	"github.com/itsLeonB/go-crud"
)

type Content struct {
	crud.BaseEntity
	Source      string
	SourceID    string
	ContentType string
	Title       string
	ReleaseYear *int
	PosterURL   string
	Metadata    json.RawMessage `gorm:"type:jsonb"`
}

func (Content) TableName() string {
	return "content"
}
