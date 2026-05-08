package biz

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/errno"
	iammodel "github.com/castlexu/micro-service/services/iam/dal/model"
	iammongo "github.com/castlexu/micro-service/services/iam/dal/mongo"
)

// UserBiz 处理用户资料业务。
type UserBiz struct {
	userRepo *iammongo.UserRepo
}

// NewUserBiz 构造 UserBiz。
func NewUserBiz(userRepo *iammongo.UserRepo) *UserBiz {
	return &UserBiz{userRepo: userRepo}
}

// UpsertByProvider 幂等：email 已存在则更新资料；否则创建新用户。
// 返回 userID 和 created 标志。
func (b *UserBiz) UpsertByProvider(ctx context.Context, email, name, avatarURL string) (userID string, created bool, err error) {
	if email == "" {
		return "", false, errno.ErrInvalidParam.WithMessage("iam: email is required")
	}

	existing, err := b.userRepo.FindByEmail(ctx, email)
	if err != nil && !isNotFound(err) {
		return "", false, err
	}

	if existing != nil {
		// 更新资料
		if updateErr := b.userRepo.UpdateProfile(ctx, existing.ID, name, avatarURL); updateErr != nil {
			return "", false, updateErr
		}
		return existing.ID.Hex(), false, nil
	}

	// 新建
	u := iammodel.NewUser(email, name, avatarURL)
	id, insertErr := b.userRepo.Insert(ctx, u)
	if insertErr != nil {
		return "", false, insertErr
	}
	return id.Hex(), true, nil
}

// GetUser 按 userID 查询用户。
func (b *UserBiz) GetUser(ctx context.Context, userID string) (*iammodel.User, error) {
	oid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, errno.ErrInvalidParam.WithMessagef("iam: invalid user_id %q", userID)
	}
	return b.userRepo.FindByID(ctx, oid)
}

// isNotFound 判断是否是 ErrUserNotFound。
func isNotFound(err error) bool {
	return err != nil && err.Error() == errno.ErrUserNotFound.Error()
}
