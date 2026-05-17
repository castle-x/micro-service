package biz

import (
	"context"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/errno"
	llmmodel "github.com/castlexu/micro-service/services/llm/dal/model"
)

type fakeModelRepo struct {
	inserted *llmmodel.Model
	items    []*llmmodel.Model
	byID     map[primitive.ObjectID]*llmmodel.Model
	byRef    map[string]*llmmodel.Model
}

func newFakeModelRepo() *fakeModelRepo {
	return &fakeModelRepo{
		byID:  map[primitive.ObjectID]*llmmodel.Model{},
		byRef: map[string]*llmmodel.Model{},
	}
}

func (r *fakeModelRepo) Insert(ctx context.Context, m *llmmodel.Model) (primitive.ObjectID, error) {
	id := primitive.NewObjectID()
	cp := *m
	cp.ID = id
	r.inserted = &cp
	r.items = append(r.items, &cp)
	r.byID[id] = &cp
	r.byRef[cp.ModelRef] = &cp
	return id, nil
}

func (r *fakeModelRepo) FindByID(ctx context.Context, id primitive.ObjectID) (*llmmodel.Model, error) {
	if m, ok := r.byID[id]; ok {
		cp := *m
		return &cp, nil
	}
	return nil, errno.ErrLLMModelNotFound
}

func (r *fakeModelRepo) FindByRef(ctx context.Context, modelRef string) (*llmmodel.Model, error) {
	if m, ok := r.byRef[modelRef]; ok {
		cp := *m
		return &cp, nil
	}
	return nil, errno.ErrLLMModelNotFound
}

func (r *fakeModelRepo) List(ctx context.Context, providerSlug string, enabled *bool) ([]*llmmodel.Model, error) {
	out := make([]*llmmodel.Model, 0, len(r.items))
	for _, m := range r.items {
		if providerSlug != "" && m.ProviderSlug != providerSlug {
			continue
		}
		if enabled != nil && m.Enabled != *enabled {
			continue
		}
		cp := *m
		out = append(out, &cp)
	}
	return out, nil
}

func (r *fakeModelRepo) Update(ctx context.Context, id primitive.ObjectID, patch ModelUpdatePatch) (*llmmodel.Model, error) {
	m, ok := r.byID[id]
	if !ok {
		return nil, errno.ErrLLMModelNotFound
	}
	if patch.DisplayName != nil {
		m.DisplayName = *patch.DisplayName
	}
	if patch.Capabilities != nil {
		m.Capabilities = append([]string(nil), *patch.Capabilities...)
	}
	if patch.ContextWindow != nil {
		m.ContextWindow = *patch.ContextWindow
	}
	if patch.MaxOutputTokens != nil {
		m.MaxOutputTokens = *patch.MaxOutputTokens
	}
	if patch.DefaultParametersJSON != nil {
		m.DefaultParametersJSON = *patch.DefaultParametersJSON
	}
	cp := *m
	return &cp, nil
}

func (r *fakeModelRepo) UpdateEnabled(ctx context.Context, id primitive.ObjectID, enabled bool) (*llmmodel.Model, error) {
	if m, ok := r.byID[id]; ok {
		m.Enabled = enabled
		cp := *m
		return &cp, nil
	}
	return nil, errno.ErrLLMModelNotFound
}

func (r *fakeModelRepo) Delete(ctx context.Context, id primitive.ObjectID) error {
	m, ok := r.byID[id]
	if !ok {
		return errno.ErrLLMModelNotFound
	}
	delete(r.byID, id)
	delete(r.byRef, m.ModelRef)
	next := r.items[:0]
	for _, item := range r.items {
		if item.ID != id {
			next = append(next, item)
		}
	}
	r.items = next
	return nil
}

func TestModelCreateValidatesProviderEnabled(t *testing.T) {
	providers := newFakeProviderRepo()
	providerID := primitive.NewObjectID()
	providers.bySlug["openai"] = &llmmodel.Provider{
		Slug:    "openai",
		Vendor:  "openai",
		BaseURL: "https://api.openai.com/v1",
		Enabled: false,
	}
	providers.bySlug["openai"].ID = providerID

	_, err := NewModelBiz(newFakeModelRepo(), providers).Create(context.Background(), ModelCreateReq{
		ProviderSlug: "openai",
		Model:        "gpt-4.1-mini",
		Capabilities: []string{"chat"},
	})
	if err == nil {
		t.Fatal("Create returned nil error for disabled provider")
	}
	if !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("Create error = %v, want disabled provider", err)
	}
}

