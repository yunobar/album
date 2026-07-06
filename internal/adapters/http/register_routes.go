package http

import (
	"os"

	"github.com/gin-gonic/gin"
	"github.com/itsLeonB/go-authkit"
	"github.com/itsLeonB/go-authkit/authgin"
	"github.com/kroma-labs/sentinel-go/httpserver"
	sentinelGin "github.com/kroma-labs/sentinel-go/httpserver/adapters/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	_ "github.com/yunobar/album/docs"
	"github.com/yunobar/album/internal/adapters/http/handler"
	"github.com/yunobar/album/internal/adapters/http/middlewares"
	"github.com/yunobar/album/internal/adapters/http/routes"
	"github.com/yunobar/album/internal/core/config"
	"github.com/yunobar/album/internal/provider"
)

func RegisterRoutes(router *gin.Engine, configs config.Config, services *provider.Services) (func(), error) {
	transport, err := authgin.NewCookieTransport(authgin.CookieConfig{
		Domain:     configs.CookieDomain,
		Secure:     configs.CookieSecure,
		SameSite:   configs.ParsedSameSite(),
		AccessTTL:  configs.TokenDuration,
		RefreshTTL: configs.RefreshTokenDuration,
	})
	if err != nil {
		return nil, err
	}

	authMW := authgin.AuthMiddleware(services.AuthKit, transport, authkit.RequireAuth)

	handlers := handler.ProvideHandlers(services, transport)
	mw := middlewares.Provide(configs.App)

	router.Use(mw.Err)

	sentinelGin.RegisterHealth(router, httpserver.NewHealthHandler())

	if configs.Env != "release" {
		sentinelGin.RegisterPprof(router, httpserver.DefaultPprofConfig())

		// Swagger UI: /docs/index.html
		router.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

		// Markdown docs: /docs.md
		router.GET("/docs.md", func(ctx *gin.Context) {
			data, err := os.ReadFile("docs/docs.md")
			if err != nil {
				ctx.Status(404)
				return
			}
			ctx.Data(200, "text/markdown; charset=utf-8", data)
		})
	}

	routes.RegisterBaseRoutes(router)
	routes.RegisterAPIRoutes(router, handlers, authMW)

	return handlers.Shutdown, nil
}
