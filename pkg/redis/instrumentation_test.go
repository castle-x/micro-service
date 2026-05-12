package redis

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestSetGetCreateRedisSpansWithoutValues(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	oldTP := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(oldTP)
		require.NoError(t, tp.Shutdown(context.Background()))
	})

	c, _ := setup(t)
	ctx := context.Background()
	require.NoError(t, c.Set(ctx, "secret:key", "super-secret-value", 0))
	_, err := c.Get(ctx, "secret:key")
	require.NoError(t, err)

	spans := sr.Ended()
	require.Len(t, spans, 2)

	for _, span := range spans {
		attrs := attrMap(span.Attributes())
		assert.Equal(t, "redis", attrs["db.system"])
		assert.Contains(t, []string{"SET", "GET"}, attrs["db.operation"])
		assert.NotContains(t, attrs, "db.statement")
		assert.NotContains(t, attrs, "redis.value")
	}
}

func attrMap(attrs []attribute.KeyValue) map[string]string {
	out := make(map[string]string, len(attrs))
	for _, attr := range attrs {
		out[string(attr.Key)] = attr.Value.AsString()
	}
	return out
}