func TestModelCreateRequiresCapabilities(t *testing.T) {
	providers := newFakeProviderRepo()
	providers.bySlug["openai"] = &llmmodel.Provider{
		Slug:    "openai",
		Vendor:  "openai",
		BaseURL: "https://api.openai.com/v1",
		Enabled: true,
	}
	providers.bySlug["openai"].ID = primitive.NewObjectID()

	_, err := NewModelBiz(newFakeModelRepo(), providers).Create(context.Background(), ModelCreateReq{
		ProviderSlug: "openai",
		Model:        "gpt-4.1-mini",
	})
	if err == nil {
		t.Fatal("Create returned nil error for empty capabilities")
	}
	if !strings.Contains(err.Error(), "capabilities") {
		t.Fatalf("Create error = %v, want capabilities validation", err)
	}
}

func TestModelCreateGeneratesModelRef(t *testing.T) {
	providers := newFakeProviderRepo()
	providerID := primitive.NewObjectID()
	providers.bySlug["openai"] = &llmmodel.Provider{
		Slug:    "openai",
		Vendor:  "openai",
		BaseURL: "https://api.openai.com/v1",
		Enabled: true,
	}
	providers.bySlug["openai"].ID = providerID
	models := newFakeModelRepo()

	got, err := NewModelBiz(models, providers).Create(context.Background(), ModelCreateReq{
		ProviderSlug:          "openai",
		Model:                 "gpt-4.1-mini",
		DisplayName:           "GPT-4.1 mini",
		Capabilities:          []string{"chat"},
		ContextWindow:         128000,
		MaxOutputTokens:       16384,
		DefaultParametersJSON: `{"temperature":0.7}`,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if got.ModelRef != "openai/gpt-4.1-mini" {
		t.Fatalf("ModelRef = %q, want provider/model ref", got.ModelRef)
	}
	if models.inserted.ProviderID != providerID {
		t.Fatalf("ProviderID = %s, want %s", models.inserted.ProviderID.Hex(), providerID.Hex())
	}
	if !models.inserted.Enabled {
		t.Fatal("model should be enabled on create")
	}
}

func TestModelListFiltersEnabledState(t *testing.T) {
	models := newFakeModelRepo()
	models.items = []*llmmodel.Model{
		{ProviderSlug: "openai", Model: "enabled", ModelRef: "openai/enabled", Enabled: true},
		{ProviderSlug: "openai", Model: "disabled", ModelRef: "openai/disabled", Enabled: false},
		{ProviderSlug: "other", Model: "enabled", ModelRef: "other/enabled", Enabled: true},
	}
	enabled := true

	items, err := NewModelBiz(models, newFakeProviderRepo()).List(context.Background(), "openai", &enabled)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 1 || items[0].ModelRef != "openai/enabled" {
		t.Fatalf("List items = %#v, want only enabled openai model", items)
	}
}

func TestModelDeleteRemovesModel(t *testing.T) {
	providers := newFakeProviderRepo()
	providerID := primitive.NewObjectID()
	providers.bySlug["openai"] = &llmmodel.Provider{
		Slug:    "openai",
		Vendor:  "openai",
		BaseURL: "https://api.openai.com/v1",
		Enabled: true,
	}
	providers.bySlug["openai"].ID = providerID
	models := newFakeModelRepo()
	biz := NewModelBiz(models, providers)

	created, err := biz.Create(context.Background(), ModelCreateReq{
		ProviderSlug: "openai",
		Model:        "gpt-4.1-mini",
		Capabilities: []string{"chat"},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if err := biz.Delete(context.Background(), created.ID.Hex()); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := models.FindByID(context.Background(), created.ID); !strings.Contains(err.Error(), errno.ErrLLMModelNotFound.Message) {
		t.Fatalf("FindByID after delete error = %v, want not found", err)
	}
}
