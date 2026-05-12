package hertz

import (
	"context"
	"net/http"
	"testing"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	mw "github.com/castlexu/micro-service/pkg/middleware"
	mwkitex "github.com/castlexu/micro-service/pkg/middleware/kitex"
)

func TestTrace_GeneratesWhenMissing(t *testing.T) {
	ctx := ut.CreateUtRequestContext(http.MethodGet, "/v1/x", nil)
	var captured string

	// 组合 Trace + 下游 handler
	h := func(c context.Context, rc *app.RequestContext) {
		captured, _ = metainfo.GetPersistentValue(c, mw.MetaKeyTraceID)
	}
	// 模拟 middleware chain：先 Trace 注入，再跑 handler
	ctx.SetHandlers([]app.HandlerFunc{Trace(), h})
	ctx.Next(context.Background())

	assert.NotEmpty(t, captured, "trace_id should be generated")
	// response header 应已写入
	assert.NotEmpty(t, string(ctx.Response.Header.Get(HeaderTraceID)))
}

func TestTrace_PreservesHeader(t *testing.T) {
	ctx := ut.CreateUtRequestContext(http.MethodGet, "/v1/x", nil,
		ut.Header{Key: HeaderTraceID, Value: "tid-abc"},
		ut.Header{Key: HeaderUserID, Value: "u-1"},
	)

	var tid, uid string
	h := func(c context.Context, rc *app.RequestContext) {
		tid, _ = metainfo.GetPersistentValue(c, mw.MetaKeyTraceID)
		uid, _ = metainfo.GetPersistentValue(c, mw.MetaKeyUserID)
	}
	ctx.SetHandlers([]app.HandlerFunc{Trace(), h})
	ctx.Next(context.Background())

	assert.Equal(t, "tid-abc", tid)
	assert.Equal(t, "u-1", uid)
	assert.Equal(t, "tid-abc", string(ctx.Response.Header.Get(HeaderTraceID)))
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
	ctx := ut.CreateUtRequestContext(http.MethodGet, "/v1/users/42", nil,
		ut.Header{Key: "traceparent", Value: "00-" + parentTraceID + "-" + parentSpanID + "-01"},
	)
	ctx.SetFullPath("/v1/users/:id")
	ctx.SetHandlers([]app.HandlerFunc{Trace(), func(c context.Context, rc *app.RequestContext) {
		sc := trace.SpanContextFromContext(c)
		assert.Equal(t, parentTraceID, sc.TraceID().String())
		assert.Equal(t, parentTraceID, string(rc.Response.Header.Get(HeaderTraceID)))
	}})

	ctx.Next(context.Background())

	spans := sr.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, "HTTP GET /v1/users/:id", spans[0].Name())
	assert.Equal(t, trace.SpanKindServer, spans[0].SpanKind())
	assert.Equal(t, parentTraceID, spans[0].SpanContext().TraceID().String())
	assert.Equal(t, parentSpanID, spans[0].Parent().SpanID().String())
}

func TestTrace_SpanTreePropagatesToKitexClientAndServer(t *testing.T) {
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

	ctx := ut.CreateUtRequestContext(http.MethodGet, "/api/v1/user/me", nil)
	ctx.SetFullPath("/api/v1/user/me")
	ctx.SetHandlers([]app.HandlerFunc{Trace(), func(c context.Context, rc *app.RequestContext) {
		err := mwkitex.ClientTrace()(func(c context.Context, _, _ any) error {
			return mwkitex.Trace()(func(context.Context, any, any) error {
				return nil
			})(c, nil, nil)
		})(c, nil, nil)
		require.NoError(t, err)
	}})

	ctx.Next(context.Background())

	spans := sr.Ended()
	require.Len(t, spans, 3)

	byName := make(map[string]sdktrace.ReadOnlySpan, len(spans))
	for _, span := range spans {
		byName[span.Name()] = span
	}
	httpSpan := byName["HTTP GET /api/v1/user/me"]
	clientSpan := byName["RPC unknown"]
	serverSpan := byName["unknown"]
	require.NotNil(t, httpSpan)
	require.NotNil(t, clientSpan)
	require.NotNil(t, serverSpan)

	traceID := httpSpan.SpanContext().TraceID()
	assert.Equal(t, traceID, clientSpan.SpanContext().TraceID())
	assert.Equal(t, traceID, serverSpan.SpanContext().TraceID())
	assert.Equal(t, httpSpan.SpanContext().SpanID(), clientSpan.Parent().SpanID())
	assert.Equal(t, clientSpan.SpanContext().SpanID(), serverSpan.Parent().SpanID())
}

func TestRecovery_CatchesPanic(t *testing.T) {
	ctx := ut.CreateUtRequestContext(http.MethodGet, "/v1/x", nil)
	h := func(c context.Context, rc *app.RequestContext) {
		panic("boom")
	}
	ctx.SetHandlers([]app.HandlerFunc{Recovery(), h})
	// 不应 panic 传播出来
	require.NotPanics(t, func() {
		ctx.Next(context.Background())
	})
	assert.Equal(t, 500, ctx.Response.StatusCode())
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

	ctx := ut.CreateUtRequestContext(http.MethodGet, "/v1/x", nil)
	h := func(c context.Context, rc *app.RequestContext) {
		panic("boom")
	}
	ctx.SetHandlers([]app.HandlerFunc{Trace(), Recovery(), h})
	require.NotPanics(t, func() {
		ctx.Next(context.Background())
	})

	spans := sr.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, 500, ctx.Response.StatusCode())
	assert.Equal(t, "Error", spans[0].Status().Code.String())
	require.NotEmpty(t, spans[0].Events())
	assert.Equal(t, "exception", spans[0].Events()[0].Name)
}

func TestLogging_PassThrough(t *testing.T) {
	ctx := ut.CreateUtRequestContext(http.MethodGet, "/v1/ok", nil)
	h := func(c context.Context, rc *app.RequestContext) {
		rc.SetStatusCode(200)
	}
	ctx.SetHandlers([]app.HandlerFunc{Logging(), h})
	require.NotPanics(t, func() {
		ctx.Next(context.Background())
	})
	assert.Equal(t, 200, ctx.Response.StatusCode())
}
