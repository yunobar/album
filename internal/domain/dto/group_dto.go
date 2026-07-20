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
	Name          string                 `json:"name"`
	InviteToken   string                 `json:"inviteToken"`
	Members       []MemberResponse       `json:"members"`
	ActiveSession *ActiveSessionResponse `json:"activeSession"`
}

// ActiveSessionResponse is the caller's in-flight session in this group, or
// nil — the only group→session resolution path in the API (ADR-0006). Never
// confirms a session exists to a member who isn't one of its participants.
type ActiveSessionResponse struct {
	ID     uuid.UUID `json:"id"`
	Method string    `json:"method"`
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
