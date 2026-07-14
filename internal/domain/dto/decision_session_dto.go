package dto

import (
	"time"

	"github.com/google/uuid"
)

type CreateSessionRequest struct {
	Method              string      `json:"method" binding:"required,oneof=majority ranked priority roundRobin random"`
	ParticipantIDs      []uuid.UUID `json:"participantIds" binding:"required,min=1,dive,required"`
	CandidateContentIDs []uuid.UUID `json:"candidateContentIds" binding:"required,min=1,dive,required"`
}

type CastVoteRequest struct {
	ContentID uuid.UUID `json:"contentId" binding:"required"`
}

type SubmitRankingRequest struct {
	Ranking []uuid.UUID `json:"ranking" binding:"required,min=1,dive,required"`
}

type TallyResponse struct {
	Tally any `json:"tally"`
}

type CountsTally struct {
	Counts map[string]int `json:"counts"`
}

type RankedTally struct {
	Round                  int            `json:"round"`
	ActiveCandidateIDs     []uuid.UUID    `json:"activeCandidateIds"`
	EliminatedCandidateIDs []uuid.UUID    `json:"eliminatedCandidateIds"`
	Counts                 map[string]int `json:"counts"`
}

type SelectionTally struct {
	SelectedContentID *uuid.UUID `json:"selectedContentId"`
}

type SessionResponse struct {
	ID                      uuid.UUID         `json:"id"`
	GroupID                 uuid.UUID         `json:"groupId"`
	Method                  string            `json:"method"`
	Status                  string            `json:"status"`
	Participants            []MemberResponse  `json:"participants"`
	Candidates              []ContentResponse `json:"candidates"`
	CurrentChooserProfileID *uuid.UUID        `json:"currentChooserProfileId"`
	Tally                   any               `json:"tally"`
	WinnerContentID         *uuid.UUID        `json:"winnerContentId"`
	FinalizedAt             *time.Time        `json:"finalizedAt"`
}
