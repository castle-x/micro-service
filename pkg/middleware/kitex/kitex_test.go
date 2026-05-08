package kitex

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestRecovery_CatchesPanic(t *testing.T) {
	next := func(ctx context.Context, _, _ any) error {
		panic("boom")
	}
	err := Recovery()(next)(context.Background(), nil, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errno.ErrInternal))
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
