package entity

import (
	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
)

type OAuthAccount struct {
	crud.BaseEntity
	UserID     uuid.UUID
	Provider   string
	ProviderID string
	Email      string

	// Relationships
	User User `gorm:"foreignKey:UserID"`
}

func (OAuthAccount) TableName() string {
	return "oauth_accounts"
}
