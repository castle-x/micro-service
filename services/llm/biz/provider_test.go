package biz

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/errno"
	"github.com/castlexu/micro-service/pkg/utils"
	llmmodel "github.com/castlexu/micro-service/services/llm/dal/model"
)

type fakeProviderRepo struct {
	inserted *llmmodel.Provider
	items    []*llmmodel.Provider
	byID     map[primitive.ObjectID]*llmmodel.Provider
	bySlug   map[string]*llmmodel.Provider
}

func newFakeProviderRepo() *fakeProviderRepo {
	return &fakeProviderRepo{
		byID:   map[primitive.ObjectID]*llmmodel.Provider{},
		bySlug: map[string]*llmmodel.Provider{},
	}
}

func (r *fakeProviderRepo) Insert(ctx context.Context, p *llmmodel.Provider) (primitive.ObjectID, error) {
	id := primitive.NewObjectID()
	cp := *p
	cp.ID = id
	r.inserted = &cp
	r.items = append(r.items, &cp)
	r.byID[id] = &cp
	r.bySlug[cp.Slug] = &cp
	return id, nil
}

func (r *fakeProviderRepo) FindByID(ctx context.Context, id primitive.ObjectID) (*llmmodel.Provider, error) {
	if p, ok := r.byID[id]; ok {
		cp := *p
		return &cp, nil
	}
	return nil, errno.ErrLLMProviderNotFound
}

func (r *fakeProviderRepo) FindBySlug(ctx context.Context, slug string) (*llmmodel.Provider, error) {
	if p, ok := r.bySlug[slug]; ok {
		cp := *p
		return &cp, nil
	}
	return nil, errno.ErrLLMProviderNotFound
}

func (r *fakeProviderRepo) List(ctx context.Context) ([]*llmmodel.Provider, error) {
	out := make([]*llmmodel.Provider, 0, len(r.items))
	for _, p := range r.items {
		cp := *p
		out = append(out, &cp)
	}
	return out, nil
}

func (r *fakeProviderRepo) Update(ctx context.Context, id primitive.ObjectID, patch ProviderUpdatePatch) (*llmmodel.Provider, error) {
	p, ok := r.byID[id]
	if !ok {
		return nil, errno.ErrLLMProviderNotFound
	}
	if patch.Name != nil {
		p.Name = *patch.Name
	}
	if patch.Vendor != nil {
		p.Vendor = *patch.Vendor
	}
	if patch.BaseURL != nil {
		p.BaseURL = *patch.BaseURL
	}
	if patch.DefaultModelRef != nil {
		p.DefaultModelRef = *patch.DefaultModelRef
	}
	if patch.ExtraJSON != nil {
		p.ExtraJSON = *patch.ExtraJSON
	}
	cp := *p
	return &cp, nil
}

func (r *fakeProviderRepo) UpdateAPIKey(ctx context.Context, id primitive.ObjectID, apiKeyCipher string) error {
	if p, ok := r.byID[id]; ok {
		p.APIKeyCipher = apiKeyCipher
		return nil
	}
	return errno.ErrLLMProviderNotFound
}

func (r *fakeProviderRepo) UpdateEnabled(ctx context.Context, id primitive.ObjectID, enabled bool) (*llmmodel.Provider, error) {
	if p, ok := r.byID[id]; ok {
		p.Enabled = enabled
		cp := *p
		return &cp, nil
	}
	return nil, errno.ErrLLMProviderNotFound
}

func (r *fakeProviderRepo) Delete(ctx context.Context, id primitive.ObjectID) error {
	p, ok := r.byID[id]
	if !ok {
		return errno.ErrLLMProviderNotFound
	}
	delete(r.byID, id)
	delete(r.bySlug, p.Slug)
	next := r.items[:0]
	for _, item := range r.items {
		if item.ID != id {
			next = append(next, item)
		}
	}
	r.items = next
	return nil
}

func TestProviderCreateValidatesRequiredFields(t *testing.T) {
	b := NewProviderBiz(newFakeProviderRepo(), []byte(strings.Repeat("k", 32)))

	_, err := b.Create(context.Background(), ProviderCreateReq{
		Name:    "OpenAI",
		Slug:    "openai",
		BaseURL: "https://api.openai.com/v1",
	})
	if err == nil {
		t.Fatal("Create returned nil error for missing vendor")
	}
	if !strings.Contains(err.Error(), "vendor") {
		t.Fatalf("Create error = %v, want vendor validation", err)
	}
}

