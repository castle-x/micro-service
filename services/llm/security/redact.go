// Package security contains defensive helpers for LLM request handling.
package security

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
)

const RedactedValue = "[REDACTED]"

var sensitiveKeyParts = []string{
	"password",
	"secret",
	"token",
	"authorization",
	"api_key",
}

var (
	authorizationPattern = regexp.MustCompile(`(?i)authorization\s*[:=]\s*(bearer\s+)?[^\s,;}\]]+`)
	bearerPattern        = regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._~+/=-]+`)
	sensitivePairPattern = regexp.MustCompile(`(?i)"?(password|secret|token|authorization|api_key|api-key|apikey)"?\s*[:=]\s*"?[^"\s,;}\]]+"?`)
	sensitiveWordPattern = regexp.MustCompile(`(?i)\b(password|secret|token|authorization|api_key|api-key|apikey)\b\s+"?[^"\s,;}\]]+"?`)
)

// Redact returns a copy-like representation with sensitive values replaced.
func Redact(v any) any {
	return redactValue(reflect.ValueOf(v), "")
}

// RedactJSONString redacts a JSON string. Malformed JSON is never echoed back.
func RedactJSONString(s string) string {
	if strings.TrimSpace(s) == "" {
		return s
	}
	out := RedactJSONBytes([]byte(s))
	return string(out)
}

// RedactText redacts sensitive material from plain error strings and summaries.
func RedactText(s string, secrets ...string) string {
	if strings.TrimSpace(s) == "" {
		return s
	}
	out := s
	if json.Valid([]byte(strings.TrimSpace(s))) {
		out = RedactJSONString(s)
	}
	for _, secret := range secrets {
		if strings.TrimSpace(secret) == "" {
			continue
		}
		out = strings.ReplaceAll(out, secret, RedactedValue)
	}
	out = authorizationPattern.ReplaceAllString(out, RedactedValue)
	out = bearerPattern.ReplaceAllString(out, RedactedValue)
	out = sensitivePairPattern.ReplaceAllString(out, RedactedValue)
	out = sensitiveWordPattern.ReplaceAllString(out, RedactedValue)
	return out
}

// RedactJSONBytes redacts JSON bytes. Malformed JSON returns only a marker.
func RedactJSONBytes(raw []byte) []byte {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return raw
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return []byte(RedactedValue)
	}
	out, err := json.Marshal(Redact(v))
	if err != nil {
		return []byte(RedactedValue)
	}
	return out
}

func redactValue(v reflect.Value, fieldName string) any {
	if isSensitiveName(fieldName) {
		return RedactedValue
	}
	if !v.IsValid() {
		return nil
	}
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Map:
		out := make(map[string]any, v.Len())
		iter := v.MapRange()
		for iter.Next() {
			key := keyString(iter.Key())
			if isSensitiveName(key) {
				out[key] = RedactedValue
				continue
			}
			out[key] = redactValue(iter.Value(), key)
		}
		return out
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.Type().Elem().Kind() == reflect.Uint8 {
			return string(RedactJSONBytes(v.Bytes()))
		}
		out := make([]any, 0, v.Len())
		for i := 0; i < v.Len(); i++ {
			out = append(out, redactValue(v.Index(i), ""))
		}
		return out
	case reflect.Struct:
		return redactStruct(v)
	default:
		if v.CanInterface() {
			return v.Interface()
		}
		return nil
	}
}

func redactStruct(v reflect.Value) map[string]any {
	t := v.Type()
	out := make(map[string]any, v.NumField())
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := jsonFieldName(field)
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		if isSensitiveName(name) || isSensitiveName(field.Name) {
			out[name] = RedactedValue
			continue
		}
		out[name] = redactValue(v.Field(i), name)
	}
	return out
}

func jsonFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "" {
		return ""
	}
	name, _, _ := strings.Cut(tag, ",")
	return name
}

func keyString(v reflect.Value) string {
	for v.Kind() == reflect.Interface {
		v = v.Elem()
	}
	if v.Kind() == reflect.String {
		return v.String()
	}
	if v.CanInterface() {
		return strings.TrimSpace(reflect.ValueOf(v.Interface()).String())
	}
	return ""
}

func isSensitiveName(name string) bool {
	lower := strings.ToLower(name)
	for _, part := range sensitiveKeyParts {
		if strings.Contains(lower, part) {
			return true
		}
	}
	return false
}
