package mq

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

var mqDuration metric.Float64Histogram

func init() {
	var err error
	mqDuration, err = otel.Meter("github.com/castlexu/micro-service/pkg/mq").Float64Histogram(
		"mq.consume.duration",
		metric.WithUnit("ms"),
	)
	if err != nil {
		mqDuration = nil
	}
}

// InjectContext injects W3C trace context into message headers.
func InjectContext(ctx context.Context, msg *Message) {
	if msg == nil {
		return
	}
	if msg.Headers == nil {
		msg.Headers = make(map[string]string)
	}
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(msg.Headers))
}

// ExtractContext extracts W3C trace context from message headers.
func ExtractContext(ctx context.Context, msg *Message) context.Context {
	if msg == nil || len(msg.Headers) == 0 {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(msg.Headers))
}

// StartPublishSpan starts an NSQ publish span. Call the returned function with the publish result.
func StartPublishSpan(ctx context.Context, topic string) (context.Context, func(error)) {
	attrs := []attribute.KeyValue{
		attribute.String("messaging.system", "nsq"),
		attribute.String("messaging.destination.name", topic),
	}
	ctx, span := otel.Tracer("github.com/castlexu/micro-service/pkg/mq").Start(
		ctx,
		"NSQ publish "+topic,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(attrs...),
	)
	return ctx, func(err error) {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}
}

// InstrumentHandler wraps a consumer handler with context extraction and a consume span.
func InstrumentHandler(topic string, handler HandlerFunc) HandlerFunc {
	return func(ctx context.Context, msg *Message) error {
		ctx = ExtractContext(ctx, msg)
		attrs := []attribute.KeyValue{
			attribute.String("messaging.system", "nsq"),
			attribute.String("messaging.destination.name", topic),
		}
		ctx, span := otel.Tracer("github.com/castlexu/micro-service/pkg/mq").Start(
			ctx,
			"NSQ consume "+topic,
			trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(attrs...),
		)
		start := time.Now()
		err := handler(ctx, msg)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		if mqDuration != nil {
			mqDuration.Record(ctx, float64(time.Since(start).Milliseconds()), metric.WithAttributes(attrs...))
		}
		span.End()
		return err
	}
}
