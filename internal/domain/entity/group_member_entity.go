package entity

import (
	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
)

type GroupMember struct {
	crud.BaseEntity
	GroupID   uuid.UUID
	ProfileID uuid.UUID

	// Relationships
	Group   Group
	Profile UserProfile
}

func (GroupMember) TableName() string {
	return "group_members"
}
