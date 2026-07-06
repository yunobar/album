package middlewares

import (
	"github.com/gin-gonic/gin"
	"github.com/itsLeonB/ginkgo/pkg/middleware"
	"github.com/yunobar/album/internal/core/config"
	"github.com/yunobar/album/internal/core/logger"
)

type Middlewares struct {
	Err gin.HandlerFunc
}

func Provide(configs config.App) *Middlewares {
	middlewareProvider := middleware.NewMiddlewareProvider(logger.Global)
	errorMiddleware := middlewareProvider.NewErrorMiddleware()

	return &Middlewares{
		errorMiddleware,
	}
}
