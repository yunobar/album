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

type WatchlistHandler struct {
	watchlistService service.WatchlistService
}

func NewWatchlistHandler(
	watchlistService service.WatchlistService,
) *WatchlistHandler {
	return &WatchlistHandler{
		watchlistService,
	}
}

// HandleList godoc
// @Summary      List current user's active watchlist items
// @Tags         watchlist
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  response.JSONResponse[[]dto.WatchlistItemResponse]
// @Failure      401  {object}  map[string]any
// @Router       /watchlist [get]
func (wh *WatchlistHandler) HandleList() gin.HandlerFunc {
	return server.Handler("WatchlistHandler.HandleList", http.StatusOK, func(ctx *gin.Context) (any, error) {
		profileID, err := getProfileID(ctx)
		if err != nil {
			return nil, err
		}

		return wh.watchlistService.List(ctx.Request.Context(), profileID)
	})
}

// HandleAdd godoc
// @Summary      Add a title to the watchlist
// @Tags         watchlist
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body body dto.AddWatchlistItemRequest true "Add watchlist item payload"
// @Success      201  {object}  response.JSONResponse[dto.WatchlistItemResponse]
// @Failure      400  {object}  map[string]any
// @Failure      409  {object}  map[string]any
// @Router       /watchlist [post]
func (wh *WatchlistHandler) HandleAdd() gin.HandlerFunc {
	return server.Handler("WatchlistHandler.HandleAdd", http.StatusCreated, func(ctx *gin.Context) (any, error) {
		profileID, err := getProfileID(ctx)
		if err != nil {
			return nil, err
		}

		request, err := server.BindJSON[dto.AddWatchlistItemRequest](ctx)
		if err != nil {
			return nil, err
		}

		return wh.watchlistService.Add(ctx.Request.Context(), profileID, request)
	})
}

// HandleUpdate godoc
// @Summary      Edit priority and/or notes of a watchlist item
// @Tags         watchlist
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        contentId path string true "Content ID"
// @Param        body body dto.UpdateWatchlistItemRequest true "Update watchlist item payload"
// @Success      200  {object}  response.JSONResponse[dto.WatchlistItemResponse]
// @Failure      400  {object}  map[string]any
// @Failure      404  {object}  map[string]any
// @Router       /watchlist/{contentId} [patch]
func (wh *WatchlistHandler) HandleUpdate() gin.HandlerFunc {
	return server.Handler("WatchlistHandler.HandleUpdate", http.StatusOK, func(ctx *gin.Context) (any, error) {
		profileID, err := getProfileID(ctx)
		if err != nil {
			return nil, err
		}

		contentID, err := server.GetRequiredPathParam[uuid.UUID](ctx, appconstant.ContextContentID.String())
		if err != nil {
			return nil, err
		}

		request, err := server.BindJSON[dto.UpdateWatchlistItemRequest](ctx)
		if err != nil {
			return nil, err
		}

		return wh.watchlistService.Update(ctx.Request.Context(), profileID, contentID, request)
	})
}

// HandleRemove godoc
// @Summary      Remove a title from the watchlist
// @Tags         watchlist
// @Security     BearerAuth
// @Param        contentId path string true "Content ID"
// @Success      204
// @Failure      404  {object}  map[string]any
// @Router       /watchlist/{contentId} [delete]
func (wh *WatchlistHandler) HandleRemove() gin.HandlerFunc {
	return server.Handler("WatchlistHandler.HandleRemove", http.StatusNoContent, func(ctx *gin.Context) (any, error) {
		profileID, err := getProfileID(ctx)
		if err != nil {
			return nil, err
		}

		contentID, err := server.GetRequiredPathParam[uuid.UUID](ctx, appconstant.ContextContentID.String())
		if err != nil {
			return nil, err
		}

		return nil, wh.watchlistService.Remove(ctx.Request.Context(), profileID, contentID)
	})
}
