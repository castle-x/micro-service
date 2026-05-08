package db

import (
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"
)

// Error 是 db 包对外统一的错误类型，支持 errors.Is/As 解包底层原因。
type Error struct {
	msg   string
	cause error
}

// NewError 构造一个 db 错误。
func NewError(msg string, cause error) *Error {
	return &Error{msg: msg, cause: cause}
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.msg, e.cause)
	}
	return e.msg
}

func (e *Error) Unwrap() error { return e.cause }

// errorf 便捷构造：wrap 一个底层错误并附加上下文。
func errorf(cause error, format string, args ...any) error {
	return &Error{msg: fmt.Sprintf(format, args...), cause: cause}
}

// ---- 错误判定 ----

// IsDuplicateKey 判断 err 是否为 MongoDB 唯一键冲突错误（E11000）。
// 优先使用 mongo.IsDuplicateKeyError；可识别被 %w wrap 的错误链。
func IsDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	return mongo.IsDuplicateKeyError(err)
}

// IsNotFound 判断 err 是否代表 "no documents"。
// 使用 errors.Is 以兼容 fmt.Errorf("...: %w", mongo.ErrNoDocuments) 的包装链。
func IsNotFound(err error) bool {
	return errors.Is(err, mongo.ErrNoDocuments)
}
