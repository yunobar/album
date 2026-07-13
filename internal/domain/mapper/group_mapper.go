package mapper

import (
	"strings"

	"github.com/google/uuid"
	"github.com/itsLeonB/ezutil/v2"
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/entity"
)

// defaultGroupName is shown for a group of one (only the caller has joined).
const defaultGroupName = "New group"

func MemberToResponse(profile entity.UserProfile) dto.MemberResponse {
	return dto.MemberResponse{
		ID:     profile.ID,
		Name:   profile.Name,
		Avatar: profile.Avatar,
	}
}

func GroupToResponse(group entity.Group, members []entity.GroupMember, callerProfileID uuid.UUID) dto.GroupResponse {
	return dto.GroupResponse{
		BaseDTO:     BaseToDTO(group.BaseEntity),
		Name:        resolveGroupName(group, members, callerProfileID),
		InviteToken: group.InviteToken,
		Members: ezutil.MapSlice(members, func(m entity.GroupMember) dto.MemberResponse {
			return MemberToResponse(m.Profile)
		}),
	}
}

func GroupToSummary(group entity.Group, members []entity.GroupMember, callerProfileID uuid.UUID) dto.GroupSummaryResponse {
	return dto.GroupSummaryResponse{
		ID:          group.ID,
		Name:        resolveGroupName(group, members, callerProfileID),
		MemberCount: len(members),
	}
}

// resolveGroupName returns the persisted name, or derives one from the other
// members' names when unset. Derivation is per-request, never persisted, so
// the same group reads as "Bob & Carol" to Alice and "Alice & Carol" to Bob.
func resolveGroupName(group entity.Group, members []entity.GroupMember, callerProfileID uuid.UUID) string {
	if group.Name != nil && *group.Name != "" {
		return *group.Name
	}

	var others []string
	for _, m := range members {
		if m.ProfileID != callerProfileID {
			others = append(others, m.Profile.Name)
		}
	}

	if len(others) == 0 {
		return defaultGroupName
	}
	if len(others) == 1 {
		return others[0]
	}

	return strings.Join(others[:len(others)-1], ", ") + " & " + others[len(others)-1]
}