func TestProviderCreateEncryptsAPIKeyAtRest(t *testing.T) {
	repo := newFakeProviderRepo()
	key := strings.Repeat("k", 32)
	b := NewProviderBiz(repo, []byte(key))

	_, err := b.Create(context.Background(), ProviderCreateReq{
		Name:    "OpenAI",
		Slug:    "openai",
		Vendor:  "openai",
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "sk-plaintext",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if repo.inserted.APIKeyCipher == "" {
		t.Fatal("APIKeyCipher is empty")
	}
	if repo.inserted.APIKeyCipher == "sk-plaintext" {
		t.Fatal("APIKeyCipher stored plaintext API key")
	}
	plain, err := utils.DecryptAESGCM([]byte(key), repo.inserted.APIKeyCipher)
	if err != nil {
		t.Fatalf("DecryptAESGCM returned error: %v", err)
	}
	if plain != "sk-plaintext" {
		t.Fatalf("decrypted API key = %q, want plaintext", plain)
	}
}

func TestProviderDeleteRejectsProviderWithModels(t *testing.T) {
	providers := newFakeProviderRepo()
	models := newFakeModelRepo()
	key := strings.Repeat("k", 32)
	b := NewProviderBiz(providers, []byte(key), models)

	p, err := b.Create(context.Background(), ProviderCreateReq{
		Name:    "OpenAI",
		Slug:    "openai",
		Vendor:  "openai",
		BaseURL: "https://api.openai.com/v1",
	})
	if err != nil {
		t.Fatalf("Create provider returned error: %v", err)
	}
	models.items = append(models.items, &llmmodel.Model{
		ProviderID:   p.ID,
		ProviderSlug: p.Slug,
		Model:        "gpt-4.1-mini",
		ModelRef:     "openai/gpt-4.1-mini",
	})

	err = b.Delete(context.Background(), p.ID.Hex())
	if err == nil {
		t.Fatal("Delete returned nil error for provider with models")
	}
	if !strings.Contains(err.Error(), "delete models first") {
		t.Fatalf("Delete error = %v, want delete models first", err)
	}
}

func TestProviderDeleteRemovesProviderWithoutModels(t *testing.T) {
	providers := newFakeProviderRepo()
	models := newFakeModelRepo()
	key := strings.Repeat("k", 32)
	b := NewProviderBiz(providers, []byte(key), models)

	p, err := b.Create(context.Background(), ProviderCreateReq{
		Name:    "OpenAI",
		Slug:    "openai",
		Vendor:  "openai",
		BaseURL: "https://api.openai.com/v1",
	})
	if err != nil {
		t.Fatalf("Create provider returned error: %v", err)
	}

	if err := b.Delete(context.Background(), p.ID.Hex()); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := providers.FindByID(context.Background(), p.ID); !strings.Contains(err.Error(), errno.ErrLLMProviderNotFound.Message) {
		t.Fatalf("FindByID after delete error = %v, want not found", err)
	}
}

func TestProviderListDTODoesNotLeakAPIKey(t *testing.T) {
	repo := newFakeProviderRepo()
	key := strings.Repeat("k", 32)
	cipher, err := utils.EncryptAESGCM([]byte(key), "sk-secret")
	if err != nil {
		t.Fatalf("EncryptAESGCM returned error: %v", err)
	}
	p := &llmmodel.Provider{
		Name:         "OpenAI",
		Slug:         "openai",
		Vendor:       "openai",
		BaseURL:      "https://api.openai.com/v1",
		APIKeyCipher: cipher,
		Enabled:      true,
	}
	p.ID = primitive.NewObjectID()
	repo.items = []*llmmodel.Provider{p}
	repo.byID[p.ID] = p
	repo.bySlug[p.Slug] = p

	items, err := NewProviderBiz(repo, []byte(key)).List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("List len = %d, want 1", len(items))
	}
	if strings.Contains(items[0].String(), "sk-secret") || strings.Contains(items[0].String(), cipher) {
		t.Fatalf("provider DTO leaked secret material: %s", items[0].String())
	}
}

func TestProviderGetForCallDecryptsEnabledProvider(t *testing.T) {
	repo := newFakeProviderRepo()
	key := strings.Repeat("k", 32)
	cipher, err := utils.EncryptAESGCM([]byte(key), "sk-secret")
	if err != nil {
		t.Fatalf("EncryptAESGCM returned error: %v", err)
	}
	repo.bySlug["openai"] = &llmmodel.Provider{
		Name:         "OpenAI",
		Slug:         "openai",
		Vendor:       "openai",
		BaseURL:      "https://api.openai.com/v1",
		APIKeyCipher: cipher,
		Enabled:      true,
	}

	got, err := NewProviderBiz(repo, []byte(key)).GetForCall(context.Background(), "openai")
	if err != nil {
		t.Fatalf("GetForCall returned error: %v", err)
	}
	if got.APIKey != "sk-secret" {
		t.Fatalf("APIKey = %q, want decrypted secret", got.APIKey)
	}
	if got.APIKeyCipher != "" {
		t.Fatalf("APIKeyCipher = %q, want redacted", got.APIKeyCipher)
	}
}

func TestProviderTestCallsOpenAICompatibleGenerate(t *testing.T) {
	var sawRequest bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		sawRequest = true
		if got := r.Header.Get("Authorization"); got != "Bearer fake-key" {
			t.Fatalf("Authorization = %q, want bearer fake-key", got)
		}
		var body struct {
			Model    string `json:"model"`
			Stream   bool   `json:"stream"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode request body error = %v", err)
		}
		if body.Model != "gpt-test" {
			t.Fatalf("model = %q, want suffix from default_model_ref", body.Model)
		}
		if body.Stream {
			t.Fatal("stream = true, want non-stream provider test")
		}
		if len(body.Messages) != 1 || body.Messages[0].Role != "user" || body.Messages[0].Content == "" {
			t.Fatalf("messages = %#v, want minimal user message", body.Messages)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-test","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":1,"total_tokens":4}}`))
	}))
	defer server.Close()

	b := NewProviderBiz(providerRepoWithTestProvider(t, server.URL, "fake-key", "openai/gpt-test"), []byte(strings.Repeat("k", 32)))
	got, err := b.Test(context.Background(), "507f1f77bcf86cd799439011", "")
	if err != nil {
		t.Fatalf("Test returned error: %v", err)
	}
	if !sawRequest {
		t.Fatal("upstream did not receive provider test request")
	}
	if got == nil || !got.OK {
		t.Fatalf("Test result = %#v, want OK", got)
	}
	if !strings.Contains(got.Message, "succeeded") {
		t.Fatalf("message = %q, want success summary", got.Message)
	}
}

