package mongo_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/errno"
	iammodel "github.com/castlexu/micro-service/services/iam/dal/model"
)

// 注意：需要真实 MongoDB 连接的集成测试留给 CI 环境（make dev 启动 Docker 后可运行）。
// 此文件只测试不依赖 DB 连接的逻辑。

func TestUserModel_NewUser_FieldsSet(t *testing.T) {
	u := iammodel.NewUser("alice@example.com", "Alice", "https://example.com/avatar.png")
	assert.Equal(t, "alice@example.com", u.Email)
	assert.Equal(t, "Alice", u.Name)
	assert.Equal(t, "https://example.com/avatar.png", u.AvatarURL)
	assert.Equal(t, iammodel.UserStatusActive, u.Status)
	assert.False(t, u.ID.IsZero())
}

func TestUserModel_NewUser_DefaultStatus(t *testing.T) {
	u := iammodel.NewUser("bob@example.com", "", "")
	assert.Equal(t, iammodel.UserStatusActive, u.Status)
	assert.False(t, u.ID.IsZero())
	assert.Empty(t, u.Name)
}

func TestUserModel_SoftDelete(t *testing.T) {
	u := iammodel.NewUser("del@example.com", "Del", "")
	assert.False(t, u.IsDeleted())
	u.SoftDelete(time.Now().Unix())
	assert.True(t, u.IsDeleted())
}

func TestUserModel_Touch(t *testing.T) {
	u := iammodel.NewUser("touch@example.com", "", "")
	u.SetTimestamps(1000)
	assert.Equal(t, int64(1000), u.CreatedAt)
	u.Touch(2000)
	assert.Equal(t, int64(2000), u.UpdatedAt)
	assert.Equal(t, int64(1000), u.CreatedAt)
}

func TestErrnoUserNotFound(t *testing.T) {
	err := errno.ErrUserNotFound
	assert.Equal(t, int32(12001), err.Code)
	assert.Contains(t, err.Error(), "12001")
}

func TestObjectIDParsing(t *testing.T) {
	id := primitive.NewObjectID()
	hex := id.Hex()
	parsed, err := primitive.ObjectIDFromHex(hex)
	require.NoError(t, err)
	assert.Equal(t, id, parsed)
}

func TestObjectIDFromHex_Invalid(t *testing.T) {
	_, err := primitive.ObjectIDFromHex("not-valid")
	assert.Error(t, err)
}
