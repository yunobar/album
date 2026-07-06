package http

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/itsLeonB/ezutil/v2/zerolog"
	"github.com/kroma-labs/sentinel-go/httpserver"
	"github.com/yunobar/album/internal/core/config"
	"github.com/yunobar/album/internal/core/logger"
	"github.com/yunobar/album/internal/provider"
)

func Setup(configs config.Config) (*httpserver.Server, func(), error) {
	providers, err := provider.All()
	if err != nil {
		return nil, nil, err
	}

	gin.SetMode(configs.Env)
	r := gin.New()
	r.HandleMethodNotAllowed = true

	zerologger := zerolog.Instance(logger.Global)

	skipPaths := []string{"/ping", "/livez", "/readyz", "/metrics", "/favicon.ico"}
	if err = setupSentinel(r, skipPaths, zerologger); err != nil {
		if sErr := providers.Shutdown(); sErr != nil {
			logger.Error(sErr)
		}
		return nil, nil, err
	}

	routesShutdown, err := RegisterRoutes(r, configs, providers.Services)
	if err != nil {
		if sErr := providers.Shutdown(); sErr != nil {
			logger.Error(sErr)
		}
		return nil, nil, err
	}

	shutdownFunc := func() {
		routesShutdown()
		if err := providers.Shutdown(); err != nil {
			logger.Error(err)
		}
	}

	httpCfg := httpserver.ProductionConfig()
	httpCfg.LoggerConfig = &httpserver.LoggerConfig{
		Logger:    zerologger,
		SkipPaths: skipPaths,
	}
	httpCfg.Addr = fmt.Sprintf(":%s", configs.App.Port)

	srv := httpserver.New(
		httpserver.WithConfig(httpCfg),
		httpserver.WithServiceName(configs.ServiceName),
		httpserver.WithHandler(r),
		httpserver.WithLogger(zerologger),
	)

	return srv, shutdownFunc, nil
}
