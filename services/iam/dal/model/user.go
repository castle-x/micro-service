package model

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/db"
)

// UserCollection 是 iam users 集合名。
const UserCollection = "users"

// UserStatus 用户状态，与 IDL 对齐。
type UserStatus int32

const (
	UserStatusUnknown  UserStatus = 0
	UserStatusActive   UserStatus = 1
	UserStatusDisabled UserStatus = 2
	UserStatusBanned   UserStatus = 3
)

// User 是 iam 的用户主数据文档。
type User struct {
	db.BaseDoc `bson:",inline"`

	Email     string     `bson:"email"`
	Name      string     `bson:"name,omitempty"`
	AvatarURL string     `bson:"avatar_url,omitempty"`
	Status    UserStatus `bson:"status"`
}

// NewUser 构造一个新 User，ID 由 Repository.InsertOne 自动设置。
func NewUser(email, name, avatarURL string) *User {
	return &User{
		BaseDoc:   db.BaseDoc{ID: primitive.NewObjectID()},
		Email:     email,
		Name:      name,
		AvatarURL: avatarURL,
		Status:    UserStatusActive,
	}
}
