package otel

import (
	"context"
	"errors"
	"time"

	"github.com/itsLeonB/ungerr"
	"github.com/yunobar/album/internal/core/config"
	"github.com/yunobar/album/internal/core/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type component struct {
	enabled bool
	initFn  func(context.Context, *resource.Resource, config.OTel) (func(context.Context) error, error)
}

var Tracer trace.Tracer = noop.NewTracerProvider().Tracer("")

// InitSDK initializes the OpenTelemetry SDK for metrics and logs.
func InitSDK(ctx context.Context, cfg config.OTel) (func(context.Context) error, error) {
	if !cfg.Enabled || (!cfg.MetricsEnabled && !cfg.LogsEnabled && !cfg.TracesEnabled) {
		return func(context.Context) error { return nil }, nil
	}

	var shutdownFuncs []func(context.Context) error
	shutdown := func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		if err != nil {
			return ungerr.Wrap(err, "error shutting down otel resources")
		}
		return nil
	}

	handleErr := func() {
		if e := shutdown(ctx); e != nil {
			logger.Error(e)
		}
	}

	// Resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceInstanceID(cfg.ServiceInstanceId),
		),
	)
	if err != nil {
		return nil, ungerr.Wrap(err, "failed to create resource")
	}

	// Propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	components := []component{
		{
			enabled: cfg.MetricsEnabled,
			initFn:  initMetrics,
		},
		{
			enabled: cfg.LogsEnabled,
			initFn:  initLogs,
		},
		{
			enabled: cfg.TracesEnabled,
			initFn:  initTraces,
		},
	}

	for _, component := range components {
		if !component.enabled {
			continue
		}
		shutdownFunc, err := component.initFn(ctx, res, cfg)
		if err != nil {
			handleErr()
			return nil, err
		}
		shutdownFuncs = append(shutdownFuncs, shutdownFunc)
	}

	return shutdown, nil
}

func initMetrics(ctx context.Context, res *resource.Resource, cfg config.OTel) (func(context.Context) error, error) {
	metricExporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithTimeout(cfg.ExportTimeout),
	)
	if err != nil {
		return nil, ungerr.Wrap(err, "failed to create metric exporter")
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter,
			sdkmetric.WithInterval(15*time.Second),
			sdkmetric.WithTimeout(cfg.ExportTimeout),
		)),
		sdkmetric.WithCardinalityLimit(cfg.MaxQueueSize),
	)
	otel.SetMeterProvider(meterProvider)

	return meterProvider.Shutdown, nil
}

func initLogs(ctx context.Context, res *resource.Resource, cfg config.OTel) (func(context.Context) error, error) {
	logExporter, err := otlploghttp.New(ctx)
	if err != nil {
		return nil, ungerr.Wrap(err, "failed to create log exporter")
	}

	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(
			sdklog.NewBatchProcessor(
				logExporter,
				sdklog.WithMaxQueueSize(cfg.MaxQueueSize),
				sdklog.WithExportMaxBatchSize(cfg.MaxExportBatchSize),
				sdklog.WithExportInterval(cfg.BatchTimeout),
				sdklog.WithExportTimeout(cfg.ExportTimeout),
			),
		),
	)
	global.SetLoggerProvider(loggerProvider)

	return loggerProvider.Shutdown, nil
}

func initTraces(ctx context.Context, res *resource.Resource, cfg config.OTel) (func(context.Context) error, error) {
	traceExporter, err := otlptrace.New(ctx, otlptracehttp.NewClient())
	if err != nil {
		return nil, ungerr.Wrap(err, "failed to create trace exporter")
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(
			traceExporter,
			sdktrace.WithMaxQueueSize(cfg.MaxQueueSize),
			sdktrace.WithMaxExportBatchSize(cfg.MaxExportBatchSize),
			sdktrace.WithBatchTimeout(cfg.BatchTimeout),
			sdktrace.WithExportTimeout(cfg.ExportTimeout),
		),
	)
	otel.SetTracerProvider(tracerProvider)

	Tracer = tracerProvider.Tracer(cfg.ServiceName)

	return tracerProvider.Shutdown, nil
}
