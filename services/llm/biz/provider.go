// Package biz implements llm service business logic.
package biz

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/errno"
	"github.com/castlexu/micro-service/pkg/utils"
	llmmodel "github.com/castlexu/micro-service/services/llm/dal/model"
	"github.com/castlexu/micro-service/services/llm/security"
)

// ProviderUpdatePatch aliases the DAL patch type for repository test doubles.
type ProviderUpdatePatch = llmmodel.ProviderUpdatePatch

// Provider aliases the DAL provider document for handler seams.
type Provider = llmmodel.Provider

// ProviderRepository is the storage seam used by ProviderBiz.
type ProviderRepository interface {
	Insert(ctx context.Context, p *llmmodel.Provider) (primitive.ObjectID, error)
	FindByID(ctx context.Context, id primitive.ObjectID) (*llmmodel.Provider, error)
	FindBySlug(ctx context.Context, slug string) (*llmmodel.Provider, error)
	List(ctx context.Context) ([]*llmmodel.Provider, error)
	Update(ctx context.Context, id primitive.ObjectID, patch ProviderUpdatePatch) (*llmmodel.Provider, error)
	UpdateAPIKey(ctx context.Context, id primitive.ObjectID, apiKeyCipher string) error
	UpdateEnabled(ctx context.Context, id primitive.ObjectID, enabled bool) (*llmmodel.Provider, error)
	Delete(ctx context.Context, id primitive.ObjectID) error
}

type providerModelLister interface {
	List(ctx context.Context, providerSlug string, enabled *bool) ([]*llmmodel.Model, error)
}

// ProviderBiz handles provider CRUD and secret encryption.
type ProviderBiz struct {
	repo       ProviderRepository
	models     providerModelLister
	encryptKey []byte
}

// NewProviderBiz constructs ProviderBiz. encryptKey must be 32 bytes.
func NewProviderBiz(repo ProviderRepository, encryptKey []byte, models ...providerModelLister) *ProviderBiz {
	b := &ProviderBiz{repo: repo, encryptKey: encryptKey}
	if len(models) > 0 {
		b.models = models[0]
	}
	return b
}

// ProviderCreateReq creates a provider. APIKey is only accepted on write.
type ProviderCreateReq struct {
	Name            string `json:"name"`
	Slug            string `json:"slug"`
	Vendor          string `json:"vendor"`
	BaseURL         string `json:"base_url"`
	APIKey          string `json:"api_key"`
	Enabled         *bool  `json:"enabled,omitempty"`
	DefaultModelRef string `json:"default_model_ref"`
	ExtraJSON       string `json:"extra_json"`
}

// ProviderUpdateReq updates writable provider fields except API key and enabled.
type ProviderUpdateReq struct {
	Name            *string `json:"name"`
	Vendor          *string `json:"vendor"`
	BaseURL         *string `json:"base_url"`
	DefaultModelRef *string `json:"default_model_ref"`
	ExtraJSON       *string `json:"extra_json"`
}

