package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNowUnix_IsUTCSeconds(t *testing.T) {
	got := NowUnix()
	diff := time.Now().Unix() - got
	assert.LessOrEqual(t, diff, int64(1))
}

func TestParseDuration(t *testing.T) {
	cases := map[string]time.Duration{
		"":      0,
		"7d":    7 * 24 * time.Hour,
		"1h30m": time.Hour + 30*time.Minute,
		"500ms": 500 * time.Millisecond,
	}
	for in, want := range cases {
		got, err := ParseDuration(in)
		assert.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}

	_, err := ParseDuration("abc")
	assert.Error(t, err)
}

func TestFormatDuration(t *testing.T) {
	d := time.Hour + 2*time.Minute + 3*time.Second
	assert.Equal(t, "1h2m3s", FormatDuration(d))
}

func TestNewIDUnique(t *testing.T) {
	set := make(map[string]struct{}, 1024)
	for i := 0; i < 1024; i++ {
		id := NewID()
		assert.NotEmpty(t, id)
		set[id] = struct{}{}
	}
	assert.Equal(t, 1024, len(set), "NewID should produce unique values")
}

func TestNewTraceIDNoDash(t *testing.T) {
	tid := NewTraceID()
	assert.NotContains(t, tid, "-")
	assert.NotEmpty(t, tid)
}

func TestHashVerifyPassword(t *testing.T) {
	hash, err := HashPassword("s3cret!")
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NoError(t, VerifyPassword(hash, "s3cret!"))
	assert.Error(t, VerifyPassword(hash, "wrong"))
}

func TestHashPassword_EmptyRejected(t *testing.T) {
	_, err := HashPassword("")
	assert.Error(t, err)
}

func TestRandomBytes(t *testing.T) {
	b, err := RandomBytes(16)
	assert.NoError(t, err)
	assert.Len(t, b, 16)

	_, err = RandomBytes(0)
	assert.Error(t, err)
}

func TestSliceDedup(t *testing.T) {
	assert.Equal(t, []int{1, 2, 3}, SliceDedup([]int{1, 2, 2, 3, 1}))
	assert.Equal(t, []string{"a", "b"}, SliceDedup([]string{"a", "a", "b", "b"}))
}

func TestSliceContains(t *testing.T) {
	assert.True(t, SliceContains([]int{1, 2, 3}, 2))
	assert.False(t, SliceContains([]int{1, 2, 3}, 9))
}

func TestSliceMapFilter(t *testing.T) {
	doubled := SliceMap([]int{1, 2, 3}, func(n int) int { return n * 2 })
	assert.Equal(t, []int{2, 4, 6}, doubled)

	evens := SliceFilter([]int{1, 2, 3, 4}, func(n int) bool { return n%2 == 0 })
	assert.Equal(t, []int{2, 4}, evens)
}

func TestJSONAndIndent(t *testing.T) {
	assert.Equal(t, `{"a":1}`, JSON(map[string]int{"a": 1}))
	assert.Contains(t, JSONIndent(map[string]int{"a": 1}), "    \"a\": 1")
}

func TestToStableString_MapOrderIndependent(t *testing.T) {
	a := map[string]any{"b": 2, "a": 1}
	b := map[string]any{"a": 1, "b": 2}
	assert.Equal(t, ToStableString(a), ToStableString(b))
	assert.Equal(t, `{"a":1,"b":2}`, ToStableString(a))
}

func TestTruncateString(t *testing.T) {
	assert.Equal(t, "abc", TruncateString("abc", 10))
	assert.Equal(t, "ab", TruncateString("abcdef", 2))
	out := TruncateString("abcdefghij", 7)
	assert.Contains(t, out, "...")
}

func TestSplitAndTrim(t *testing.T) {
	assert.Equal(t, []string{"a", "b", "c"}, SplitAndTrim("a, b, ,c ", ","))
}

func TestConvertAtoi(t *testing.T) {
	assert.Equal(t, 10, Atoi("10"))
	assert.Equal(t, int32(10), Atoi32("10"))
	assert.Equal(t, int64(10), Atoi64("10"))
	assert.Equal(t, 0, Atoi("bad"))
	assert.Equal(t, true, Atob("true"))
	assert.Equal(t, false, Atob("nope"))
	assert.Equal(t, "true", Btoa(true))
	assert.Equal(t, "10", Itoa(10))
}
