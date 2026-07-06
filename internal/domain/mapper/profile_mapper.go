package mapper

import (
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/entity"
)

func ProfileToResponse(profile entity.UserProfile, email string) dto.ProfileResponse {
	return dto.ProfileResponse{
		BaseDTO: BaseToDTO(profile.BaseEntity),
		UserID:  profile.UserID.UUID,
		Name:    profile.Name,
		Avatar:  profile.Avatar,
		Email:   email,
	}
}
