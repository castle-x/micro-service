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

// EnsureIndexes 建立必要索引。
func (r *UserRepo) EnsureIndexes(ctx context.Context, client *db.Client) error {
	if err := client.CreateIndexes(ctx, iammodel.UserCollection, []string{"email"}, true); err != nil {
		return err
	}
	// phone 稀疏唯一索引（允许多个 null）
	return client.CreateIndexesWithOptions(ctx, iammodel.UserCollection, []string{"phone"},
		db.IndexOptions{Unique: true, Sparse: true, Name: "phone_sparse_unique"})
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

// UpdateProfile 更新用户资料字段。
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

// UpdateRole 更新用户角色。
func (r *UserRepo) UpdateRole(ctx context.Context, id primitive.ObjectID, role string) error {
	_, err := r.repo.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "role", Value: role}}}},
	)
	if err != nil {
		return errno.ErrInternal.WithMessagef("iam: update user role: %v", err)
	}
	return nil
}

// UpdateStatus 更新用户状态。
func (r *UserRepo) UpdateStatus(ctx context.Context, id primitive.ObjectID, status iammodel.UserStatus) error {
	_, err := r.repo.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "status", Value: status}}}},
	)
	if err != nil {
		return errno.ErrInternal.WithMessagef("iam: update user status: %v", err)
	}
	return nil
}

// List 分页查询用户，可按 role/status 过滤。
func (r *UserRepo) List(ctx context.Context, page, pageSize int, role string, status *iammodel.UserStatus) ([]*iammodel.User, int64, error) {
	filter := bson.D{}
	if role != "" {
		filter = append(filter, bson.E{Key: "role", Value: role})
	}
	if status != nil {
		filter = append(filter, bson.E{Key: "status", Value: *status})
	}

	total, err := r.repo.Count(ctx, filter)
	if err != nil {
		return nil, 0, errno.ErrInternal.WithMessagef("iam: count users: %v", err)
	}

	skip := int64((page - 1) * pageSize)
	users, err := r.repo.Find(ctx, filter, db.FindOptions{
		Sort:  bson.D{{Key: "created_at", Value: -1}},
		Skip:  skip,
		Limit: int64(pageSize),
	})
	if err != nil {
		return nil, 0, errno.ErrInternal.WithMessagef("iam: list users: %v", err)
	}
	return users, total, nil
}
