package mq

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestPublishConsumeContextPropagation(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	oldTP := otel.GetTracerProvider()
	oldProp := otel.GetTextMapPropagator()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		otel.SetTracerProvider(oldTP)
		otel.SetTextMapPropagator(oldProp)
		require.NoError(t, tp.Shutdown(context.Background()))
	})

	ctx, publishEnd := StartPublishSpan(context.Background(), "billing.order_paid")
	msg := &Message{Topic: "billing.order_paid", Payload: []byte(`{"secret":"hidden"}`)}
	InjectContext(ctx, msg)
	publishEnd(nil)

	var consumeTraceID trace.TraceID
	handler := InstrumentHandler("billing.order_paid", func(ctx context.Context, msg *Message) error {
		consumeTraceID = trace.SpanContextFromContext(ctx).TraceID()
		return nil
	})
	require.NoError(t, handler(context.Background(), msg))

	spans := sr.Ended()
	require.Len(t, spans, 2)
	assert.Equal(t, spans[0].SpanContext().TraceID(), consumeTraceID)

	byName := make(map[string]sdktrace.ReadOnlySpan, len(spans))
	for _, span := range spans {
		byName[span.Name()] = span
		attrs := attrMap(span.Attributes())
		assert.Equal(t, "nsq", attrs["messaging.system"])
		assert.Equal(t, "billing.order_paid", attrs["messaging.destination.name"])
		assert.NotContains(t, attrs, "messaging.message.payload")
	}
	publishSpan := byName["NSQ publish billing.order_paid"]
	consumeSpan := byName["NSQ consume billing.order_paid"]
	require.NotNil(t, publishSpan)
	require.NotNil(t, consumeSpan)
	assert.Equal(t, publishSpan.SpanContext().TraceID(), consumeSpan.SpanContext().TraceID())
	assert.Equal(t, publishSpan.SpanContext().SpanID(), consumeSpan.Parent().SpanID())
}

func attrMap(attrs []attribute.KeyValue) map[string]string {
	out := make(map[string]string, len(attrs))
	for _, attr := range attrs {
		out[string(attr.Key)] = attr.Value.AsString()
	}
	return out
}
