package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/itsLeonB/ginkgo/pkg/response"
	"github.com/itsLeonB/ginkgo/pkg/server"
	"github.com/yunobar/album/internal/appconstant"
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/service"
)

type GroupHandler struct {
	groupService service.GroupService
}

func NewGroupHandler(groupService service.GroupService) *GroupHandler {
	return &GroupHandler{groupService}
}

// HandleCreate godoc
// @Summary      Create a group
// @Tags         groups
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body body dto.CreateGroupRequest true "Create group payload"
// @Success      201  {object}  response.JSONResponse[dto.GroupResponse]
// @Failure      400  {object}  map[string]any
// @Router       /groups [post]
func (gh *GroupHandler) HandleCreate() gin.HandlerFunc {
	return server.Handler("GroupHandler.HandleCreate", http.StatusCreated, func(ctx *gin.Context) (any, error) {
		profileID, err := getProfileID(ctx)
		if err != nil {
			return nil, err
		}

		request, err := server.BindJSON[dto.CreateGroupRequest](ctx)
		if err != nil {
			return nil, err
		}

		return gh.groupService.Create(ctx.Request.Context(), profileID, request)
	})
}

// HandleList godoc
// @Summary      List the caller's groups
// @Tags         groups
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  response.JSONResponse[dto.ListGroupsResponse]
// @Router       /groups [get]
func (gh *GroupHandler) HandleList() gin.HandlerFunc {
	return server.Handler("GroupHandler.HandleList", http.StatusOK, func(ctx *gin.Context) (any, error) {
		profileID, err := getProfileID(ctx)
		if err != nil {
			return nil, err
		}

		return gh.groupService.List(ctx.Request.Context(), profileID)
	})
}

// HandleGet godoc
// @Summary      Get a group's detail
// @Tags         groups
// @Security     BearerAuth
// @Produce      json
// @Param        groupID path string true "Group ID"
// @Success      200  {object}  response.JSONResponse[dto.GroupResponse]
// @Failure      404  {object}  map[string]any
// @Router       /groups/{groupID} [get]
func (gh *GroupHandler) HandleGet() gin.HandlerFunc {
	return server.Handler("GroupHandler.HandleGet", http.StatusOK, func(ctx *gin.Context) (any, error) {
		profileID, err := getProfileID(ctx)
		if err != nil {
			return nil, err
		}

		groupID, err := server.GetRequiredPathParam[uuid.UUID](ctx, appconstant.ContextGroupID.String())
		if err != nil {
			return nil, err
		}

		return gh.groupService.Get(ctx.Request.Context(), profileID, groupID)
	})
}

// HandleJoin godoc
// @Summary      Join a group by invite token
// @Tags         groups
// @Security     BearerAuth
// @Produce      json
// @Param        token path string true "Invite token"
// @Success      200  {object}  response.JSONResponse[dto.GroupResponse]
// @Failure      404  {object}  map[string]any
// @Router       /groups/join/{token} [post]
func (gh *GroupHandler) HandleJoin() gin.HandlerFunc {
	return server.Handler("GroupHandler.HandleJoin", http.StatusOK, func(ctx *gin.Context) (any, error) {
		profileID, err := getProfileID(ctx)
		if err != nil {
			return nil, err
		}

		token, err := server.GetRequiredPathParam[string](ctx, appconstant.ContextToken.String())
		if err != nil {
			return nil, err
		}

		return gh.groupService.Join(ctx.Request.Context(), profileID, token)
	})
}
