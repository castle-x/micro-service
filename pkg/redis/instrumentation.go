package redis

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var redisDuration metric.Float64Histogram

func init() {
	var err error
	redisDuration, err = otel.Meter("github.com/castlexu/micro-service/pkg/redis").Float64Histogram(
		"redis.client.duration",
		metric.WithUnit("ms"),
	)
	if err != nil {
		redisDuration = nil
	}
}

func startRedisOperation(ctx context.Context, operation string) (context.Context, func(error)) {
	attrs := []attribute.KeyValue{
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", operation),
	}
	ctx, span := otel.Tracer("github.com/castlexu/micro-service/pkg/redis").Start(
		ctx,
		"Redis "+operation,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...),
	)
	start := time.Now()
	return ctx, func(err error) {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		if redisDuration != nil {
			redisDuration.Record(ctx, float64(time.Since(start).Milliseconds()), metric.WithAttributes(attrs...))
		}
		span.End()
	}
}
