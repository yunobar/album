package mapper

import (
	"github.com/itsLeonB/go-crud"
	"github.com/yunobar/album/internal/domain/dto"
)

func BaseToDTO(be crud.BaseEntity) dto.BaseDTO {
	return dto.BaseDTO{
		ID:        be.ID,
		CreatedAt: be.CreatedAt,
		UpdatedAt: be.UpdatedAt,
	}
}
