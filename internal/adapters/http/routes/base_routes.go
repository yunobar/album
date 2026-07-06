package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/itsLeonB/ungerr"
)

func RegisterBaseRoutes(r *gin.Engine) {
	r.NoMethod(func(ctx *gin.Context) {
		if ctx.Request.Method == http.MethodOptions {
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}
		_ = ctx.Error(ungerr.MethodNotAllowedError("method not allowed"))
	})

	r.NoRoute(func(ctx *gin.Context) {
		if ctx.Request.Method == http.MethodOptions {
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}
		if ctx.Request.URL.Path == "/favicon.ico" {
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}
		_ = ctx.Error(ungerr.NotFoundError("route not found"))
	})
}
