package utils

import "strconv"

// 基础字符串 <-> 数值转换。解析失败统一返回零值，不返回 error；
// 业务校验请在上游完成。

// Atoi 字符串转 int，失败返回 0。
func Atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// Atoi32 字符串转 int32。
func Atoi32(s string) int32 {
	n, _ := strconv.ParseInt(s, 10, 32)
	return int32(n)
}

// Atoi64 字符串转 int64。
func Atoi64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// Atou32 字符串转 uint32。
func Atou32(s string) uint32 {
	n, _ := strconv.ParseUint(s, 10, 32)
	return uint32(n)
}

// Atou64 字符串转 uint64。
func Atou64(s string) uint64 {
	n, _ := strconv.ParseUint(s, 10, 64)
	return n
}

// Atof32 字符串转 float32。
func Atof32(s string) float32 {
	f, _ := strconv.ParseFloat(s, 32)
	return float32(f)
}

// Atof64 字符串转 float64。
func Atof64(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// Atob 字符串转 bool：仅识别 "true/false"（大小写不敏感），其它一律 false。
func Atob(s string) bool {
	b, _ := strconv.ParseBool(s)
	return b
}

// Btoa bool 转字符串。
func Btoa(b bool) string {
	return strconv.FormatBool(b)
}

// Itoa int 转字符串。
func Itoa(n int) string { return strconv.Itoa(n) }

// I32toa int32 转字符串。
func I32toa(n int32) string { return strconv.FormatInt(int64(n), 10) }

// I64toa int64 转字符串。
func I64toa(n int64) string { return strconv.FormatInt(n, 10) }
