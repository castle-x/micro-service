// Package biz 实现 model service 的业务逻辑。
package biz

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/errno"
	"github.com/castlexu/micro-service/pkg/utils"
	mdlmodel "github.com/castlexu/micro-service/services/model/dal/model"
	mdlmongo "github.com/castlexu/micro-service/services/model/dal/mongo"
)

// ProviderBiz 处理 provider 的增删改查。
type ProviderBiz struct {
	repo       *mdlmongo.ProviderRepo
	encryptKey []byte // 32 字节 AES-256 主密钥，用于加密 api_key
}

// NewProviderBiz 构造 ProviderBiz。encryptKey 必须为 32 字节。
func NewProviderBiz(repo *mdlmongo.ProviderRepo, encryptKey []byte) *ProviderBiz {
	return &ProviderBiz{repo: repo, encryptKey: encryptKey}
}

// encryptKey 加密 api_key；空字符串直接返回空。
func (b *ProviderBiz) encryptAPIKey(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	return utils.EncryptAESGCM(b.encryptKey, plain)
}

// decryptAPIKey 解密 api_key；空字符串直接返回空。
func (b *ProviderBiz) decryptAPIKey(cipher string) (string, error) {
	if cipher == "" {
		return "", nil
	}
	return utils.DecryptAESGCM(b.encryptKey, cipher)
}

// CreateReq 新建 provider 的请求。
type CreateReq struct {
	Name         string                `json:"name"`
	Slug         string                `json:"slug"`
	Type         mdlmodel.ProviderType `json:"type"`
	BaseURL      string                `json:"base_url"`
	APIKey       string                `json:"api_key"`
	DefaultModel string                `json:"default_model"`
}

// Create 新建 provider（api_key 加密后入库）。
func (b *ProviderBiz) Create(ctx context.Context, req CreateReq) (*mdlmodel.Provider, error) {
	if req.Name == "" || req.Slug == "" || req.BaseURL == "" {
		return nil, errno.ErrInvalidParam.WithMessage("name, slug, base_url required")
	}
	if req.Type != mdlmodel.ProviderTypeLLM && req.Type != mdlmodel.ProviderTypeImage {
		return nil, errno.ErrInvalidParam.WithMessage("type must be llm or image")
	}
	encKey, err := b.encryptAPIKey(req.APIKey)
	if err != nil {
		return nil, errno.ErrInternal.WithMessagef("model: encrypt api_key: %v", err)
	}
	now := utils.NowUnix()
	p := &mdlmodel.Provider{
		Name:         req.Name,
		Slug:         req.Slug,
		Type:         req.Type,
		BaseURL:      req.BaseURL,
		APIKey:       encKey,
		DefaultModel: req.DefaultModel,
		Enabled:      true,
	}
	p.SetTimestamps(now)
	id, err := b.repo.Insert(ctx, p)
	if err != nil {
		return nil, err
	}
	p.ID = id
	return p, nil
}

// List 列出所有 provider（不暴露解密后 api_key）。
func (b *ProviderBiz) List(ctx context.Context) ([]*mdlmodel.Provider, error) {
	return b.repo.List(ctx)
}

// GetBySlug 按 slug 获取并解密 api_key（用于适配器调用）。
func (b *ProviderBiz) GetBySlug(ctx context.Context, slug string) (*mdlmodel.Provider, error) {
	p, err := b.repo.FindBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if !p.Enabled {
		return nil, errno.ErrProviderDisabled
	}
	plain, err := b.decryptAPIKey(p.APIKey)
	if err != nil {
		return nil, errno.ErrInternal.WithMessagef("model: decrypt api_key for slug %s: %v", slug, err)
	}
	p.APIKey = plain
	return p, nil
}

// SetEnabled 切换启用状态。
func (b *ProviderBiz) SetEnabled(ctx context.Context, id string, enabled bool) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errno.ErrInvalidParam.WithMessage("invalid provider id")
	}
	return b.repo.UpdateEnabled(ctx, oid, enabled)
}

// UpdateAPIKey 更新 API key（加密后入库）。
func (b *ProviderBiz) UpdateAPIKey(ctx context.Context, id, apiKey string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errno.ErrInvalidParam.WithMessage("invalid provider id")
	}
	encKey, err := b.encryptAPIKey(apiKey)
	if err != nil {
		return errno.ErrInternal.WithMessagef("model: encrypt api_key: %v", err)
	}
	return b.repo.UpdateAPIKey(ctx, oid, encKey)
}
