// Package logger 提供基于 zap 的结构化日志封装。
//
// 设计目标：
//   - 默认 JSON 格式、输出到 stdout（容器化场景由平台收集，不做文件轮转）。
//   - 统一全局实例 L，同时支持 Ctx(ctx) 自动注入 trace_id / caller / user_id / tenant_id。
//   - 保留 Printf 式兼容 API（Infof 等），便于从旧日志库平滑迁移。
//
// 使用示例：
//
//	logger.Init(logger.Options{Service: "idp", Level: "info"})
//	logger.L().Info("service started", zap.Int("port", 8080))
//	logger.Ctx(ctx).Warn("login failed", zap.String("user", username))
//	logger.Ctx(ctx).Infof("order %s created", orderID)
package logger

import (
	"context"
	"os"
	"strings"
	"sync/atomic"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Options 控制日志初始化行为。零值可直接使用。
type Options struct {
	// Service 作为每条日志的固定字段 "service" 输出。为空则读取环境变量 SERVICE_NAME。
	Service string

	// Level 日志等级：debug / info / warn / error / fatal。大小写不敏感。
	// 为空则读取环境变量 LOG_LEVEL，再兜底为 info。
	Level string

	// Development 为 true 时使用开发者友好的 console encoder（彩色、短堆栈）；
	// false（默认）使用生产级 JSON。
	Development bool

	// DisableCaller 为 true 时关闭调用点文件:行号记录（默认开启）。
	DisableCaller bool

	// CallerSkip 额外调用栈深度。当业务在 logger 之外再包装一层 helper 时可通过此值校准。
	CallerSkip int
}

// 使用 atomic.Pointer 存储全局 logger，保证 Init 替换对 L() 读取方安全可见。
var gLogger atomic.Pointer[zap.Logger]

func init() {
	// 默认实例：读环境变量初始化一个 prod JSON logger，避免 main 未调 Init 时 nil panic。
	_ = Init(Options{})
}

// Init 根据 opts 构造并替换全局 logger。多次调用以最后一次为准。
// 返回 error 方便 main 做启动失败处理（例如等级解析失败）。
func Init(opts Options) error {
	lg, err := build(opts)
	if err != nil {
		return err
	}
	gLogger.Store(lg)
	return nil
}

// L 返回全局 logger。永远非 nil。
func L() *zap.Logger {
	return gLogger.Load()
}

// Sync 刷新底层 writer 缓冲，通常在进程退出前 defer 调用。
// stdout 在某些平台 Sync 会返回 EINVAL，此处吞错。
func Sync() {
	if lg := gLogger.Load(); lg != nil {
		_ = lg.Sync()
	}
}

// Ctx 返回带上下文元数据（trace_id / caller / user_id / tenant_id）的 Logger。
// ctx 为 nil 或未注入元数据时，等价于 {z: L()}。
func Ctx(ctx context.Context) *Logger {
	base := L()
	if ctx == nil {
		return &Logger{z: base}
	}
	traceID, caller, userID, tenantID := extractMeta(ctx)
	fields := make([]zap.Field, 0, 4)
	if traceID != "" {
		fields = append(fields, zap.String("trace_id", traceID))
	}
	if caller != "" {
		fields = append(fields, zap.String("caller", caller))
	}
	if userID != "" {
		fields = append(fields, zap.String("user_id", userID))
	}
	if tenantID != "" {
		fields = append(fields, zap.String("tenant_id", tenantID))
	}
	if len(fields) == 0 {
		return &Logger{z: base}
	}
	return &Logger{z: base.With(fields...)}
}

// Logger 是对 *zap.Logger 的薄封装，额外提供 Printf 式方法（Debugf/Infof/...）。
// 结构化 API（Info/Warn/Error 带 zap.Field）直接透传给底层 zap.Logger。
type Logger struct {
	z *zap.Logger
}

// Zap 返回底层 *zap.Logger，便于需要原生 API 时使用。
func (l *Logger) Zap() *zap.Logger { return l.z }

// With 追加固定字段，返回新的 Logger。
func (l *Logger) With(fields ...zap.Field) *Logger {
	return &Logger{z: l.z.With(fields...)}
}

// ---- 结构化 API（推荐） ----

func (l *Logger) Debug(msg string, fields ...zap.Field) { l.z.Debug(msg, fields...) }
func (l *Logger) Info(msg string, fields ...zap.Field)  { l.z.Info(msg, fields...) }
func (l *Logger) Warn(msg string, fields ...zap.Field)  { l.z.Warn(msg, fields...) }
func (l *Logger) Error(msg string, fields ...zap.Field) { l.z.Error(msg, fields...) }

// Fatal 记录后调用 os.Exit(1)。
func (l *Logger) Fatal(msg string, fields ...zap.Field) { l.z.Fatal(msg, fields...) }

// ---- Printf 兼容 API（迁移过渡用，推荐逐步改为结构化） ----

func (l *Logger) Debugf(format string, args ...any) { l.z.Sugar().Debugf(format, args...) }
func (l *Logger) Infof(format string, args ...any)  { l.z.Sugar().Infof(format, args...) }
func (l *Logger) Warnf(format string, args ...any)  { l.z.Sugar().Warnf(format, args...) }
func (l *Logger) Errorf(format string, args ...any) { l.z.Sugar().Errorf(format, args...) }
func (l *Logger) Fatalf(format string, args ...any) { l.z.Sugar().Fatalf(format, args...) }

// build 构造一个 zap.Logger。
func build(opts Options) (*zap.Logger, error) {
	service := opts.Service
	if service == "" {
		service = os.Getenv("SERVICE_NAME")
	}

	levelStr := opts.Level
	if levelStr == "" {
		levelStr = os.Getenv("LOG_LEVEL")
	}
	lvl, err := parseLevel(levelStr)
	if err != nil {
		return nil, err
	}

	var encCfg zapcore.EncoderConfig
	var encoder zapcore.Encoder
	if opts.Development {
		encCfg = zap.NewDevelopmentEncoderConfig()
		encCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encCfg)
	} else {
		encCfg = zap.NewProductionEncoderConfig()
		encCfg.TimeKey = "time"
		encCfg.MessageKey = "msg"
		encCfg.LevelKey = "level"
		encCfg.CallerKey = "caller_file"
		encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		encoder = zapcore.NewJSONEncoder(encCfg)
	}

	core := zapcore.NewCore(encoder, zapcore.Lock(os.Stdout), zap.NewAtomicLevelAt(lvl))

	zapOpts := []zap.Option{}
	if !opts.DisableCaller {
		zapOpts = append(zapOpts, zap.AddCaller(), zap.AddCallerSkip(opts.CallerSkip))
	}
	zapOpts = append(zapOpts, zap.AddStacktrace(zapcore.ErrorLevel))
	if service != "" {
		zapOpts = append(zapOpts, zap.Fields(zap.String("service", service)))
	}

	return zap.New(core, zapOpts...), nil
}

// parseLevel 解析日志等级字符串。
func parseLevel(s string) (zapcore.Level, error) {
	if s == "" {
		return zapcore.InfoLevel, nil
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn", "warning":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	case "fatal":
		return zapcore.FatalLevel, nil
	default:
		var lvl zapcore.Level
		if err := lvl.UnmarshalText([]byte(s)); err != nil {
			return zapcore.InfoLevel, err
		}
		return lvl, nil
	}
}
