package kitex

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/castlexu/micro-service/pkg/errno"
	mw "github.com/castlexu/micro-service/pkg/middleware"
)

// 构造一个空的下一跳 endpoint，便于测试 middleware 行为。
func okEndpoint(ctx context.Context, _, _ any) error {
	_ = ctx
	return nil
}

func TestTrace_GeneratesWhenMissing(t *testing.T) {
	var captured string
	next := func(ctx context.Context, _, _ any) error {
		captured, _ = metainfo.GetPersistentValue(ctx, mw.MetaKeyTraceID)
		return nil
	}
	err := Trace()(next)(context.Background(), nil, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, captured, "trace_id should be auto-generated")
}

func TestTrace_PreservesExisting(t *testing.T) {
	ctx := metainfo.WithPersistentValue(context.Background(), mw.MetaKeyTraceID, "tid-123")
	var captured string
	next := func(ctx context.Context, _, _ any) error {
		captured, _ = metainfo.GetPersistentValue(ctx, mw.MetaKeyTraceID)
		return nil
	}
	err := Trace()(next)(ctx, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "tid-123", captured)
}

func TestTrace_ExtractsTraceparentAndStartsServerSpan(t *testing.T) {
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

	parentTraceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	parentSpanID := "00f067aa0ba902b7"
	ctx := metainfo.WithPersistentValue(context.Background(), "traceparent", "00-"+parentTraceID+"-"+parentSpanID+"-01")
	next := func(ctx context.Context, _, _ any) error {
		sc := trace.SpanContextFromContext(ctx)
		assert.Equal(t, parentTraceID, sc.TraceID().String())
		return nil
	}

	err := Trace()(next)(ctx, nil, nil)
	require.NoError(t, err)

	spans := sr.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, "unknown", spans[0].Name())
	assert.Equal(t, trace.SpanKindServer, spans[0].SpanKind())
	assert.Equal(t, parentTraceID, spans[0].SpanContext().TraceID().String())
	assert.Equal(t, parentSpanID, spans[0].Parent().SpanID().String())
}

func TestClientTrace_StartsClientSpanAndInjectsTraceparent(t *testing.T) {
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

	next := func(ctx context.Context, _, _ any) error {
		tpHeader, ok := metainfo.GetPersistentValue(ctx, "traceparent")
		assert.True(t, ok)
		assert.NotEmpty(t, tpHeader)
		return nil
	}

	err := ClientTrace()(next)(context.Background(), nil, nil)
	require.NoError(t, err)

	spans := sr.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, "RPC unknown", spans[0].Name())
	assert.Equal(t, trace.SpanKindClient, spans[0].SpanKind())
}

func TestRecovery_CatchesPanic(t *testing.T) {
	next := func(ctx context.Context, _, _ any) error {
		panic("boom")
	}
	err := Recovery()(next)(context.Background(), nil, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errno.ErrInternal))
}

func TestRecovery_RecordsPanicOnCurrentSpan(t *testing.T) {
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

	next := func(ctx context.Context, _, _ any) error {
		panic("boom")
	}
	err := Trace()(Recovery()(next))(context.Background(), nil, nil)
	require.Error(t, err)

	spans := sr.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, "Error", spans[0].Status().Code.String())
	require.NotEmpty(t, spans[0].Events())
	assert.Equal(t, "exception", spans[0].Events()[0].Name)
}

func TestTrace_AddsErrnoCodeAttributeOnError(t *testing.T) {
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

	err := Trace()(func(context.Context, any, any) error {
		return errno.ErrInvalidParam
	})(context.Background(), nil, nil)
	require.Error(t, err)

	spans := sr.Ended()
	require.Len(t, spans, 1)
	attrs := spans[0].Attributes()
	var found bool
	for _, attr := range attrs {
		if string(attr.Key) == "error.code" {
			found = true
			assert.EqualValues(t, errno.ErrInvalidParam.Code, attr.Value.AsInt64())
		}
	}
	assert.True(t, found)
}

func TestRecovery_PassThroughSuccess(t *testing.T) {
	err := Recovery()(okEndpoint)(context.Background(), nil, nil)
	assert.NoError(t, err)
}

func TestLogging_DoesNotAlterError(t *testing.T) {
	custom := errno.ErrInvalidParam.WithMessage("bad arg")
	next := func(ctx context.Context, _, _ any) error { return custom }
	err := Logging()(next)(context.Background(), nil, nil)
	assert.True(t, errors.Is(err, errno.ErrInvalidParam))
}

func TestErrnoCode(t *testing.T) {
	assert.EqualValues(t, 0, errnoCode(nil))
	assert.Equal(t, errno.ErrInvalidParam.Code, errnoCode(errno.ErrInvalidParam))
	assert.Equal(t, errno.ErrInternal.Code, errnoCode(errors.New("raw")))
}

func TestMethodFromCtx_Unknown(t *testing.T) {
	assert.Equal(t, "unknown", methodFromCtx(context.Background()))
}