func TestProviderTestUsesRequestedModelRefWhenProvided(t *testing.T) {
	var gotModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode request body error = %v", err)
		}
		gotModel = body.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"total_tokens":1}}`))
	}))
	defer server.Close()

	b := NewProviderBiz(providerRepoWithTestProvider(t, server.URL, "fake-key", "openai/default-model"), []byte(strings.Repeat("k", 32)))
	got, err := b.Test(context.Background(), "507f1f77bcf86cd799439011", "openai/override-model")
	if err != nil {
		t.Fatalf("Test returned error: %v", err)
	}
	if got == nil || !got.OK {
		t.Fatalf("Test result = %#v, want OK", got)
	}
	if gotModel != "override-model" {
		t.Fatalf("model = %q, want override-model from request model_ref", gotModel)
	}
}

func TestProviderTestDoesNotDuplicateV1Path(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"total_tokens":1}}`))
	}))
	defer server.Close()

	b := NewProviderBiz(providerRepoWithTestProvider(t, server.URL+"/v1", "fake-key", "openai/gpt-test"), []byte(strings.Repeat("k", 32)))
	got, err := b.Test(context.Background(), "507f1f77bcf86cd799439011", "")
	if err != nil {
		t.Fatalf("Test returned error: %v", err)
	}
	if got == nil || !got.OK {
		t.Fatalf("Test result = %#v, want OK", got)
	}
	if gotPath != "/v1/chat/completions" {
		t.Fatalf("upstream path = %q, want /v1/chat/completions", gotPath)
	}
}

func TestProviderTestUpstreamFailuresAreSanitized(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		want       string
	}{
		{
			name:       "unauthorized",
			statusCode: http.StatusUnauthorized,
			body:       `{"error":{"message":"bad Authorization: Bearer fake-key api_key=raw-provider-key","secret":"raw-provider-key"}}`,
			want:       "401",
		},
		{
			name:       "forbidden",
			statusCode: http.StatusForbidden,
			body:       `{"error":{"message":"forbidden token raw-provider-key","password":"p@ss"}}`,
			want:       "403",
		},
		{
			name:       "server_error",
			statusCode: http.StatusServiceUnavailable,
			body:       `{"error":{"message":"server failed with api_key fake-key"}}`,
			want:       "503",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/chat/completions" {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			b := NewProviderBiz(providerRepoWithTestProvider(t, server.URL, "fake-key", "openai/gpt-test"), []byte(strings.Repeat("k", 32)))
			got, err := b.Test(context.Background(), "507f1f77bcf86cd799439011", "")
			if err != nil {
				t.Fatalf("Test returned error: %v", err)
			}
			assertProviderTestFailureSanitized(t, got, tt.want, "fake-key", "raw-provider-key", "Authorization", "Bearer", "p@ss")
		})
	}
}

