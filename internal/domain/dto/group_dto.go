package dto

import "github.com/google/uuid"

type CreateGroupRequest struct {
	Name *string `json:"name"`
}

type MemberResponse struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Avatar string    `json:"avatar,omitempty"`
}

type GroupResponse struct {
	BaseDTO
	Name        string           `json:"name"`
	InviteToken string           `json:"inviteToken"`
	Members     []MemberResponse `json:"members"`
}

type GroupSummaryResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	MemberCount int       `json:"memberCount"`
}

type ListGroupsResponse struct {
	Groups []GroupSummaryResponse `json:"groups"`
}

type MergedWatchlistRequest struct {
	Filter string `form:"filter" binding:"omitempty,oneof=all movie tv"`
}

type MergedItemResponse struct {
	Content         ContentResponse   `json:"content"`
	InterestedCount int               `json:"interestedCount"`
	Members         []uuid.UUID       `json:"members"`
	Priorities      map[string]string `json:"priorities"`
}

type MergedWatchlistResponse struct {
	Filter string               `json:"filter"`
	Items  []MergedItemResponse `json:"items"`
}
