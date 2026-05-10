package model

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/db"
)

// RoleCollection 是 roles 集合名。
const RoleCollection = "roles"

// Role 是角色文档，存储角色名、展示名和权限 code 列表。
type Role struct {
	db.BaseDoc  `bson:",inline"`
	Name        string             `bson:"name"`         // 唯一键，如 "super_admin"
	DisplayName string             `bson:"display_name"` // 展示名，如 "超级管理员"
	Permissions []string           `bson:"permissions"`  // permission code 列表
	IsSystem    bool               `bson:"is_system"`    // true = 内置角色，不可删除
	CreatedBy   primitive.ObjectID `bson:"created_by,omitempty"`
}
