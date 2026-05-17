package biz

import (
	"context"
	"encoding/json"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/errno"
	"github.com/castlexu/micro-service/services/llm/component"
	llmmodel "github.com/castlexu/micro-service/services/llm/dal/model"
)

// ModelUpdatePatch aliases the DAL patch type for repository test doubles.
type ModelUpdatePatch = llmmodel.ModelUpdatePatch

// Model aliases the DAL model document for handler seams.
type Model = llmmodel.Model

// ModelRepository is the storage seam used by ModelBiz.
type ModelRepository interface {
	Insert(ctx context.Context, m *llmmodel.Model) (primitive.ObjectID, error)
	FindByID(ctx context.Context, id primitive.ObjectID) (*llmmodel.Model, error)
	FindByRef(ctx context.Context, modelRef string) (*llmmodel.Model, error)
	List(ctx context.Context, providerSlug string, enabled *bool) ([]*llmmodel.Model, error)
	Update(ctx context.Context, id primitive.ObjectID, patch ModelUpdatePatch) (*llmmodel.Model, error)
	UpdateEnabled(ctx context.Context, id primitive.ObjectID, enabled bool) (*llmmodel.Model, error)
	Delete(ctx context.Context, id primitive.ObjectID) error
}

type providerReader interface {
	FindBySlug(ctx context.Context, slug string) (*llmmodel.Provider, error)
}

// ModelBiz handles model CRUD and resolution.
type ModelBiz struct {
	models    ModelRepository
	providers providerReader
}

// NewModelBiz constructs ModelBiz.
func NewModelBiz(models ModelRepository, providers providerReader) *ModelBiz {
	return &ModelBiz{models: models, providers: providers}
}

// ModelCreateReq creates an LLM model under a provider.
type ModelCreateReq struct {
	ProviderSlug          string   `json:"provider_slug"`
	Model                 string   `json:"model"`
	DisplayName           string   `json:"display_name"`
	Capabilities          []string `json:"capabilities"`
	ContextWindow         int      `json:"context_window"`
	MaxOutputTokens       int      `json:"max_output_tokens"`
	DefaultParametersJSON string   `json:"default_parameters_json"`
	Enabled               *bool    `json:"enabled,omitempty"`
}

// ModelUpdateReq updates writable model fields.
type ModelUpdateReq struct {
	DisplayName           *string   `json:"display_name"`
	Capabilities          *[]string `json:"capabilities"`
	ContextWindow         *int      `json:"context_window"`
	MaxOutputTokens       *int      `json:"max_output_tokens"`
	DefaultParametersJSON *string   `json:"default_parameters_json"`
}

// ModelDTO is the public model view.
type ModelDTO struct {
	ID                    string   `json:"id"`
	ProviderID            string   `json:"provider_id"`
	ProviderSlug          string   `json:"provider_slug"`
	Model                 string   `json:"model"`
	ModelRef              string   `json:"model_ref"`
	DisplayName           string   `json:"display_name,omitempty"`
	Capabilities          []string `json:"capabilities"`
	ContextWindow         int      `json:"context_window,omitempty"`
	MaxOutputTokens       int      `json:"max_output_tokens,omitempty"`
	DefaultParametersJSON string   `json:"default_parameters_json,omitempty"`
	Enabled               bool     `json:"enabled"`
	CreatedAt             int64    `json:"created_at"`
	UpdatedAt             int64    `json:"updated_at"`
}

