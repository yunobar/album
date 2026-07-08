package mapper

import (
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/entity"
)

func ContentToResponse(c entity.Content) dto.ContentResponse {
	return dto.ContentResponse{
		BaseDTO:     BaseToDTO(c.BaseEntity),
		ContentType: c.ContentType,
		Title:       c.Title,
		ReleaseYear: c.ReleaseYear,
		PosterUrl:   c.PosterURL,
	}
}
