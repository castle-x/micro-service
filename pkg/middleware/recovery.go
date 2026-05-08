// Package middleware 提供 Kitex / Hertz 通用中间件。
// TODO: implement per SPEC.md §9
package middleware

// Recovery 捕获 panic 并记录日志，避免进程崩溃。
// TODO: implement，适配 Kitex endpoint 与 Hertz app.HandlerFunc 两种签名。
func Recovery() interface{} {
	// TODO
	return nil
}
