// Package mongo 封装 idp 的 MongoDB 访问，禁止写业务逻辑。
package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/db"
	"github.com/castlexu/micro-service/pkg/errno"
	idpmodel "github.com/castlexu/micro-service/services/idp/dal/model"
)

// PasswordCredRepo 封装 password_credentials 集合操作。
type PasswordCredRepo struct {
	repo *db.Repository[idpmodel.PasswordCredential]
}

// NewPasswordCredRepo 构造 PasswordCredRepo。
func NewPasswordCredRepo(client *db.Client) *PasswordCredRepo {
	return &PasswordCredRepo{repo: db.NewRepository[idpmodel.PasswordCredential](client, idpmodel.PasswordCredentialCollection)}
}

// EnsureIndexes 建立 email 唯一索引。
func (r *PasswordCredRepo) EnsureIndexes(ctx context.Context, client *db.Client) error {
	return client.CreateIndexes(ctx, idpmodel.PasswordCredentialCollection, []string{"email"}, true)
}

// FindByEmail 按 email 查凭据。未找到返回 errno.ErrNotFound。
func (r *PasswordCredRepo) FindByEmail(ctx context.Context, email string) (*idpmodel.PasswordCredential, error) {
	doc, err := r.repo.FindOne(ctx, bson.D{{Key: "email", Value: email}})
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrNotFound.WithMessage("idp: credential not found")
		}
		return nil, errno.ErrInternal.WithMessagef("idp: find credential: %v", err)
	}
	return doc, nil
}

// InsertDirect 直接插入完整 PasswordCredential 文档（供 bootstrap 脚本使用）。
func (r *PasswordCredRepo) InsertDirect(ctx context.Context, cred *idpmodel.PasswordCredential) error {
	if _, err := r.repo.InsertOne(ctx, cred); err != nil {
		if db.IsDuplicateKey(err) {
			return errno.ErrDuplicateKey.WithMessage("idp: email already registered")
		}
		return errno.ErrInternal.WithMessagef("idp: insert credential: %v", err)
	}
	return nil
}
func (r *PasswordCredRepo) Insert(ctx context.Context, userID primitive.ObjectID, email, passwordHash string) error {
	doc := &idpmodel.PasswordCredential{
		BaseDoc:      db.BaseDoc{ID: primitive.NewObjectID()},
		UserID:       userID,
		Email:        email,
		PasswordHash: passwordHash,
	}
	if _, err := r.repo.InsertOne(ctx, doc); err != nil {
		if db.IsDuplicateKey(err) {
			return errno.ErrDuplicateKey.WithMessage("idp: email already registered")
		}
		return errno.ErrInternal.WithMessagef("idp: insert credential: %v", err)
	}
	return nil
}
