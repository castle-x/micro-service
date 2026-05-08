package utils

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// NowUnix 返回当前 Unix 时间戳（秒，UTC）。
// SPEC 要求所有 MongoDB 时间字段使用秒级时间戳，禁止 time.Time。
func NowUnix() int64 {
	return time.Now().Unix()
}

// NowUnixMilli 返回当前毫秒级 Unix 时间戳（UTC）。
func NowUnixMilli() int64 {
	return time.Now().UnixMilli()
}

// NowUnixNano 返回当前纳秒级 Unix 时间戳（UTC）。
func NowUnixNano() int64 {
	return time.Now().UnixNano()
}

// UnixToTime 将秒级时间戳转为 time.Time（UTC）。
func UnixToTime(sec int64) time.Time {
	return time.Unix(sec, 0).UTC()
}

// UnixMilliToTime 将毫秒级时间戳转为 time.Time（UTC）。
func UnixMilliToTime(ms int64) time.Time {
	return time.UnixMilli(ms).UTC()
}

// ParseDuration 在 time.ParseDuration 基础上额外支持 "Nd"（天）后缀。
// 解析失败返回 0（Duration 零值）与 error；通常业务可忽略 error 用零值兜底。
//
//	ParseDuration("7d")   -> 168h
//	ParseDuration("1h30m") -> 1h30m
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	// "Nd" 特殊处理
	if n := len(s); n > 1 && (s[n-1] == 'd' || s[n-1] == 'D') {
		days, err := strconv.Atoi(s[:n-1])
		if err == nil && days >= 0 {
			return time.Duration(days) * 24 * time.Hour, nil
		}
	}
	return time.ParseDuration(s)
}

// FormatDuration 格式化 Duration 为 "XhYmZs"。
func FormatDuration(d time.Duration) string {
	hours := d / time.Hour
	d -= hours * time.Hour
	minutes := d / time.Minute
	d -= minutes * time.Minute
	seconds := d / time.Second
	return fmt.Sprintf("%dh%dm%ds", hours, minutes, seconds)
}
