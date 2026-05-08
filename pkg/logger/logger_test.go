package logger

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// newObservedLogger 构造一个绑定 observer 的全局 Logger，用于断言产出字段。
func newObservedLogger(t *testing.T, lvl zapcore.Level) *observer.ObservedLogs {
	t.Helper()
	core, recorded := observer.New(lvl)
	zl := zap.New(core)
	gLogger.Store(zl)
	t.Cleanup(func() { _ = Init(Options{}) })
	return recorded
}

func TestInit_DefaultLevelIsInfo(t *testing.T) {
	assert.NoError(t, Init(Options{}))
	// debug 应被过滤，info 应保留
	recorded := newObservedLogger(t, zapcore.InfoLevel)
	L().Debug("should be dropped")
	L().Info("should be kept")
	assert.Equal(t, 1, recorded.Len())
	assert.Equal(t, "should be kept", recorded.All()[0].Message)
}

func TestInit_InvalidLevelReturnsError(t *testing.T) {
	err := Init(Options{Level: "nosuchlevel"})
	assert.Error(t, err)
}

func TestCtx_InjectsAllMetaFields(t *testing.T) {
	recorded := newObservedLogger(t, zapcore.DebugLevel)

	ctx := context.Background()
	ctx = WithTraceID(ctx, "trace-xyz")
	ctx = WithCaller(ctx, "edge-api")
	ctx = WithUserID(ctx, "u-123")
	ctx = WithTenantID(ctx, "tn-7")

	Ctx(ctx).Info("hello", zap.String("k", "v"))

	entries := recorded.All()
	if !assert.Len(t, entries, 1) {
		return
	}
	fm := entries[0].ContextMap()
	assert.Equal(t, "trace-xyz", fm["trace_id"])
	assert.Equal(t, "edge-api", fm["caller"])
	assert.Equal(t, "u-123", fm["user_id"])
	assert.Equal(t, "tn-7", fm["tenant_id"])
	assert.Equal(t, "v", fm["k"])
}

func TestCtx_NilCtxEqualsGlobal(t *testing.T) {
	recorded := newObservedLogger(t, zapcore.DebugLevel)
	Ctx(nil).Info("no-ctx")
	entries := recorded.All()
	if !assert.Len(t, entries, 1) {
		return
	}
	_, hasTrace := entries[0].ContextMap()["trace_id"]
	assert.False(t, hasTrace, "nil ctx should not inject trace_id")
}

func TestMetaInfoExtractor_OverridesCtxKeys(t *testing.T) {
	recorded := newObservedLogger(t, zapcore.DebugLevel)

	SetMetaInfoExtractor(func(ctx context.Context) (trace, caller, user, tenant string) {
		return "from-extractor", "", "", ""
	})
	t.Cleanup(func() { SetMetaInfoExtractor(nil) })

	ctx := WithTraceID(context.Background(), "from-ctx")
	Ctx(ctx).Info("check")

	entries := recorded.All()
	if !assert.Len(t, entries, 1) {
		return
	}
	// extractor 给出的 trace 优先
	assert.Equal(t, "from-extractor", entries[0].ContextMap()["trace_id"])
}

func TestPrintfCompatAPIs(t *testing.T) {
	recorded := newObservedLogger(t, zapcore.DebugLevel)
	l := Ctx(context.Background())
	l.Debugf("d-%d", 1)
	l.Infof("i-%d", 2)
	l.Warnf("w-%d", 3)
	l.Errorf("e-%d", 4)
	msgs := make([]string, 0, recorded.Len())
	for _, e := range recorded.All() {
		msgs = append(msgs, e.Message)
	}
	assert.Equal(t, []string{"d-1", "i-2", "w-3", "e-4"}, msgs)
}
