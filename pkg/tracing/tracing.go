// Package tracing provides OpenTelemetry initialization with OTLP HTTP export.
package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.uber.org/zap"
)

// Config holds tracing configuration.
type Config struct {
	Enabled      bool    `mapstructure:"enabled"`
	ServiceName  string  `mapstructure:"service_name"`
	OTLPEndpoint string  `mapstructure:"otlp_endpoint"`
	Sampler      float64 `mapstructure:"sampler"`
}

// Init initialises the OpenTelemetry TracerProvider.
// If tracing is disabled it returns a no-op shutdown function.
func Init(ctx context.Context, cfg Config, logger *zap.Logger) (shutdown func(context.Context) error, err error) {
	noop := func(context.Context) error { return nil }

	if !cfg.Enabled {
		logger.Info("tracing disabled, using noop provider")
		return noop, nil
	}

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(cfg.OTLPEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return noop, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
		),
	)
	if err != nil {
		return noop, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.Sampler)),
	)

	otel.SetTracerProvider(tp)

	logger.Info("tracing initialised",
		zap.String("service", cfg.ServiceName),
		zap.String("endpoint", cfg.OTLPEndpoint),
		zap.Float64("sampler", cfg.Sampler),
	)

	return tp.Shutdown, nil
}
