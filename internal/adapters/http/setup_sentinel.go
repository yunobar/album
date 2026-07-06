package http

import (
	"github.com/gin-gonic/gin"
	"github.com/itsLeonB/ungerr"
	"github.com/kroma-labs/sentinel-go/httpserver"
	sentinelGin "github.com/kroma-labs/sentinel-go/httpserver/adapters/gin"
	"github.com/rs/zerolog"
	"github.com/yunobar/album/internal/core/config"
)

func setupSentinel(router *gin.Engine, skipPaths []string, logger zerolog.Logger) error {
	metricsCfg := httpserver.DefaultMetricsConfig()
	metricsCfg.SkipPaths = skipPaths

	metrics, err := httpserver.NewMetrics(metricsCfg)
	if err != nil {
		return ungerr.Wrap(err, "error setting up metrics config")
	}

	tracingCfg := httpserver.DefaultTracingConfig()
	tracingCfg.SkipPaths = skipPaths

	router.Use(
		sentinelGin.Recovery(logger),
		sentinelGin.Tracing(tracingCfg),
		// sentinelGin.RequestID(),
		// sentinelGin.Logger(httpserver.LoggerConfig{
		// 	Logger:    logger,
		// 	SkipPaths: []string{"/ping", "/livez", "/readyz", "/metrics"},
		// }),
		sentinelGin.Timeout(config.Global.Timeout),
		sentinelGin.Metrics(metrics),
		sentinelGin.RateLimit(httpserver.DefaultRateLimitConfig()),
	)

	return nil
}
