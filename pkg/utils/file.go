package utils

import (
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// FileExists 判断路径指向的文件是否存在（目录返回 false）。
func FileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

// DirExists 判断路径指向的目录是否存在。
func DirExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// MkdirAll 等价 os.MkdirAll，默认权限 0755。
func MkdirAll(path string) error {
	return os.MkdirAll(path, 0o755)
}

// LoadJSONFile 读取并解码 JSON 到 out（须为指针）。
func LoadJSONFile(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("utils: read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("utils: decode json %s: %w", path, err)
	}
	return nil
}

// SaveJSONFile 将 v 以缩进 JSON 写入 path。
func SaveJSONFile(path string, v any) error {
	buf, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		return fmt.Errorf("utils: encode json: %w", err)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		return fmt.Errorf("utils: write %s: %w", path, err)
	}
	return nil
}

// LoadYAMLFile 读取并解码 YAML 到 out（须为指针）。
func LoadYAMLFile(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("utils: read %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, out); err != nil {
		return fmt.Errorf("utils: decode yaml %s: %w", path, err)
	}
	return nil
}

// SaveYAMLFile 将 v 编码为 YAML 写入 path。
func SaveYAMLFile(path string, v any) error {
	buf, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("utils: encode yaml: %w", err)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		return fmt.Errorf("utils: write %s: %w", path, err)
	}
	return nil
}