// Create creates a model and generates model_ref as <provider_slug>/<model>.
func (b *ModelBiz) Create(ctx context.Context, req ModelCreateReq) (*llmmodel.Model, error) {
	req.ProviderSlug = strings.TrimSpace(req.ProviderSlug)
	req.Model = strings.TrimSpace(req.Model)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.Capabilities = normalizeCapabilities(req.Capabilities)
	if req.ProviderSlug == "" || req.Model == "" {
		return nil, errno.ErrInvalidParam.WithMessage("provider_slug, model required")
	}
	if len(req.Capabilities) == 0 {
		return nil, errno.ErrInvalidParam.WithMessage("capabilities required")
	}
	if req.DefaultParametersJSON != "" && !json.Valid([]byte(req.DefaultParametersJSON)) {
		return nil, errno.ErrInvalidParam.WithMessage("default_parameters_json must be valid JSON")
	}
	p, err := b.providers.FindBySlug(ctx, req.ProviderSlug)
	if err != nil {
		return nil, err
	}
	if !p.Enabled {
		return nil, errno.ErrLLMProviderDisabled
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	m := &llmmodel.Model{
		ProviderID:            p.ID,
		ProviderSlug:          p.Slug,
		Model:                 req.Model,
		ModelRef:              p.Slug + "/" + req.Model,
		DisplayName:           req.DisplayName,
		Capabilities:          req.Capabilities,
		ContextWindow:         req.ContextWindow,
		MaxOutputTokens:       req.MaxOutputTokens,
		DefaultParametersJSON: req.DefaultParametersJSON,
		Enabled:               enabled,
	}
	id, err := b.models.Insert(ctx, m)
	if err != nil {
		return nil, err
	}
	m.ID = id
	return m, nil
}

// List returns model DTOs, optionally filtered by provider slug and enabled state.
func (b *ModelBiz) List(ctx context.Context, providerSlug string, enabled *bool) ([]ModelDTO, error) {
	items, err := b.models.List(ctx, strings.TrimSpace(providerSlug), enabled)
	if err != nil {
		return nil, err
	}
	out := make([]ModelDTO, 0, len(items))
	for _, m := range items {
		out = append(out, modelDTO(m))
	}
	return out, nil
}

// Update updates model metadata.
func (b *ModelBiz) Update(ctx context.Context, id string, req ModelUpdateReq) (*ModelDTO, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errno.ErrInvalidParam.WithMessage("invalid model id")
	}
	patch := ModelUpdatePatch{
		DisplayName:           trimStringPtr(req.DisplayName),
		Capabilities:          req.Capabilities,
		ContextWindow:         req.ContextWindow,
		MaxOutputTokens:       req.MaxOutputTokens,
		DefaultParametersJSON: req.DefaultParametersJSON,
	}
	if patch.Capabilities != nil {
		caps := normalizeCapabilities(*patch.Capabilities)
		if len(caps) == 0 {
			return nil, errno.ErrInvalidParam.WithMessage("capabilities required")
		}
		patch.Capabilities = &caps
	}
	if patch.DefaultParametersJSON != nil && *patch.DefaultParametersJSON != "" && !json.Valid([]byte(*patch.DefaultParametersJSON)) {
		return nil, errno.ErrInvalidParam.WithMessage("default_parameters_json must be valid JSON")
	}
	m, err := b.models.Update(ctx, oid, patch)
	if err != nil {
		return nil, err
	}
	dto := modelDTO(m)
	return &dto, nil
}

// SetEnabled toggles model enabled state.
func (b *ModelBiz) SetEnabled(ctx context.Context, id string, enabled bool) (*ModelDTO, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errno.ErrInvalidParam.WithMessage("invalid model id")
	}
	m, err := b.models.UpdateEnabled(ctx, oid, enabled)
	if err != nil {
		return nil, err
	}
	dto := modelDTO(m)
	return &dto, nil
}

// Delete removes a model.
func (b *ModelBiz) Delete(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errno.ErrInvalidParam.WithMessage("invalid model id")
	}
	return b.models.Delete(ctx, oid)
}

// ResolveModel implements component.ModelResolver.
func (b *ModelBiz) ResolveModel(ctx context.Context, modelRef string) (*component.ResolvedModel, error) {
	modelRef = strings.TrimSpace(modelRef)
	if modelRef == "" {
		return nil, errno.ErrInvalidParam.WithMessage("model_ref required")
	}
	m, err := b.models.FindByRef(ctx, modelRef)
	if err != nil {
		return nil, err
	}
	if !m.Enabled {
		return nil, errno.ErrLLMModelDisabled
	}
	p, err := b.providers.FindBySlug(ctx, m.ProviderSlug)
	if err != nil {
		return nil, err
	}
	if !p.Enabled {
		return nil, errno.ErrLLMProviderDisabled
	}
	return &component.ResolvedModel{
		Ref:             m.ModelRef,
		Model:           m.Model,
		ProviderSlug:    p.Slug,
		ProviderVendor:  p.Vendor,
		ProviderType:    component.ProviderTypeLLM,
		ProviderBaseURL: p.BaseURL,
		EncryptedAPIKey: p.APIKeyCipher,
		Capabilities:    append([]string(nil), m.Capabilities...),
		MaxOutputTokens: m.MaxOutputTokens,
	}, nil
}

func modelDTO(m *llmmodel.Model) ModelDTO {
	if m == nil {
		return ModelDTO{}
	}
	return ModelDTO{
		ID:                    m.ID.Hex(),
		ProviderID:            m.ProviderID.Hex(),
		ProviderSlug:          m.ProviderSlug,
		Model:                 m.Model,
		ModelRef:              m.ModelRef,
		DisplayName:           m.DisplayName,
		Capabilities:          append([]string(nil), m.Capabilities...),
		ContextWindow:         m.ContextWindow,
		MaxOutputTokens:       m.MaxOutputTokens,
		DefaultParametersJSON: m.DefaultParametersJSON,
		Enabled:               m.Enabled,
		CreatedAt:             m.CreatedAt,
		UpdatedAt:             m.UpdatedAt,
	}
}

func normalizeCapabilities(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, cap := range in {
		cap = strings.TrimSpace(cap)
		if cap == "" {
			continue
		}
		if _, ok := seen[cap]; ok {
			continue
		}
		seen[cap] = struct{}{}
		out = append(out, cap)
	}
	return out
}
