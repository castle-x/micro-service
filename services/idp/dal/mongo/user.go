// Package mongo 封装 idp 的 MongoDB 访问，禁止写业务逻辑。
package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/db"
	"github.com/castlexu/micro-service/pkg/errno"
	idpmodel "github.com/castlexu/micro-service/services/idp/dal/model"
)

// IdentityRepo 封装 identities 集合操作。
type IdentityRepo struct {
	repo *db.Repository[idpmodel.Identity]
}

// NewIdentityRepo 构造 IdentityRepo。
func NewIdentityRepo(client *db.Client) *IdentityRepo {
	return &IdentityRepo{repo: db.NewRepository[idpmodel.Identity](client, idpmodel.IdentityCollection)}
}

// EnsureIndexes 建立 (provider, provider_sub) 复合唯一索引。
func (r *IdentityRepo) EnsureIndexes(ctx context.Context, client *db.Client) error {
	return client.CreateIndexes(ctx, idpmodel.IdentityCollection, []string{"provider", "provider_sub"}, true)
}

// FindByProvider 按 provider + sub 查身份记录。
func (r *IdentityRepo) FindByProvider(ctx context.Context, provider, sub string) (*idpmodel.Identity, error) {
	doc, err := r.repo.FindOne(ctx, bson.D{
		{Key: "provider", Value: provider},
		{Key: "provider_sub", Value: sub},
	})
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrNotFound.WithMessage("idp: identity not found")
		}
		return nil, errno.ErrInternal.WithMessagef("idp: find identity: %v", err)
	}
	return doc, nil
}

// Upsert 幂等：存在则返回原记录，不存在则插入。返回 identity 和 created 标志。
func (r *IdentityRepo) Upsert(ctx context.Context, provider, sub string, userID primitive.ObjectID, email string) (*idpmodel.Identity, bool, error) {
	existing, err := r.FindByProvider(ctx, provider, sub)
	if err == nil {
		return existing, false, nil
	}

	id := &idpmodel.Identity{
		BaseDoc:     db.BaseDoc{ID: primitive.NewObjectID()},
		Provider:    provider,
		ProviderSub: sub,
		UserID:      userID,
		Email:       email,
	}
	if _, err := r.repo.InsertOne(ctx, id); err != nil {
		return nil, false, errno.ErrInternal.WithMessagef("idp: insert identity: %v", err)
	}
	return id, true, nil
}

// ---- OAuthStateRepo ----

// OAuthStateRepo 封装 oauth_states 集合操作。
type OAuthStateRepo struct {
	repo   *db.Repository[idpmodel.OAuthState]
	client *db.Client
}

// NewOAuthStateRepo 构造 OAuthStateRepo。
func NewOAuthStateRepo(client *db.Client) *OAuthStateRepo {
	return &OAuthStateRepo{
		repo:   db.NewRepository[idpmodel.OAuthState](client, idpmodel.OAuthStateCollection),
		client: client,
	}
}

// EnsureIndexes 建立 TTL 索引（10 分钟）和 state 唯一索引。
func (r *OAuthStateRepo) EnsureIndexes(ctx context.Context) error {
	ttl := int32(600)
	if err := r.client.CreateIndexesWithOptions(ctx, idpmodel.OAuthStateCollection,
		[]string{"created_at"},
		db.IndexOptions{Name: "created_at_ttl", ExpireAfterSeconds: &ttl},
	); err != nil {
		return err
	}
	return r.client.CreateIndexes(ctx, idpmodel.OAuthStateCollection, []string{"state"}, true)
}

// Save 存入一个 state。
func (r *OAuthStateRepo) Save(ctx context.Context, state, redirectURI string) error {
	doc := &idpmodel.OAuthState{
		BaseDoc:     db.BaseDoc{ID: primitive.NewObjectID()},
		State:       state,
		RedirectURI: redirectURI,
	}
	_, err := r.repo.InsertOne(ctx, doc)
	if err != nil {
		return errno.ErrInternal.WithMessagef("idp: save oauth state: %v", err)
	}
	return nil
}

// ConsumeAndDelete 验证 state 并物理删除（一次性消费，防重放）。
func (r *OAuthStateRepo) ConsumeAndDelete(ctx context.Context, state string) (*idpmodel.OAuthState, error) {
	doc, err := r.repo.FindOne(ctx, bson.D{{Key: "state", Value: state}})
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrInvalidParam.WithMessage("idp: oauth state not found or expired")
		}
		return nil, errno.ErrInternal.WithMessagef("idp: find oauth state: %v", err)
	}
	// 双重校验：TTL 索引是异步清理，防止极端竞态
	if doc.CreatedAt > 0 && time.Now().Unix()-doc.CreatedAt > 600 {
		return nil, errno.ErrInvalidParam.WithMessage("idp: oauth state expired")
	}
	if err := r.repo.HardDeleteOne(ctx, bson.D{{Key: "state", Value: state}}); err != nil {
		return nil, errno.ErrInternal.WithMessagef("idp: delete oauth state: %v", err)
	}
	return doc, nil
}
