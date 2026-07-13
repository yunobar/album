package entity

import "github.com/itsLeonB/go-crud"

type Group struct {
	crud.BaseEntity
	Name        *string
	InviteToken string
}

func (Group) TableName() string {
	return "groups"
}
