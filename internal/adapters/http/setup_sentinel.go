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
	corsCfg := httpserver.CORSConfig{
		AllowedOrigins: config.Global.ClientUrls,
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
			"X-Requested-With",
			"Content-Length",
			"Accept-Encoding",
			"X-CSRF-Token",
			"Cache-Control",
			"Referer",
			"User-Agent",
			"range",
			"DNT",
			"sec-ch-ua",
			"sec-ch-ua-platform",
			"sec-ch-ua-mobile",
		},
		ExposedHeaders:   []string{"Content-Length", "X-Total-Count"},
		AllowCredentials: true,
	}

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
		sentinelGin.CORS(corsCfg),
		sentinelGin.Timeout(config.Global.Timeout),
		sentinelGin.Metrics(metrics),
		sentinelGin.RateLimit(httpserver.RateLimitConfig{
			// ponytail: per-IP global limit. 100 req/s burst 200 is generous for
			// legitimate users but stops single-IP floods. Ceiling: doesn't help
			// distributed DDoS. Upgrade path: Redis-backed limiter + WAF.
			Limit:   100,
			Burst:   200,
			KeyFunc: httpserver.KeyFuncByIP(),
		}),
	)

	return nil
}
