package errno

import (
	"errors"
	"fmt"

	"github.com/castlexu/micro-service/pkg/db"
)

// Errno 是项目统一错误类型：Code 用于机器判定，Message 面向人。
//
// 设计要点：
//   - 值类型而非指针：便于作为 package 级 var 全局常量化暴露（ErrXxx）。
//   - 实现 error 接口 + 自定义 Is：保证不同 Message 但 Code 相同的 Errno 可 errors.Is 命中。
//   - WithMessage / WithMessagef 返回新 Errno，不修改原全局变量。
type Errno struct {
	Code    int32
	Message string
}

// New 构造一个 Errno。一般用于 code.go 中定义预置错误码。
func New(code int32, msg string) Errno {
	return Errno{Code: code, Message: msg}
}

// Error 实现 error 接口。
func (e Errno) Error() string {
	return fmt.Sprintf("errno: %d, message: %s", e.Code, e.Message)
}

// Is 支持 errors.Is 按 Code 判定。两个 Errno 只要 Code 相等即视为相同错误，
// 以便业务层可以用预置的 errno.ErrOrderNotFound 匹配附带上下文信息的实例。
func (e Errno) Is(target error) bool {
	var t Errno
	if errors.As(target, &t) {
		return e.Code == t.Code
	}
	return false
}

// WithMessage 返回复制体并替换 Message。
func (e Errno) WithMessage(msg string) Errno {
	return Errno{Code: e.Code, Message: msg}
}

// WithMessagef 类似 WithMessage，但支持格式化参数。
func (e Errno) WithMessagef(format string, args ...any) Errno {
	return Errno{Code: e.Code, Message: fmt.Sprintf(format, args...)}
}

// FromDBError 将 pkg/db 的错误归一为 Errno：
//   - db.IsNotFound(err)    -> ErrNotFound（消息保留原 err）
//   - db.IsDuplicateKey(err) -> ErrDuplicateKey
//   - 其余                   -> ErrInternal 包装原始 err 字符串
//
// 若 err 本身已是 Errno，直接返回，避免重复包装。nil 透传。
func FromDBError(err error) error {
	if err == nil {
		return nil
	}
	var e Errno
	if errors.As(err, &e) {
		return e
	}
	switch {
	case db.IsNotFound(err):
		return ErrNotFound.WithMessagef("db: %v", err)
	case db.IsDuplicateKey(err):
		return ErrDuplicateKey.WithMessagef("db: %v", err)
	default:
		return ErrInternal.WithMessagef("db: %v", err)
	}
}
