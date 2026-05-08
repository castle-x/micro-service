// Package utils 提供项目级通用工具函数集合，按文件职责拆分：
//
//	time.go    —— 时间戳与 Duration 辅助
//	id.go      —— UUID / trace_id 生成
//	crypto.go  —— 密码散列、随机字节
//	json.go    —— JSON 序列化 / 稳定字符串
//	convert.go —— 基础类型字符串转换
//	slice.go   —— 泛型切片工具
//	net.go     —— 本机网络 / 主机信息
//	file.go    —— 文件与目录辅助 / JSON / YAML 读写
//	context.go —— Context 检查辅助
//
// 约定：
//   - 所有返回 Unix 时间戳的函数一律返回 UTC 秒，禁止强绑时区。
//   - 包内函数不依赖项目其他 pkg 子包，保持零内部依赖以避免循环。
package utils
