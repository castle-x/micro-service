package utils

import (
	"strings"

	"github.com/google/uuid"
)

// NewID 生成一个全局唯一 ID（UUID v7，时间有序，适合作为订单号 / trace_id 的基础）。
// 若系统熵不足导致 v7 生成失败，会退化到 v4，保证可用性。
func NewID() string {
	if id, err := uuid.NewV7(); err == nil {
		return id.String()
	}
	return uuid.NewString()
}

// NewTraceID 生成无分隔符的 trace_id，方便日志检索。
func NewTraceID() string {
	return strings.ReplaceAll(NewID(), "-", "")
}
