// Package model 定义 idp 的 MongoDB 文档模型。
package model

import (
	"github.com/castlexu/micro-service/pkg/db"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// IdentityCollection 是 idp identities 集合名。
// 每条记录映射 (provider, provider_sub) → iam user_id。
const IdentityCollection = "identities"

// Identity 描述第三方身份与本地用户的映射关系。
type Identity struct {
	db.BaseDoc  `bson:",inline"`
	Provider    string             `bson:"provider"`     // "google"
	ProviderSub string             `bson:"provider_sub"` // Google sub 字段
	UserID      primitive.ObjectID `bson:"user_id"`      // iam 侧 user_id
	Email       string             `bson:"email"`
}

// OAuthStateCollection 是 oauth_states 集合名（防 CSRF state 临时存储）。
const OAuthStateCollection = "oauth_states"

// OAuthState 存储 Google OAuth2 防 CSRF state，TTL 由 MongoDB TTL 索引控制（10 分钟）。
type OAuthState struct {
	db.BaseDoc  `bson:",inline"`
	State       string `bson:"state"`
	RedirectURI string `bson:"redirect_uri,omitempty"`
}
