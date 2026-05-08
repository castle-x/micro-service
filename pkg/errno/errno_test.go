package errno

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestErrno_Error(t *testing.T) {
	e := New(10001, "boom")
	assert.Equal(t, "errno: 10001, message: boom", e.Error())
}

func TestErrno_Is_ByCode(t *testing.T) {
	// 相同 Code，不同 Message，应互相 Is。
	a := ErrOrderNotFound.WithMessagef("order_id: %s", "o-1")
	b := ErrOrderNotFound
	assert.True(t, errors.Is(a, b), "same code should Is each other")
	assert.True(t, errors.Is(b, a))
}

func TestErrno_Is_DifferentCode(t *testing.T) {
	assert.False(t, errors.Is(ErrOrderNotFound, ErrUserNotFound))
}

func TestErrno_Is_NonErrno(t *testing.T) {
	assert.False(t, errors.Is(ErrInternal, errors.New("raw")))
}

func TestWithMessage(t *testing.T) {
	orig := ErrInvalidParam
	e := orig.WithMessage("amount must be positive")
	assert.Equal(t, orig.Code, e.Code)
	assert.Equal(t, "amount must be positive", e.Message)
	// 原实例不被修改。
	assert.Equal(t, "invalid parameter", orig.Message)
}

func TestWithMessagef(t *testing.T) {
	e := ErrUserNotFound.WithMessagef("uid=%s", "u-42")
	assert.Equal(t, ErrUserNotFound.Code, e.Code)
	assert.Equal(t, "uid=u-42", e.Message)
}

func TestFromDBError_Nil(t *testing.T) {
	assert.Nil(t, FromDBError(nil))
}

func TestFromDBError_NotFound(t *testing.T) {
	err := FromDBError(mongo.ErrNoDocuments)
	var e Errno
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, ErrNotFound.Code, e.Code)
}

func TestFromDBError_NotFound_Wrapped(t *testing.T) {
	wrapped := fmt.Errorf("load user failed: %w", mongo.ErrNoDocuments)
	err := FromDBError(wrapped)
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestFromDBError_Passthrough_Errno(t *testing.T) {
	// 已是 Errno，应原样返回。
	in := ErrForbidden.WithMessage("no perm")
	err := FromDBError(in)
	var e Errno
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, ErrForbidden.Code, e.Code)
	assert.Equal(t, "no perm", e.Message)
}

func TestFromDBError_Fallback_Internal(t *testing.T) {
	err := FromDBError(errors.New("unknown db boom"))
	assert.True(t, errors.Is(err, ErrInternal))
}

// 区段边界冒烟：每个区段至少一个码，避免忘记更新 code.go。
func TestCodeRanges(t *testing.T) {
	cases := map[string]int32{
		"sys-lower":          ErrInternal.Code,
		"sys-upper":          ErrDuplicateKey.Code,
		"idp":                ErrInvalidCredentials.Code,
		"iam":                ErrUserNotFound.Code,
		"billing":            ErrOrderNotFound.Code,
		"credits":            ErrInsufficientCredits.Code,
		"notification":       ErrTemplateNotFound.Code,
	}
	ranges := map[string][2]int32{
		"sys-lower":    {10001, 10999},
		"sys-upper":    {10001, 10999},
		"idp":          {11001, 11999},
		"iam":          {12001, 12999},
		"billing":      {13001, 13999},
		"credits":      {14001, 14999},
		"notification": {15001, 15999},
	}
	for name, code := range cases {
		r := ranges[name]
		assert.GreaterOrEqual(t, code, r[0], name)
		assert.LessOrEqual(t, code, r[1], name)
	}
}
