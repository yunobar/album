package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	_ "github.com/itsLeonB/ginkgo/pkg/response"
	"github.com/itsLeonB/ginkgo/pkg/server"
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/service"
)

type ContentHandler struct {
	contentService service.ContentService
}

func NewContentHandler(
	contentService service.ContentService,
) *ContentHandler {
	return &ContentHandler{
		contentService,
	}
}

// HandleSearch godoc
// @Summary      Search content
// @Tags         content
// @Security     BearerAuth
// @Produce      json
// @Param        q query string true "Search query"
// @Success      200  {object}  response.JSONResponse[[]dto.ContentResponse]
// @Failure      400  {object}  map[string]any
// @Router       /content/search [get]
func (ch *ContentHandler) HandleSearch() gin.HandlerFunc {
	return server.Handler("ContentHandler.HandleSearch", http.StatusOK, func(ctx *gin.Context) (any, error) {
		request, err := server.BindRequest[dto.ContentSearchRequest](ctx, binding.Query)
		if err != nil {
			return nil, err
		}

		return ch.contentService.Search(ctx.Request.Context(), request.Query)
	})
}
