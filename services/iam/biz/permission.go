package biz

import (
	"context"
	"regexp"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/db"
	"github.com/castlexu/micro-service/pkg/errno"
	iammodel "github.com/castlexu/micro-service/services/iam/dal/model"
	iammongo "github.com/castlexu/micro-service/services/iam/dal/mongo"
)

var permCodeRe = regexp.MustCompile(`^[a-z][a-z0-9_]*(?::[a-z][a-z0-9_]*)+$`)

// PermissionBiz 处理权限管理业务。
type PermissionBiz struct {
	permRepo *iammongo.PermissionRepo
}

// NewPermissionBiz 构造 PermissionBiz。
func NewPermissionBiz(permRepo *iammongo.PermissionRepo) *PermissionBiz {
	return &PermissionBiz{permRepo: permRepo}
}

// ListPermissions 返回所有权限。
func (b *PermissionBiz) ListPermissions(ctx context.Context) ([]*iammodel.Permission, error) {
	return b.permRepo.ListAll(ctx)
}

// CreatePermission 创建自定义权限，code 格式必须为 "resource:action"。
func (b *PermissionBiz) CreatePermission(ctx context.Context, code, displayName, description string) (*iammodel.Permission, error) {
	if !permCodeRe.MatchString(code) {
		return nil, errno.ErrInvalidParam.WithMessage("iam: permission code must match resource:action format (lowercase, colon-separated)")
	}
	if displayName == "" {
		return nil, errno.ErrInvalidParam.WithMessage("iam: display_name is required")
	}

	p := &iammodel.Permission{
		BaseDoc:     db.BaseDoc{ID: primitive.NewObjectID()},
		Code:        code,
		DisplayName: displayName,
		Description: description,
		IsSystem:    false,
	}
	if err := b.permRepo.Insert(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}
