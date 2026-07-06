package main

import (
	"context"
	"os"

	_ "github.com/joho/godotenv/autoload"
	"github.com/yunobar/album/internal/adapters/job"
	"github.com/yunobar/album/internal/core/config"
	"github.com/yunobar/album/internal/core/logger"
	"github.com/yunobar/album/internal/core/otel"
)

func main() {
	os.Exit(run())
}

func run() int {
	logger.Init("album-job")

	if err := config.Load(); err != nil {
		logger.Error(err)
		return 1
	}

	ctx := context.Background()
	otelShutdown, err := otel.InitSDK(ctx, config.Global.OTel)
	if err != nil {
		logger.Errorf("failed to initialize OTel SDK: %v", err)
		return 1
	}
	defer func() {
		if err := otelShutdown(ctx); err != nil {
			logger.Errorf("error shutting down OTel SDK: %v", err)
		}
	}()

	j, err := job.Setup(config.Global)
	if err != nil {
		logger.Error(err)
		return 1
	}

	if err = j.Run(); err != nil {
		logger.Error(err)
		return 1
	}

	return 0
}
