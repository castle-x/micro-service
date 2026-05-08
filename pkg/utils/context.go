package utils

import (
	"context"
	"errors"
)

// CheckContext 合并 ctx.Err() 与 context.Cause(ctx) 产出最终错误。
// 两者都为 nil 时返回 nil；二者都非 nil 时以 errors.Join 合并。
func CheckContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	cause := context.Cause(ctx)
	ctxErr := ctx.Err()
	switch {
	case cause == nil && ctxErr == nil:
		return nil
	case cause != nil && ctxErr != nil && !errors.Is(cause, ctxErr):
		return errors.Join(cause, ctxErr)
	case cause != nil:
		return cause
	default:
		return ctxErr
	}
}
