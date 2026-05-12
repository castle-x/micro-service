package db

import "go.mongodb.org/mongo-driver/bson/primitive"

// BaseDocument 是所有 MongoDB 文档模型应实现的接口，驱动 Repository 的公共逻辑：
//   - 软删除判定 / 标记
//   - 自动维护 created_at / updated_at 时间戳
//
// 推荐通过内嵌 BaseDoc 一并获得字段与实现，业务模型只需扩展自己的字段。
type BaseDocument interface {
	GetID() primitive.ObjectID
	SetID(id primitive.ObjectID)

	GetCreatedAt() int64
	GetUpdatedAt() int64
	GetDeletedAt() *int64

	// SetTimestamps 同时写入 created_at 与 updated_at（InsertOne 时调用）。
	SetTimestamps(now int64)
	// Touch 只更新 updated_at（UpdateOne/Replace 时调用）。
	Touch(now int64)
	// SoftDelete 在 deleted_at 打上时间戳。
	SoftDelete(now int64)
	// IsDeleted 判断该文档是否已软删除。
	IsDeleted() bool
}

// BaseDoc 提供 BaseDocument 的默认嵌入实现，业务模型可这样使用：
//
//	type User struct {
//	    db.BaseDoc `bson:",inline"`
//	    Username   string `bson:"username"`
//	    Email      string `bson:"email"`
//	}
//
// 注意：必须在 bson tag 上加 ",inline"，否则 BaseDoc 会作为嵌套对象入库。
type BaseDoc struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	CreatedAt int64              `bson:"created_at" json:"created_at"`
	UpdatedAt int64              `bson:"updated_at" json:"updated_at"`
	DeletedAt *int64             `bson:"deleted_at,omitempty" json:"-"`
}

func (b *BaseDoc) GetID() primitive.ObjectID   { return b.ID }
func (b *BaseDoc) SetID(id primitive.ObjectID) { b.ID = id }
func (b *BaseDoc) GetCreatedAt() int64         { return b.CreatedAt }
func (b *BaseDoc) GetUpdatedAt() int64         { return b.UpdatedAt }
func (b *BaseDoc) GetDeletedAt() *int64        { return b.DeletedAt }
func (b *BaseDoc) SetTimestamps(now int64)     { b.CreatedAt = now; b.UpdatedAt = now }
func (b *BaseDoc) Touch(now int64)             { b.UpdatedAt = now }
func (b *BaseDoc) SoftDelete(now int64)        { b.DeletedAt = &now; b.UpdatedAt = now }
func (b *BaseDoc) IsDeleted() bool             { return b.DeletedAt != nil }
