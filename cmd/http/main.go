// @title           Album API
// @version         1.0
// @description     Nobar backend API
// @host            localhost:8080
// @BasePath        /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
package main

import (
	"context"
	"os"

	_ "github.com/joho/godotenv/autoload"
	"github.com/yunobar/album/internal/adapters/http"
	"github.com/yunobar/album/internal/core/config"
	"github.com/yunobar/album/internal/core/logger"
	"github.com/yunobar/album/internal/core/otel"
)

func main() {
	os.Exit(run())
}

func run() int {
	logger.Init("album-http")

	if err := config.Load(); err != nil {
		logger.Error(err)
		return 1
	}

	ctx := context.Background()
	otelShutdown, err := otel.InitSDK(ctx, config.Global.OTel)
	if err != nil {
		logger.Error(err)
		return 1
	}
	defer func() {
		if err := otelShutdown(ctx); err != nil {
			logger.Error(err)
		}
	}()

	srv, shutdownFunc, err := http.Setup(*config.Global)
	if err != nil {
		logger.Error(err)
		return 1
	}
	defer shutdownFunc()

	if err := srv.ListenAndServe(ctx); err != nil {
		logger.Error(err)
		return 1
	}

	return 0
}
