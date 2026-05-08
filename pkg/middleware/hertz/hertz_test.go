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

	mw "github.com/castlexu/micro-service/pkg/middleware"
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
