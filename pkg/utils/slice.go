package utils

// SliceDedup 返回去重后的切片，保留首次出现顺序。
// 元素类型需可比较（comparable）。
func SliceDedup[T comparable](in []T) []T {
	if len(in) == 0 {
		return in
	}
	seen := make(map[T]struct{}, len(in))
	out := make([]T, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// SliceContains 判断 slice 中是否包含 item。
func SliceContains[T comparable](slice []T, item T) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

// SliceMap 对切片每个元素应用 fn 并收集结果。
func SliceMap[S any, D any](in []S, fn func(S) D) []D {
	if in == nil {
		return nil
	}
	out := make([]D, 0, len(in))
	for _, v := range in {
		out = append(out, fn(v))
	}
	return out
}

// SliceFilter 保留 pred 返回 true 的元素。
func SliceFilter[T any](in []T, pred func(T) bool) []T {
	out := make([]T, 0, len(in))
	for _, v := range in {
		if pred(v) {
			out = append(out, v)
		}
	}
	return out
}
