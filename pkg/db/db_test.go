package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestApplySoftDeleteFilter_NilFilter(t *testing.T) {
	got := applySoftDeleteFilter(nil)
	assert.Equal(t, bson.D{{Key: "deleted_at", Value: bson.D{{Key: "$exists", Value: false}}}}, got)
}

func TestApplySoftDeleteFilter_EmptyBsonD(t *testing.T) {
	got := applySoftDeleteFilter(bson.D{})
	assert.Equal(t, bson.D{{Key: "deleted_at", Value: bson.D{{Key: "$exists", Value: false}}}}, got)
}

func TestApplySoftDeleteFilter_WrapsWithAnd(t *testing.T) {
	in := bson.D{{Key: "username", Value: "alice"}}
	got := applySoftDeleteFilter(in)
	// 期望顶层是 { $and: [in, notDeleted] }
	assert.Len(t, got, 1)
	assert.Equal(t, "$and", got[0].Key)
	arr, ok := got[0].Value.(bson.A)
	assert.True(t, ok)
	assert.Len(t, arr, 2)
	assert.Equal(t, in, arr[0])
}

func TestApplySoftDeleteFilter_WorksWithBsonM(t *testing.T) {
	in := bson.M{"status": 1}
	got := applySoftDeleteFilter(in)
	assert.Equal(t, "$and", got[0].Key)
}

func TestInjectUpdatedAt_BsonDAddsSet(t *testing.T) {
	in := bson.D{{Key: "$inc", Value: bson.D{{Key: "count", Value: 1}}}}
	out := injectUpdatedAt(in, 1700000000).(bson.D)
	// 应追加 $set: {updated_at: 1700000000}
	var foundSet bool
	for _, e := range out {
		if e.Key == "$set" {
			foundSet = true
			sd, ok := e.Value.(bson.D)
			assert.True(t, ok)
			assert.True(t, containsKey(sd, "updated_at"))
		}
	}
	assert.True(t, foundSet)
}

func TestInjectUpdatedAt_ExistingSetNoOverride(t *testing.T) {
	in := bson.D{{Key: "$set", Value: bson.D{{Key: "updated_at", Value: int64(1)}}}}
	out := injectUpdatedAt(in, 1700000000).(bson.D)
	// 用户显式指定的 updated_at 不应被覆盖
	sd := out[0].Value.(bson.D)
	assert.Equal(t, int64(1), sd[0].Value)
}

func TestInjectUpdatedAt_BsonM(t *testing.T) {
	in := bson.M{"$set": bson.M{"name": "alice"}}
	out := injectUpdatedAt(in, 1700000000).(bson.M)
	set := out["$set"].(bson.M)
	assert.Equal(t, int64(1700000000), set["updated_at"])
	assert.Equal(t, "alice", set["name"])
}

func TestParseIndexSpec(t *testing.T) {
	cases := []struct {
		in    string
		name  string
		dir   any
		isErr bool
	}{
		{"user_id", "user_id", 1, false},
		{"created_at:-1", "created_at", -1, false},
		{"created_at:desc", "created_at", -1, false},
		{"location:2dsphere", "location", "2dsphere", false},
		{"hash_field:hashed", "hash_field", "hashed", false},
		{":1", "", nil, true},
	}
	for _, c := range cases {
		f, err := parseIndexSpec(c.in)
		if c.isErr {
			assert.Error(t, err, c.in)
			continue
		}
		assert.NoError(t, err, c.in)
		assert.Equal(t, c.name, f.Name, c.in)
		assert.Equal(t, c.dir, f.Direction, c.in)
	}
}

func TestIsPrefixMatch(t *testing.T) {
	// 已有索引 {a:1, b:1, c:-1}
	existing := bson.D{
		{Key: "a", Value: int32(1)},
		{Key: "b", Value: int32(1)},
		{Key: "c", Value: int32(-1)},
	}
	assert.True(t, isPrefixMatch(existing, []IndexField{{"a", 1}}))
	assert.True(t, isPrefixMatch(existing, []IndexField{{"a", 1}, {"b", 1}}))
	assert.True(t, isPrefixMatch(existing, []IndexField{{"a", 1}, {"b", 1}, {"c", -1}}))
	// 方向不一致不匹配
	assert.False(t, isPrefixMatch(existing, []IndexField{{"a", -1}}))
	// 字段顺序错误不匹配
	assert.False(t, isPrefixMatch(existing, []IndexField{{"b", 1}}))
	// 超过既有长度不匹配
	assert.False(t, isPrefixMatch(existing, []IndexField{{"a", 1}, {"b", 1}, {"c", -1}, {"d", 1}}))
}

func TestDocumentEmbeddingImplementsBaseDocument(t *testing.T) {
	type User struct {
		BaseDoc  `bson:",inline"`
		Username string `bson:"username"`
	}
	u := &User{Username: "alice"}
	// 通过 type assertion 验证 *User 实现 BaseDocument
	var _ BaseDocument = u

	u.SetTimestamps(1700000000)
	assert.Equal(t, int64(1700000000), u.CreatedAt)
	assert.Equal(t, int64(1700000000), u.UpdatedAt)
	assert.False(t, u.IsDeleted())

	u.SoftDelete(1800000000)
	assert.True(t, u.IsDeleted())
	assert.Equal(t, int64(1800000000), *u.GetDeletedAt())
	assert.Equal(t, int64(1800000000), u.UpdatedAt)
}

func TestErrorsIsNotFoundThroughWrap(t *testing.T) {
	e := errorf(mongoErrNoDocuments(), "db: find one")
	assert.True(t, IsNotFound(e), "wrapped ErrNoDocuments should still be detected")
}

// mongoErrNoDocuments 避免直接 import mongo 到测试文件里（保持最小依赖）。
func mongoErrNoDocuments() error {
	return wrappedNoDoc
}

// 通过一个 func 生成，避免 init 顺序问题
var wrappedNoDoc = func() error {
	// 从 errors.go 间接引用 driver 的 mongo.ErrNoDocuments
	return errNoDocumentsSentinel
}()
