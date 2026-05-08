// Package config 提供统一的服务配置加载能力。
//
// 设计目标（见 SPEC.md §10）：
//   - yaml 文件承载非敏感配置；
//   - 密钥 / Secret 强制走环境变量，yaml 中可用 ${VAR} 占位引用；
//   - 环境变量可直接覆盖任意配置项（key 中的 . 替换为 _，并统一大写）；
//   - 典型用法：
//
//	type IdpConfig struct {
//	    Mongo struct{ URI, DB string } `mapstructure:"mongo"`
//	    JWT   struct{ Secret string; AccessTTL int } `mapstructure:"jwt"`
//	}
//	var cfg IdpConfig
//	if err := config.Load("deployments/config/idp.yaml", &cfg); err != nil { ... }
//
// 加载顺序：
//  1. 读取 yaml 文件（允许不存在，此时仅使用环境变量与默认值）；
//  2. 对所有字符串值执行 os.ExpandEnv，实现 ${VAR} 展开；
//  3. 环境变量 AutomaticEnv 覆盖（同键名，大写，点号替换为下划线）。
package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/spf13/viper"

	"github.com/castlexu/micro-service/pkg/errno"
)

// Load 从 path 读取 yaml 并反序列化到 out。
//
//   - path 为空或文件不存在时：不报错，仅应用 env 覆盖与默认零值；
//   - out 必须是指向结构体的非 nil 指针；
//   - 结构体字段通过 `mapstructure:"xxx"` 标签与 yaml key 关联（viper 默认）。
func Load[T any](path string, out *T) error {
	if out == nil {
		return errno.ErrInvalidParam.WithMessage("config.Load: out is nil")
	}

	v := viper.New()
	v.SetConfigType("yaml")

	// 环境变量自动覆盖：LOG_LEVEL 覆盖 log.level，REDIS_ADDR 覆盖 redis.addr。
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 扫描 out 的 mapstructure tag，显式 BindEnv，让 env-only（无 yaml 文件）
	// 场景下 Unmarshal 也能读到环境变量。
	for _, key := range collectKeys(reflect.TypeOf(*out), "") {
		_ = v.BindEnv(key)
	}

	if path != "" {
		info, err := os.Stat(path)
		switch {
		case err == nil && !info.IsDir():
			raw, rerr := os.ReadFile(path)
			if rerr != nil {
				return errno.ErrInternal.WithMessagef("config.Load: read %s: %v", path, rerr)
			}
			// ${VAR} 展开：允许 yaml 引用环境变量（未设置则替换为空串，与 os.ExpandEnv 一致）。
			expanded := os.ExpandEnv(string(raw))
			if err := v.ReadConfig(strings.NewReader(expanded)); err != nil {
				return errno.ErrInternal.WithMessagef("config.Load: parse %s: %v", path, err)
			}
		case err != nil && !errors.Is(err, os.ErrNotExist):
			return errno.ErrInternal.WithMessagef("config.Load: stat %s: %v", path, err)
		}
	}

	if err := v.Unmarshal(out); err != nil {
		return errno.ErrInternal.WithMessagef("config.Load: unmarshal: %v", err)
	}
	return nil
}

// MustLoad 封装 Load，失败时 panic。仅在服务启动阶段使用。
func MustLoad[T any](path string, out *T) {
	if err := Load(path, out); err != nil {
		panic(fmt.Sprintf("config.MustLoad: %v", err))
	}
}

// RequireEnv 检查一组环境变量必须全部非空，否则返回 ErrInvalidParam 并列出缺失项。
// 典型用于 main.go 校验 JWT_SECRET / MONGO_URI 等关键 secret 已就绪。
func RequireEnv(keys ...string) error {
	var missing []string
	for _, k := range keys {
		if os.Getenv(k) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return errno.ErrInvalidParam.WithMessagef("required env missing: %s", strings.Join(missing, ","))
	}
	return nil
}

// collectKeys 递归扫描 struct 的 mapstructure tag，生成点分隔的配置键列表。
// 用于在无 yaml 文件时显式 BindEnv，使 AutomaticEnv 生效。
// 字段无 mapstructure tag 时，使用小写字段名；嵌套匿名字段视为平铺。
func collectKeys(t reflect.Type, prefix string) []string {
	if t == nil {
		return nil
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	var keys []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("mapstructure")
		name := tag
		if name == "" {
			name = strings.ToLower(f.Name)
		}
		if name == "-" {
			continue
		}
		// 去掉 ",squash"/",omitempty" 等修饰。
		if comma := strings.Index(name, ","); comma >= 0 {
			name = name[:comma]
		}
		full := name
		if prefix != "" && name != "" {
			full = prefix + "." + name
		} else if name == "" { // squash
			full = prefix
		}

		ft := f.Type
		for ft.Kind() == reflect.Ptr {
			ft = ft.Elem()
		}
		if ft.Kind() == reflect.Struct {
			keys = append(keys, collectKeys(ft, full)...)
			continue
		}
		if full != "" {
			keys = append(keys, full)
		}
	}
	return keys
}