// ProviderDTO is the public provider view. It never includes API key material.
type ProviderDTO struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Slug            string `json:"slug"`
	Vendor          string `json:"vendor"`
	BaseURL         string `json:"base_url"`
	Enabled         bool   `json:"enabled"`
	DefaultModelRef string `json:"default_model_ref,omitempty"`
	ExtraJSON       string `json:"extra_json,omitempty"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
}

func (d ProviderDTO) String() string {
	raw, _ := json.Marshal(d)
	return string(raw)
}

// ProviderForCall is the short-lived internal view used for upstream calls.
type ProviderForCall struct {
	ID              string
	Name            string
	Slug            string
	Vendor          string
	BaseURL         string
	APIKey          string
	APIKeyCipher    string
	DefaultModelRef string
	ExtraJSON       string
}

// ProviderTestResult is returned by the provider test endpoint.
type ProviderTestResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

func (b *ProviderBiz) encryptAPIKey(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	return utils.EncryptAESGCM(b.encryptKey, plain)
}

func (b *ProviderBiz) decryptAPIKey(cipher string) (string, error) {
	if cipher == "" {
		return "", nil
	}
	return utils.DecryptAESGCM(b.encryptKey, cipher)
}

// Decrypt implements component.APIKeyDecrypter without exposing plaintext in DTOs.
func (b *ProviderBiz) Decrypt(ctx context.Context, ciphertext string) (string, error) {
	_ = ctx
	return b.decryptAPIKey(ciphertext)
}

// Create creates a provider and stores api_key encrypted.
func (b *ProviderBiz) Create(ctx context.Context, req ProviderCreateReq) (*llmmodel.Provider, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Slug = strings.TrimSpace(req.Slug)
	req.Vendor = strings.TrimSpace(req.Vendor)
	req.BaseURL = strings.TrimSpace(req.BaseURL)
	if req.Name == "" || req.Slug == "" || req.Vendor == "" || req.BaseURL == "" {
		return nil, errno.ErrInvalidParam.WithMessage("name, slug, vendor, base_url required")
	}
	if req.ExtraJSON != "" && !json.Valid([]byte(req.ExtraJSON)) {
		return nil, errno.ErrInvalidParam.WithMessage("extra_json must be valid JSON")
	}
	encKey, err := b.encryptAPIKey(req.APIKey)
	if err != nil {
		return nil, errno.ErrInternal.WithMessage("llm: encrypt api_key failed")
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	p := &llmmodel.Provider{
		Name:            req.Name,
		Slug:            req.Slug,
		Vendor:          req.Vendor,
		BaseURL:         req.BaseURL,
		APIKeyCipher:    encKey,
		Enabled:         enabled,
		DefaultModelRef: strings.TrimSpace(req.DefaultModelRef),
		ExtraJSON:       req.ExtraJSON,
	}
	id, err := b.repo.Insert(ctx, p)
	if err != nil {
		return nil, err
	}
	p.ID = id
	return p, nil
}

// List returns provider DTOs without API key material.
func (b *ProviderBiz) List(ctx context.Context) ([]ProviderDTO, error) {
	providers, err := b.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ProviderDTO, 0, len(providers))
	for _, p := range providers {
		out = append(out, providerDTO(p))
	}
	return out, nil
}

// Update updates writable provider metadata.
func (b *ProviderBiz) Update(ctx context.Context, id string, req ProviderUpdateReq) (*ProviderDTO, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errno.ErrInvalidParam.WithMessage("invalid provider id")
	}
	patch := ProviderUpdatePatch{
		Name:            trimStringPtr(req.Name),
		Vendor:          trimStringPtr(req.Vendor),
		BaseURL:         trimStringPtr(req.BaseURL),
		DefaultModelRef: trimStringPtr(req.DefaultModelRef),
		ExtraJSON:       req.ExtraJSON,
	}
	if patch.Name != nil && *patch.Name == "" {
		return nil, errno.ErrInvalidParam.WithMessage("name required")
	}
	if patch.Vendor != nil && *patch.Vendor == "" {
		return nil, errno.ErrInvalidParam.WithMessage("vendor required")
	}
	if patch.BaseURL != nil && *patch.BaseURL == "" {
		return nil, errno.ErrInvalidParam.WithMessage("base_url required")
	}
	if patch.ExtraJSON != nil && *patch.ExtraJSON != "" && !json.Valid([]byte(*patch.ExtraJSON)) {
		return nil, errno.ErrInvalidParam.WithMessage("extra_json must be valid JSON")
	}
	p, err := b.repo.Update(ctx, oid, patch)
	if err != nil {
		return nil, err
	}
	dto := providerDTO(p)
	return &dto, nil
}

// UpdateAPIKey encrypts and stores a provider API key.
func (b *ProviderBiz) UpdateAPIKey(ctx context.Context, id, apiKey string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errno.ErrInvalidParam.WithMessage("invalid provider id")
	}
	if apiKey == "" {
		return errno.ErrInvalidParam.WithMessage("api_key required")
	}
	encKey, err := b.encryptAPIKey(apiKey)
	if err != nil {
		return errno.ErrInternal.WithMessage("llm: encrypt api_key failed")
	}
	return b.repo.UpdateAPIKey(ctx, oid, encKey)
}

// SetEnabled toggles provider enabled state.
func (b *ProviderBiz) SetEnabled(ctx context.Context, id string, enabled bool) (*ProviderDTO, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errno.ErrInvalidParam.WithMessage("invalid provider id")
	}
	p, err := b.repo.UpdateEnabled(ctx, oid, enabled)
	if err != nil {
		return nil, err
	}
	dto := providerDTO(p)
	return &dto, nil
}

// Delete removes a provider after ensuring no models still reference it.
func (b *ProviderBiz) Delete(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errno.ErrInvalidParam.WithMessage("invalid provider id")
	}
	p, err := b.repo.FindByID(ctx, oid)
	if err != nil {
		return err
	}
	if b.models != nil {
		models, err := b.models.List(ctx, p.Slug, nil)
		if err != nil {
			return err
		}
		if len(models) > 0 {
			return errno.ErrInvalidParam.WithMessage("provider has models; delete models first")
		}
	}
	return b.repo.Delete(ctx, oid)
}

// GetForCall fetches an enabled provider and decrypts the API key for immediate use.
func (b *ProviderBiz) GetForCall(ctx context.Context, slug string) (*ProviderForCall, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, errno.ErrInvalidParam.WithMessage("provider slug required")
	}
	p, err := b.repo.FindBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if !p.Enabled {
		return nil, errno.ErrLLMProviderDisabled
	}
	plain, err := b.decryptAPIKey(p.APIKeyCipher)
	if err != nil {
		return nil, errno.ErrInternal.WithMessage("llm: decrypt api_key failed")
	}
	return &ProviderForCall{
		ID:              p.ID.Hex(),
		Name:            p.Name,
		Slug:            p.Slug,
		Vendor:          p.Vendor,
		BaseURL:         p.BaseURL,
		APIKey:          plain,
		DefaultModelRef: p.DefaultModelRef,
		ExtraJSON:       p.ExtraJSON,
	}, nil
}

// Test performs a minimal non-streaming OpenAI-compatible connectivity check.
func (b *ProviderBiz) Test(ctx context.Context, id, modelRef string) (*ProviderTestResult, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errno.ErrInvalidParam.WithMessage("invalid provider id")
	}
	p, err := b.repo.FindByID(ctx, oid)
	if err != nil {
		return nil, err
	}
	if !p.Enabled {
		return nil, errno.ErrLLMProviderDisabled
	}
	target, err := providerTestURL(p.BaseURL)
	if err != nil {
		return &ProviderTestResult{OK: false, Message: "invalid provider base_url"}, nil
	}
	apiKey, err := b.decryptAPIKey(p.APIKeyCipher)
	if err != nil {
		return &ProviderTestResult{OK: false, Message: "provider api_key decrypt failed"}, nil
	}
	if strings.TrimSpace(apiKey) == "" {
		return &ProviderTestResult{OK: false, Message: "provider api_key required"}, nil
	}
	selectedModelRef := strings.TrimSpace(modelRef)
	modelRefField := "model_ref"
	if selectedModelRef == "" {
		selectedModelRef = p.DefaultModelRef
		modelRefField = "default_model_ref"
	}
	model, err := providerTestModel(selectedModelRef, modelRefField)
	if err != nil {
		return &ProviderTestResult{OK: false, Message: err.Error()}, nil
	}

	body, err := json.Marshal(map[string]any{
		"model":  model,
		"stream": false,
		"messages": []map[string]string{
			{"role": "user", "content": "ping"},
		},
	})
	if err != nil {
		return &ProviderTestResult{OK: false, Message: "provider test request encode failed"}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return &ProviderTestResult{OK: false, Message: "provider test request build failed"}, nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return &ProviderTestResult{OK: false, Message: providerTestTransportMessage(err, apiKey, p.APIKeyCipher)}, nil
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		msg := fmt.Sprintf("provider test failed: upstream returned HTTP %d", resp.StatusCode)
		if len(strings.TrimSpace(string(raw))) > 0 {
			msg += ": " + security.RedactText(string(raw), apiKey, p.APIKeyCipher)
		}
		return &ProviderTestResult{OK: false, Message: msg}, nil
	}

	var out providerTestChatCompletionResp
	if err := json.Unmarshal(raw, &out); err != nil {
		return &ProviderTestResult{OK: false, Message: providerTestInvalidJSONMessage(resp, raw, apiKey, p.APIKeyCipher)}, nil
	}
	if len(out.Choices) == 0 {
		return &ProviderTestResult{OK: false, Message: "provider test failed: upstream returned no choices"}, nil
	}
	if out.Usage == nil {
		return &ProviderTestResult{OK: false, Message: "provider test failed: upstream returned no usage"}, nil
	}
	finishReason := out.Choices[0].FinishReason
	if finishReason == "" {
		finishReason = "unknown"
	}
	return &ProviderTestResult{
		OK:      true,
		Message: fmt.Sprintf("provider test succeeded: model %s finish_reason=%s total_tokens=%d", model, finishReason, out.Usage.TotalTokens),
	}, nil
}

type providerTestChatCompletionResp struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *Usage `json:"usage"`
}

func providerTestURL(baseURL string) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "", errors.New("empty base url")
	}
	parsed, err := url.ParseRequestURI(baseURL)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("base url requires scheme and host")
	}
	path := strings.TrimRight(parsed.Path, "/")
	if path == "" {
		path = "/chat/completions"
	} else if !strings.HasSuffix(path, "/chat/completions") {
		path += "/chat/completions"
	}
	parsed.Path = path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func providerTestModel(modelRef, field string) (string, error) {
	modelRef = strings.TrimSpace(modelRef)
	if modelRef == "" {
		return "", fmt.Errorf("%s required", field)
	}
	provider, model, ok := strings.Cut(modelRef, "/")
	provider = strings.TrimSpace(provider)
	model = strings.TrimSpace(model)
	if !ok || provider == "" || model == "" {
		return "", fmt.Errorf("%s must use provider/model", field)
	}
	return model, nil
}

func providerTestInvalidJSONMessage(resp *http.Response, raw []byte, secrets ...string) string {
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "unknown"
	}
	body := strings.TrimSpace(string(raw))
	if len(body) > 240 {
		body = body[:240] + "..."
	}
	body = security.RedactText(body, secrets...)
	return fmt.Sprintf("provider test failed: upstream returned invalid JSON: HTTP %d content_type=%s body=%s", resp.StatusCode, contentType, body)
}

func providerTestTransportMessage(err error, secrets ...string) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "provider test timeout"
	}
	return "provider test failed: " + security.RedactText(err.Error(), secrets...)
}

func providerDTO(p *llmmodel.Provider) ProviderDTO {
	if p == nil {
		return ProviderDTO{}
	}
	return ProviderDTO{
		ID:              p.ID.Hex(),
		Name:            p.Name,
		Slug:            p.Slug,
		Vendor:          p.Vendor,
		BaseURL:         p.BaseURL,
		Enabled:         p.Enabled,
		DefaultModelRef: p.DefaultModelRef,
		ExtraJSON:       p.ExtraJSON,
		CreatedAt:       p.CreatedAt,
		UpdatedAt:       p.UpdatedAt,
	}
}

func trimStringPtr(in *string) *string {
	if in == nil {
		return nil
	}
	out := strings.TrimSpace(*in)
	return &out
}
