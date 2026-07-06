package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/itsLeonB/ginkgo/pkg/middleware"
	"github.com/yunobar/album/internal/appconstant"
	"github.com/yunobar/album/internal/core/logger"
)

func init() {
	gin.SetMode(gin.TestMode)
	logger.Init("album-test")
}

// testErrorMiddleware delegates to the real production error middleware so tests
// stay aligned with actual status-code mapping.
func testErrorMiddleware() gin.HandlerFunc {
	return middleware.NewMiddlewareProvider(logger.Global).NewErrorMiddleware()
}

func injectProfileID(profileID uuid.UUID) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Set(appconstant.ContextProfileID.String(), profileID.String())
		ctx.Next()
	}
}
