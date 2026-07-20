package mapper

import (
	"github.com/google/uuid"
	"github.com/itsLeonB/ezutil/v2"
	"github.com/yunobar/album/internal/appconstant"
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/entity"
)

// MethodToDB translates the API's camelCase method name to the DB's enum
// value. roundRobin/round_robin is the only differing pair. Exported because
// the service package needs it to build the entity.DecisionSession row on
// create.
func MethodToDB(apiMethod string) string {
	if apiMethod == "roundRobin" {
		return appconstant.SessionMethodRoundRobin
	}
	return apiMethod
}

// MethodToAPI is MethodToDB's inverse.
func MethodToAPI(dbMethod string) string {
	if dbMethod == appconstant.SessionMethodRoundRobin {
		return "roundRobin"
	}
	return dbMethod
}

// SessionToResponse is pure/stateless like every other mapper — the caller
// computes currentChooserProfileID (it needs a DB round-trip) and passes it
// in rather than this function deriving it.
func SessionToResponse(session entity.DecisionSession, currentChooserProfileID *uuid.UUID, tally any) dto.SessionResponse {
	return dto.SessionResponse{
		ID:      session.ID,
		GroupID: session.GroupID,
		Method:  MethodToAPI(session.Method),
		Status:  session.Status,
		Participants: ezutil.MapSlice(session.Participants, func(p entity.SessionParticipant) dto.MemberResponse {
			return MemberToResponse(p.Profile)
		}),
		Candidates: ezutil.MapSlice(session.Candidates, func(c entity.SessionCandidate) dto.ContentResponse {
			return ContentToResponse(c.Content)
		}),
		CurrentChooserProfileID: currentChooserProfileID,
		Tally:                   tally,
		WinnerContentID:         session.WinnerContentID,
		FinalizedAt:             session.FinalizedAt,
	}
}
