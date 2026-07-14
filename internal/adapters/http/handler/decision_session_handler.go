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

type DecisionSessionHandler struct {
	decisionSessionService service.DecisionSessionService
}

func NewDecisionSessionHandler(decisionSessionService service.DecisionSessionService) *DecisionSessionHandler {
	return &DecisionSessionHandler{decisionSessionService}
}

// HandleCreate godoc
// @Summary      Create a decision session
// @Tags         sessions
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        groupID path string true "Group ID"
// @Param        body body dto.CreateSessionRequest true "Create session payload"
// @Success      201  {object}  response.JSONResponse[dto.SessionResponse]
// @Failure      400  {object}  map[string]any
// @Failure      404  {object}  map[string]any
// @Router       /groups/{groupID}/sessions [post]
func (dsh *DecisionSessionHandler) HandleCreate() gin.HandlerFunc {
	return server.Handler("DecisionSessionHandler.HandleCreate", http.StatusCreated, func(ctx *gin.Context) (any, error) {
		profileID, err := getProfileID(ctx)
		if err != nil {
			return nil, err
		}

		groupID, err := server.GetRequiredPathParam[uuid.UUID](ctx, appconstant.ContextGroupID.String())
		if err != nil {
			return nil, err
		}

		request, err := server.BindJSON[dto.CreateSessionRequest](ctx)
		if err != nil {
			return nil, err
		}

		return dsh.decisionSessionService.Create(ctx.Request.Context(), profileID, groupID, request)
	})
}

// HandleGet godoc
// @Summary      Get a decision session's detail
// @Tags         sessions
// @Security     BearerAuth
// @Produce      json
// @Param        sessionID path string true "Session ID"
// @Success      200  {object}  response.JSONResponse[dto.SessionResponse]
// @Failure      404  {object}  map[string]any
// @Router       /sessions/{sessionID} [get]
func (dsh *DecisionSessionHandler) HandleGet() gin.HandlerFunc {
	return server.Handler("DecisionSessionHandler.HandleGet", http.StatusOK, func(ctx *gin.Context) (any, error) {
		profileID, err := getProfileID(ctx)
		if err != nil {
			return nil, err
		}

		sessionID, err := server.GetRequiredPathParam[uuid.UUID](ctx, appconstant.ContextSessionID.String())
		if err != nil {
			return nil, err
		}

		return dsh.decisionSessionService.Get(ctx.Request.Context(), profileID, sessionID)
	})
}

// HandleCastVote godoc
// @Summary      Cast or replace a Majority-method vote
// @Tags         sessions
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        sessionID path string true "Session ID"
// @Param        body body dto.CastVoteRequest true "Cast vote payload"
// @Success      200  {object}  response.JSONResponse[dto.TallyResponse]
// @Failure      400  {object}  map[string]any
// @Failure      404  {object}  map[string]any
// @Failure      409  {object}  map[string]any
// @Router       /sessions/{sessionID}/votes [post]
func (dsh *DecisionSessionHandler) HandleCastVote() gin.HandlerFunc {
	return server.Handler("DecisionSessionHandler.HandleCastVote", http.StatusOK, func(ctx *gin.Context) (any, error) {
		profileID, err := getProfileID(ctx)
		if err != nil {
			return nil, err
		}

		sessionID, err := server.GetRequiredPathParam[uuid.UUID](ctx, appconstant.ContextSessionID.String())
		if err != nil {
			return nil, err
		}

		request, err := server.BindJSON[dto.CastVoteRequest](ctx)
		if err != nil {
			return nil, err
		}

		return dsh.decisionSessionService.CastVote(ctx.Request.Context(), profileID, sessionID, request)
	})
}

// HandleSubmitRanking godoc
// @Summary      Submit or replace a Ranked-Choice ballot
// @Tags         sessions
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        sessionID path string true "Session ID"
// @Param        body body dto.SubmitRankingRequest true "Submit ranking payload"
// @Success      200  {object}  response.JSONResponse[dto.TallyResponse]
// @Failure      400  {object}  map[string]any
// @Failure      404  {object}  map[string]any
// @Failure      409  {object}  map[string]any
// @Router       /sessions/{sessionID}/rankings [post]
func (dsh *DecisionSessionHandler) HandleSubmitRanking() gin.HandlerFunc {
	return server.Handler("DecisionSessionHandler.HandleSubmitRanking", http.StatusOK, func(ctx *gin.Context) (any, error) {
		profileID, err := getProfileID(ctx)
		if err != nil {
			return nil, err
		}

		sessionID, err := server.GetRequiredPathParam[uuid.UUID](ctx, appconstant.ContextSessionID.String())
		if err != nil {
			return nil, err
		}

		request, err := server.BindJSON[dto.SubmitRankingRequest](ctx)
		if err != nil {
			return nil, err
		}

		return dsh.decisionSessionService.SubmitRanking(ctx.Request.Context(), profileID, sessionID, request)
	})
}

// HandleSelect godoc
// @Summary      Round Robin chooser pick
// @Tags         sessions
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        sessionID path string true "Session ID"
// @Param        body body dto.CastVoteRequest true "Select payload"
// @Success      200  {object}  response.JSONResponse[dto.TallyResponse]
// @Failure      400  {object}  map[string]any
// @Failure      403  {object}  map[string]any
// @Failure      404  {object}  map[string]any
// @Failure      409  {object}  map[string]any
// @Router       /sessions/{sessionID}/select [post]
func (dsh *DecisionSessionHandler) HandleSelect() gin.HandlerFunc {
	return server.Handler("DecisionSessionHandler.HandleSelect", http.StatusOK, func(ctx *gin.Context) (any, error) {
		profileID, err := getProfileID(ctx)
		if err != nil {
			return nil, err
		}

		sessionID, err := server.GetRequiredPathParam[uuid.UUID](ctx, appconstant.ContextSessionID.String())
		if err != nil {
			return nil, err
		}

		request, err := server.BindJSON[dto.CastVoteRequest](ctx)
		if err != nil {
			return nil, err
		}

		return dsh.decisionSessionService.Select(ctx.Request.Context(), profileID, sessionID, request)
	})
}

// HandleFinalize godoc
// @Summary      Finalize a decision session and lock in the winner
// @Tags         sessions
// @Security     BearerAuth
// @Produce      json
// @Param        sessionID path string true "Session ID"
// @Success      200  {object}  response.JSONResponse[dto.SessionResponse]
// @Failure      404  {object}  map[string]any
// @Failure      409  {object}  map[string]any
// @Router       /sessions/{sessionID}/finalize [post]
func (dsh *DecisionSessionHandler) HandleFinalize() gin.HandlerFunc {
	return server.Handler("DecisionSessionHandler.HandleFinalize", http.StatusOK, func(ctx *gin.Context) (any, error) {
		profileID, err := getProfileID(ctx)
		if err != nil {
			return nil, err
		}

		sessionID, err := server.GetRequiredPathParam[uuid.UUID](ctx, appconstant.ContextSessionID.String())
		if err != nil {
			return nil, err
		}

		return dsh.decisionSessionService.Finalize(ctx.Request.Context(), profileID, sessionID)
	})
}
