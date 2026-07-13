package mapper

import (
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/entity"
)

func WatchlistItemToResponse(wi entity.WatchlistItem) dto.WatchlistItemResponse {
	return dto.WatchlistItemResponse{
		BaseDTO:  BaseToDTO(wi.BaseEntity),
		Priority: wi.Priority,
		Notes:    wi.Notes,
		Content:  ContentToResponse(wi.Content),
	}
}
