package dto

import (
	"github.com/google/uuid"
)

type ProfileResponse struct {
	BaseDTO
	UserID uuid.UUID `json:"userId"`
	Name   string    `json:"name"`
	Avatar string    `json:"avatar"`
	Email  string    `json:"email"`
}

type UpdateProfileRequest struct {
	ID   uuid.UUID `json:"-"`
	Name string    `json:"name" binding:"required,min=3,max=255"`
}

type SimpleProfile struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Avatar string    `json:"avatar"`
	IsUser bool      `json:"isUser"`
}

type NewProfileRequest struct {
	UserID uuid.UUID
	Name   string `validate:"required,min=1,max=255"`
	Avatar string
}
