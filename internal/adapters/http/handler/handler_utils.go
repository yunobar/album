package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/itsLeonB/ginkgo/pkg/response"
	"github.com/itsLeonB/ginkgo/pkg/server"
	"github.com/yunobar/album/internal/appconstant"
)

func getProfileID(ctx *gin.Context) (uuid.UUID, error) {
	return server.GetAndParseFromContext[uuid.UUID](ctx, appconstant.ContextProfileID.String())
}
