// Package otel owns process-wide OpenTelemetry initialization.
package otel

import (
	"context"
	"errors"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
)

// ShutdownFunc releases OpenTelemetry providers created by Init.
type ShutdownFunc func(context.Context) error

// Config describes service OpenTelemetry settings.
type Config struct {
	Enabled        bool    `mapstructure:"enabled"`
	Endpoint       string  `mapstructure:"endpoint"`
	Protocol       string  `mapstructure:"protocol"`
	Environment    string  `mapstructure:"environment"`
	ServiceVersion string  `mapstructure:"service_version"`
	SampleRatio    float64 `mapstructure:"sample_ratio"`
	Strict         bool    `mapstructure:"strict"`
	Insecure       bool    `mapstructure:"insecure"`
}

// Init installs global tracer, meter, and propagation providers.
func Init(ctx context.Context, serviceName string, cfg Config) (ShutdownFunc, error) {
	cfg = cfg.withDefaults()
	if !cfg.Enabled || cfg.Endpoint == "" {
		return noopShutdown, nil
	}
	if serviceName == "" {
		return nil, errors.New("otel.Init: serviceName is required")
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			"",
			attribute.String("service.name", serviceName),
			attribute.String("service.version", cfg.ServiceVersion),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, err
	}

	traceExporter, metricExporter, err := cfg.exporters(ctx)
	if err != nil {
		if cfg.Strict {
			return nil, err
		}
		return noopShutdown, nil
	}

	tp := trace.NewTracerProvider(
		trace.WithResource(res),
		trace.WithSampler(cfg.sampler()),
		trace.WithBatcher(traceExporter),
	)
	mp := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(metricExporter)),
	)

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return func(ctx context.Context) error {
		return errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx))
	}, nil
}

func (c Config) withDefaults() Config {
	if c.Protocol == "" {
		c.Protocol = "grpc"
	}
	if c.Environment == "" {
		c.Environment = "local"
	}
	if c.SampleRatio < 0 {
		c.SampleRatio = 0
	}
	if c.SampleRatio > 1 {
		c.SampleRatio = 1
	}
	return c
}

func (c Config) sampler() trace.Sampler {
	env := strings.ToLower(c.Environment)
	if env == "local" || env == "staging" {
		return trace.ParentBased(trace.AlwaysSample())
	}
	return trace.ParentBased(trace.TraceIDRatioBased(c.SampleRatio))
}

func (c Config) exporters(ctx context.Context) (trace.SpanExporter, metric.Exporter, error) {
	switch strings.ToLower(c.Protocol) {
	case "", "grpc":
		traceOpts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(c.Endpoint)}
		metricOpts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(c.Endpoint)}
		if c.Insecure {
			traceOpts = append(traceOpts, otlptracegrpc.WithInsecure())
			metricOpts = append(metricOpts, otlpmetricgrpc.WithInsecure())
		}
		te, err := otlptracegrpc.New(ctx, traceOpts...)
		if err != nil {
			return nil, nil, err
		}
		me, err := otlpmetricgrpc.New(ctx, metricOpts...)
		if err != nil {
			return nil, nil, err
		}
		return te, me, nil
	case "http":
		traceOpts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(c.Endpoint)}
		metricOpts := []otlpmetrichttp.Option{otlpmetrichttp.WithEndpoint(c.Endpoint)}
		if c.Insecure {
			traceOpts = append(traceOpts, otlptracehttp.WithInsecure())
			metricOpts = append(metricOpts, otlpmetrichttp.WithInsecure())
		}
		te, err := otlptracehttp.New(ctx, traceOpts...)
		if err != nil {
			return nil, nil, err
		}
		me, err := otlpmetrichttp.New(ctx, metricOpts...)
		if err != nil {
			return nil, nil, err
		}
		return te, me, nil
	default:
		return nil, nil, errors.New("otel.Init: unsupported protocol")
	}
}

func noopShutdown(context.Context) error { return nil }
