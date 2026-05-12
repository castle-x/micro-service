package db

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

func TestMongoOperationCreatesSpanWithLowCardinalityAttributes(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	oldTP := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(oldTP)
		require.NoError(t, tp.Shutdown(context.Background()))
	})

	_, end := startMongoOperation(context.Background(), "users", "findOne")
	end(nil)

	spans := sr.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, "MongoDB users.findOne", spans[0].Name())

	attrs := attrMap(spans[0].Attributes())
	assert.Equal(t, "mongodb", attrs["db.system"])
	assert.Equal(t, "users", attrs["db.collection.name"])
	assert.Equal(t, "findOne", attrs["db.operation"])
	assert.NotContains(t, attrs, "db.statement")
}

func attrMap(attrs []attribute.KeyValue) map[string]string {
	out := make(map[string]string, len(attrs))
	for _, attr := range attrs {
		out[string(attr.Key)] = attr.Value.AsString()
	}
	return out
}
