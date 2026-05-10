package model

import "github.com/castlexu/micro-service/pkg/db"

// PermissionCollection 是 permissions 集合名。
const PermissionCollection = "permissions"

// Permission 是权限文档，code 格式为 "resource:action"。
type Permission struct {
	db.BaseDoc  `bson:",inline"`
	Code        string `bson:"code"`         // 唯一键，如 "user:read"
	DisplayName string `bson:"display_name"` // 展示名
	Description string `bson:"description"`  // 描述
	IsSystem    bool   `bson:"is_system"`    // true = 内置权限，不可删除
}
