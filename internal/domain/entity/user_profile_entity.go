package entity

import (
	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
)

type UserProfile struct {
	crud.BaseEntity
	UserID uuid.NullUUID
	Name   string
	Avatar string
}

func (up UserProfile) IsReal() bool {
	return up.UserID.Valid
}
