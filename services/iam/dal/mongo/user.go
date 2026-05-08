package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/db"
	"github.com/castlexu/micro-service/pkg/errno"
	iammodel "github.com/castlexu/micro-service/services/iam/dal/model"
)

// UserRepo 封装 iam users 集合的 CRUD。
type UserRepo struct {
	repo *db.Repository[iammodel.User]
}

// NewUserRepo 构造 UserRepo。
func NewUserRepo(client *db.Client) *UserRepo {
	return &UserRepo{repo: db.NewRepository[iammodel.User](client, iammodel.UserCollection)}
}

// EnsureIndexes 建立 email 唯一索引。应在服务启动时调用一次。
func (r *UserRepo) EnsureIndexes(ctx context.Context, client *db.Client) error {
	return client.CreateIndexes(ctx, iammodel.UserCollection, []string{"email"}, true)
}

// Insert 插入新用户。
func (r *UserRepo) Insert(ctx context.Context, u *iammodel.User) (primitive.ObjectID, error) {
	id, err := r.repo.InsertOne(ctx, u)
	if err != nil {
		if db.IsDuplicateKey(err) {
			return primitive.NilObjectID, errno.ErrDuplicateKey.WithMessage("iam: user email already exists")
		}
		return primitive.NilObjectID, errno.ErrInternal.WithMessagef("iam: insert user: %v", err)
	}
	return id, nil
}

// FindByID 按 ID 查询用户。
func (r *UserRepo) FindByID(ctx context.Context, id primitive.ObjectID) (*iammodel.User, error) {
	u, err := r.repo.FindByID(ctx, id)
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrUserNotFound
		}
		return nil, errno.ErrInternal.WithMessagef("iam: find user by id: %v", err)
	}
	return u, nil
}

// FindByEmail 按 email 查询用户。
func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*iammodel.User, error) {
	u, err := r.repo.FindOne(ctx, bson.D{{Key: "email", Value: email}})
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrUserNotFound
		}
		return nil, errno.ErrInternal.WithMessagef("iam: find user by email: %v", err)
	}
	return u, nil
}

// UpdateProfile 更新用户资料字段（name / avatar_url）。
func (r *UserRepo) UpdateProfile(ctx context.Context, id primitive.ObjectID, name, avatarURL string) error {
	_, err := r.repo.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "name", Value: name},
			{Key: "avatar_url", Value: avatarURL},
		}}},
	)
	if err != nil {
		return errno.ErrInternal.WithMessagef("iam: update user profile: %v", err)
	}
	return nil
}
