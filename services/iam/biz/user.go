package biz

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/errno"
	iammodel "github.com/castlexu/micro-service/services/iam/dal/model"
	iammongo "github.com/castlexu/micro-service/services/iam/dal/mongo"
)

// UserBiz 处理用户资料业务。
type UserBiz struct {
	userRepo *iammongo.UserRepo
	roleRepo *iammongo.RoleRepo
}

// NewUserBiz 构造 UserBiz。
func NewUserBiz(userRepo *iammongo.UserRepo, roleRepo *iammongo.RoleRepo) *UserBiz {
	return &UserBiz{userRepo: userRepo, roleRepo: roleRepo}
}

// UpsertByProvider 幂等：email 已存在则更新资料；否则创建新用户。
func (b *UserBiz) UpsertByProvider(ctx context.Context, email, name, avatarURL string) (userID string, role string, status iammodel.UserStatus, created bool, err error) {
	if email == "" {
		return "", "", 0, false, errno.ErrInvalidParam.WithMessage("iam: email is required")
	}

	existing, findErr := b.userRepo.FindByEmail(ctx, email)
	if findErr != nil && !isNotFound(findErr) {
		return "", "", 0, false, findErr
	}

	if existing != nil {
		if updateErr := b.userRepo.UpdateProfile(ctx, existing.ID, name, avatarURL); updateErr != nil {
			return "", "", 0, false, updateErr
		}
		return existing.ID.Hex(), existing.Role, existing.Status, false, nil
	}

	u := iammodel.NewUser(email, name, avatarURL)
	id, insertErr := b.userRepo.Insert(ctx, u)
	if insertErr != nil {
		return "", "", 0, false, insertErr
	}
	return id.Hex(), u.Role, u.Status, true, nil
}

// GetUser 按 userID 查询用户。
func (b *UserBiz) GetUser(ctx context.Context, userID string) (*iammodel.User, error) {
	oid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, errno.ErrInvalidParam.WithMessagef("iam: invalid user_id %q", userID)
	}
	return b.userRepo.FindByID(ctx, oid)
}

// ListUsers 分页查询用户。
func (b *UserBiz) ListUsers(ctx context.Context, page, pageSize int, role string, status *iammodel.UserStatus) ([]*iammodel.User, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return b.userRepo.List(ctx, page, pageSize, role, status)
}

// UpdateUserRole 修改用户角色，操作者必须是 super_admin 或 admin。
func (b *UserBiz) UpdateUserRole(ctx context.Context, targetUserID, role, operatorUserID string) error {
	// 校验角色存在
	if _, err := b.roleRepo.FindByName(ctx, role); err != nil {
		return errno.ErrInvalidParam.WithMessagef("iam: role %q not found", role)
	}

	// 不允许降级 super_admin
	target, err := b.userRepo.FindByID(ctx, mustOID(targetUserID))
	if err != nil {
		return err
	}
	if target.Role == "super_admin" {
		return errno.ErrForbidden.WithMessage("iam: cannot change super_admin role")
	}

	oid, err := primitive.ObjectIDFromHex(targetUserID)
	if err != nil {
		return errno.ErrInvalidParam.WithMessagef("iam: invalid user_id: %v", err)
	}
	return b.userRepo.UpdateRole(ctx, oid, role)
}

// UpdateUserStatus 修改用户状态。
func (b *UserBiz) UpdateUserStatus(ctx context.Context, targetUserID string, status iammodel.UserStatus) error {
	target, err := b.userRepo.FindByID(ctx, mustOID(targetUserID))
	if err != nil {
		return err
	}
	if target.Role == "super_admin" {
		return errno.ErrForbidden.WithMessage("iam: cannot disable super_admin")
	}
	oid, _ := primitive.ObjectIDFromHex(targetUserID)
	return b.userRepo.UpdateStatus(ctx, oid, status)
}

func mustOID(s string) primitive.ObjectID {
	oid, _ := primitive.ObjectIDFromHex(s)
	return oid
}

func isNotFound(err error) bool {
	return err != nil && (fmt.Sprintf("%v", err) == errno.ErrUserNotFound.Error() ||
		fmt.Sprintf("%v", err) == errno.ErrNotFound.Error())
}
