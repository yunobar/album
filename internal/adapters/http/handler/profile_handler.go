package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	_ "github.com/itsLeonB/ginkgo/pkg/response"
	"github.com/itsLeonB/ginkgo/pkg/server"
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/service"
)

type ProfileHandler struct {
	profileService service.ProfileService
}

func NewProfileHandler(
	profileService service.ProfileService,
) *ProfileHandler {
	return &ProfileHandler{
		profileService,
	}
}

// HandleProfile godoc
// @Summary      Get current user's profile
// @Tags         profile
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  response.JSONResponse[dto.ProfileResponse]
// @Failure      401  {object}  map[string]any
// @Router       /profile [get]
func (ph *ProfileHandler) HandleProfile() gin.HandlerFunc {
	return server.Handler("ProfileHandler.HandleProfile", http.StatusOK, func(ctx *gin.Context) (any, error) {
		profileID, err := getProfileID(ctx)
		if err != nil {
			return nil, err
		}

		return ph.profileService.GetByID(ctx.Request.Context(), profileID)
	})
}

// HandleUpdate godoc
// @Summary      Update current user's profile
// @Tags         profile
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body body dto.UpdateProfileRequest true "Update profile payload"
// @Success      200  {object}  response.JSONResponse[dto.ProfileResponse]
// @Failure      400  {object}  map[string]any
// @Failure      401  {object}  map[string]any
// @Router       /profile [patch]
func (ph *ProfileHandler) HandleUpdate() gin.HandlerFunc {
	return server.Handler("ProfileHandler.HandleUpdate", http.StatusOK, func(ctx *gin.Context) (any, error) {
		profileID, err := getProfileID(ctx)
		if err != nil {
			return nil, err
		}

		request, err := server.BindJSON[dto.UpdateProfileRequest](ctx)
		if err != nil {
			return nil, err
		}

		request.ID = profileID

		return ph.profileService.Update(ctx.Request.Context(), request)
	})
}