func TestProviderTestReports404PathFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.Error(w, `{"error":{"message":"wrong path","api_key":"fake-key"}}`, http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"total_tokens":1}}`))
	}))
	defer server.Close()

	b := NewProviderBiz(providerRepoWithTestProvider(t, server.URL+"/wrong", "fake-key", "openai/gpt-test"), []byte(strings.Repeat("k", 32)))
	got, err := b.Test(context.Background(), "507f1f77bcf86cd799439011", "")
	if err != nil {
		t.Fatalf("Test returned error: %v", err)
	}
	assertProviderTestFailureSanitized(t, got, "404", "fake-key", "api_key")
}

func TestProviderTestReportsTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"total_tokens":1}}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	b := NewProviderBiz(providerRepoWithTestProvider(t, server.URL, "fake-key", "openai/gpt-test"), []byte(strings.Repeat("k", 32)))
	got, err := b.Test(ctx, "507f1f77bcf86cd799439011", "")
	if err != nil {
		t.Fatalf("Test returned error: %v", err)
	}
	assertProviderTestFailureSanitized(t, got, "timeout", "fake-key", "Authorization")
}

func TestProviderTestRequiresUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	b := NewProviderBiz(providerRepoWithTestProvider(t, server.URL, "fake-key", "openai/gpt-test"), []byte(strings.Repeat("k", 32)))
	got, err := b.Test(context.Background(), "507f1f77bcf86cd799439011", "")
	if err != nil {
		t.Fatalf("Test returned error: %v", err)
	}
	assertProviderTestFailureSanitized(t, got, "usage", "fake-key", "Authorization")
}

func TestProviderTestReportsInvalidJSONWithSanitizedBodySummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>Cloud proxy returned fake-key</body></html>`))
	}))
	defer server.Close()

	b := NewProviderBiz(providerRepoWithTestProvider(t, server.URL, "fake-key", "openai/gpt-test"), []byte(strings.Repeat("k", 32)))
	got, err := b.Test(context.Background(), "507f1f77bcf86cd799439011", "")
	if err != nil {
		t.Fatalf("Test returned error: %v", err)
	}
	assertProviderTestFailureSanitized(t, got, "invalid JSON", "fake-key")
	if !strings.Contains(got.Message, "content_type=text/html") {
		t.Fatalf("message = %q, want content type hint", got.Message)
	}
	if !strings.Contains(got.Message, "<html><body>Cloud proxy returned") {
		t.Fatalf("message = %q, want body summary", got.Message)
	}
}

func TestProviderTestRequiresDefaultModelRef(t *testing.T) {
	tests := []struct {
		name     string
		modelRef string
		want     string
	}{
		{name: "missing", modelRef: "", want: "default_model_ref"},
		{name: "invalid", modelRef: "gpt-test", want: "provider/model"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("upstream should not be called for invalid default_model_ref")
			}))
			defer server.Close()

			b := NewProviderBiz(providerRepoWithTestProvider(t, server.URL, "fake-key", tt.modelRef), []byte(strings.Repeat("k", 32)))
			got, err := b.Test(context.Background(), "507f1f77bcf86cd799439011", "")
			if err != nil {
				t.Fatalf("Test returned error: %v", err)
			}
			assertProviderTestFailureSanitized(t, got, tt.want, "fake-key")
		})
	}
}

func providerRepoWithTestProvider(t *testing.T, baseURL, apiKey, defaultModelRef string) *fakeProviderRepo {
	t.Helper()
	id, err := primitive.ObjectIDFromHex("507f1f77bcf86cd799439011")
	if err != nil {
		t.Fatalf("ObjectIDFromHex error = %v", err)
	}
	key := []byte(strings.Repeat("k", 32))
	cipher, err := utils.EncryptAESGCM(key, apiKey)
	if err != nil {
		t.Fatalf("EncryptAESGCM returned error: %v", err)
	}
	p := &llmmodel.Provider{
		Name:            "Fake OpenAI",
		Slug:            "openai",
		Vendor:          "openai_compatible",
		BaseURL:         baseURL,
		APIKeyCipher:    cipher,
		Enabled:         true,
		DefaultModelRef: defaultModelRef,
	}
	p.ID = id
	repo := newFakeProviderRepo()
	repo.byID[id] = p
	repo.bySlug[p.Slug] = p
	repo.items = []*llmmodel.Provider{p}
	return repo
}

func assertProviderTestFailureSanitized(t *testing.T, got *ProviderTestResult, want string, forbidden ...string) {
	t.Helper()
	if got == nil {
		t.Fatal("Test result is nil")
	}
	if got.OK {
		t.Fatalf("OK = true, want failure: %#v", got)
	}
	if !strings.Contains(strings.ToLower(got.Message), strings.ToLower(want)) {
		t.Fatalf("message = %q, want containing %q", got.Message, want)
	}
	for _, leaked := range forbidden {
		if leaked != "" && strings.Contains(got.Message, leaked) {
			t.Fatalf("message leaked %q: %s", leaked, got.Message)
		}
	}
}
