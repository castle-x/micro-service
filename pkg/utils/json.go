package utils

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
)

// JSON 将 v 序列化为紧凑 JSON 字符串。失败返回 "null"。
// 适合日志、调试输出；性能敏感场景请直接调 encoding/json。
func JSON(v any) string {
	buf, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(buf)
}

// JSONIndent 将 v 序列化为带 4 空格缩进的易读 JSON。
func JSONIndent(v any) string {
	buf, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		return "null"
	}
	return string(buf)
}

// StructToMap 经 JSON 中转将结构体转为 map[string]any，用于需要动态字段的场景。
// 仅保留 json tag 可见字段；private 字段自动过滤。
func StructToMap(v any) map[string]any {
	buf, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(buf, &m); err != nil {
		return nil
	}
	return m
}

// ToStableString 将任意 v 转为"与排列无关"的稳定字符串，主要用于：
//   - 生成缓存 Key
//   - 计算签名 / 幂等键
//
// map 会按 key 字典序排序后序列化；基本类型直接 fmt.Sprint。
func ToStableString(v any) string {
	if v == nil {
		return "null"
	}
	switch val := v.(type) {
	case string:
		return val
	case map[string]any:
		return sortedMapToJSON(val)
	case map[string]string:
		conv := make(map[string]any, len(val))
		for k, v := range val {
			conv[k] = v
		}
		return sortedMapToJSON(conv)
	default:
		// 对于结构体/切片，encoding/json 对 map key 会自动排序，其它字段按源顺序——
		// 多数业务场景够稳定；如果需要更强保证可扩展反射遍历。
		buf, err := json.Marshal(v)
		if err != nil {
			return JSON(v)
		}
		return string(buf)
	}
}

func sortedMapToJSON(m map[string]any) string {
	if len(m) == 0 {
		return "{}"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kj, _ := json.Marshal(k)
		buf.Write(kj)
		buf.WriteByte(':')
		// 递归使用 ToStableString 处理嵌套 map
		if nested, ok := m[k].(map[string]any); ok {
			buf.WriteString(sortedMapToJSON(nested))
		} else {
			vj, err := json.Marshal(m[k])
			if err != nil {
				vj = []byte("null")
			}
			buf.Write(vj)
		}
	}
	buf.WriteByte('}')
	return buf.String()
}

// TruncateString 超过 maxLen 时截断中间，使用 " ... " 连接首尾，便于日志查看。
// maxLen <= 5 时直接按位截断。
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 5 {
		return s[:maxLen]
	}
	half := (maxLen - 5) / 2
	return s[:half] + " ... " + s[len(s)-half:]
}

// SplitAndTrim 以 sep 切分后去除空白并剔除空串。
func SplitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
