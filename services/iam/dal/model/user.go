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

// UserSource 用户来源。
type UserSource int32

const (
	UserSourceUnknown      UserSource = 0
	UserSourcePassword     UserSource = 1
	UserSourceGoogle       UserSource = 2
	UserSourceAlipay       UserSource = 3
	UserSourcePhone        UserSource = 4
	UserSourceAdminCreated UserSource = 5
)

// User 是 iam 的用户主数据文档。
type User struct {
	db.BaseDoc `bson:",inline"`

	// 身份
	Email    string `bson:"email"`
	Phone    string `bson:"phone,omitempty"`
	Username string `bson:"username,omitempty"`

	// 档案
	Name      string `bson:"name,omitempty"`
	AvatarURL string `bson:"avatar_url,omitempty"`
	Bio       string `bson:"bio,omitempty"`
	Locale    string `bson:"locale,omitempty"`
	Timezone  string `bson:"timezone,omitempty"`

	// 状态
	Status        UserStatus `bson:"status"`
	EmailVerified bool       `bson:"email_verified"`
	PhoneVerified bool       `bson:"phone_verified"`

	// 安全
	MFAEnabled          bool  `bson:"mfa_enabled"`
	FailedLoginAttempts int   `bson:"failed_login_attempts"`
	LockedUntil         int64 `bson:"locked_until,omitempty"` // Unix 秒，0=未锁定

	// 权限：单角色，存角色名（指向 roles.name）
	Role string `bson:"role"`

	// 溯源
	Source    UserSource         `bson:"source"`
	CreatedBy primitive.ObjectID `bson:"created_by,omitempty"` // 管理员创建时记录操作者

	// 时间戳
	LastLoginAt int64 `bson:"last_login_at,omitempty"`
}

// NewUser 构造一个新 User。
func NewUser(email, name, avatarURL string) *User {
	return &User{
		BaseDoc:   db.BaseDoc{ID: primitive.NewObjectID()},
		Email:     email,
		Name:      name,
		AvatarURL: avatarURL,
		Status:    UserStatusActive,
		Role:      "user",
		Source:    UserSourcePassword,
	}
}
