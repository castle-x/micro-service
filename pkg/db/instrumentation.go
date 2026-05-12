package db

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var mongoDuration metric.Float64Histogram

func init() {
	var err error
	mongoDuration, err = otel.Meter("github.com/castlexu/micro-service/pkg/db").Float64Histogram(
		"db.client.duration",
		metric.WithUnit("ms"),
	)
	if err != nil {
		mongoDuration = nil
	}
}

func startMongoOperation(ctx context.Context, collection, operation string) (context.Context, func(error)) {
	attrs := []attribute.KeyValue{
		attribute.String("db.system", "mongodb"),
		attribute.String("db.collection.name", collection),
		attribute.String("db.operation", operation),
	}
	ctx, span := otel.Tracer("github.com/castlexu/micro-service/pkg/db").Start(
		ctx,
		"MongoDB "+collection+"."+operation,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...),
	)
	start := time.Now()
	return ctx, func(err error) {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		if mongoDuration != nil {
			mongoDuration.Record(ctx, float64(time.Since(start).Milliseconds()), metric.WithAttributes(attrs...))
		}
		span.End()
	}
}
