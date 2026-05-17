package component

import (
	"context"
	"errors"
	"testing"

	"github.com/castlexu/micro-service/pkg/errno"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestFactoryBuildResolvesOpenAICompatibleModel(t *testing.T) {
	ctx := context.Background()
	resolved := &ResolvedModel{
		Ref:             "chat.default",
		ProviderSlug:    "deepseek",
		ProviderVendor:  VendorOpenAICompatible,
		ProviderType:    ProviderTypeLLM,
		ProviderBaseURL: "https://api.deepseek.com",
		EncryptedAPIKey: "encrypted-key",
		Model:           "deepseek-chat",
		Capabilities:    []string{CapabilityToolCalling},
	}
	expectedModel := &fakeChatModel{}
	repo := &fakeResolver{resolved: resolved}
	decrypter := &fakeDecrypter{plain: "plain-key"}
	builder := &fakeBuilder{chatModel: expectedModel}

	factory := NewFactory(repo, decrypter, WithBuilder(VendorOpenAICompatible, builder))

	gotModel, gotResolved, err := factory.Build(ctx, "chat.default")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if gotModel != expectedModel {
		t.Fatalf("Build() model = %v, want fake model", gotModel)
	}
	if gotResolved != resolved {
		t.Fatalf("Build() resolved = %v, want repository resolved model", gotResolved)
	}
	if repo.gotRef != "chat.default" {
		t.Fatalf("resolver ref = %q, want chat.default", repo.gotRef)
	}
	if decrypter.gotCiphertext != "encrypted-key" {
		t.Fatalf("decrypted ciphertext = %q, want encrypted-key", decrypter.gotCiphertext)
	}
	if builder.gotResolved != resolved {
		t.Fatalf("builder resolved model mismatch")
	}
	if builder.gotAPIKey != "plain-key" {
		t.Fatalf("builder api key = %q, want plain-key", builder.gotAPIKey)
	}
}

func TestFactoryBuildRejectsUnsupportedVendor(t *testing.T) {
	ctx := context.Background()
	resolved := &ResolvedModel{
		Ref:             "chat.default",
		ProviderVendor:  "unknown_vendor",
		ProviderType:    ProviderTypeLLM,
		EncryptedAPIKey: "encrypted-key",
		Model:           "some-model",
	}
	factory := NewFactory(&fakeResolver{resolved: resolved}, &fakeDecrypter{plain: "plain-key"})

	_, _, err := factory.Build(ctx, "chat.default")
	if !errors.Is(err, errno.ErrLLMAdapterUnsupported) {
		t.Fatalf("Build() error = %v, want ErrLLMAdapterUnsupported", err)
	}
}

func TestBindToolsRequiresToolCallingCapability(t *testing.T) {
	chatModel := &fakeChatModel{}
	tool := &schema.ToolInfo{Name: "search", Desc: "Search"}

	_, err := BindTools(chatModel, &ResolvedModel{Capabilities: []string{}}, []*schema.ToolInfo{tool})
	if !errors.Is(err, errno.ErrLLMModelCapabilityUnsupported) {
		t.Fatalf("BindTools() error = %v, want ErrLLMModelCapabilityUnsupported", err)
	}
	if chatModel.withToolsCalled {
		t.Fatalf("BindTools() called WithTools for unsupported model")
	}

	withTools, err := BindTools(chatModel, &ResolvedModel{Capabilities: []string{CapabilityToolCalling}}, []*schema.ToolInfo{tool})
	if err != nil {
		t.Fatalf("BindTools() supported error = %v", err)
	}
	if withTools != chatModel {
		t.Fatalf("BindTools() model mismatch")
	}
	if !chatModel.withToolsCalled {
		t.Fatalf("BindTools() did not call WithTools")
	}
	if len(chatModel.tools) != 1 || chatModel.tools[0].Name != "search" {
		t.Fatalf("BindTools() tools = %#v, want search tool", chatModel.tools)
	}
}

type fakeResolver struct {
	resolved *ResolvedModel
	err      error
	gotRef   string
}

func (f *fakeResolver) ResolveModel(ctx context.Context, modelRef string) (*ResolvedModel, error) {
	f.gotRef = modelRef
	return f.resolved, f.err
}

type fakeDecrypter struct {
	plain         string
	err           error
	gotCiphertext string
}

func (f *fakeDecrypter) Decrypt(ctx context.Context, ciphertext string) (string, error) {
	f.gotCiphertext = ciphertext
	return f.plain, f.err
}

type fakeBuilder struct {
	chatModel   einomodel.ToolCallingChatModel
	err         error
	gotResolved *ResolvedModel
	gotAPIKey   string
}

func (f *fakeBuilder) Build(ctx context.Context, resolved *ResolvedModel, apiKey string) (einomodel.ToolCallingChatModel, error) {
	f.gotResolved = resolved
	f.gotAPIKey = apiKey
	return f.chatModel, f.err
}

type fakeChatModel struct {
	withToolsCalled bool
	tools           []*schema.ToolInfo
}

func (f *fakeChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.Message, error) {
	return &schema.Message{}, nil
}

func (f *fakeChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

func (f *fakeChatModel) WithTools(tools []*schema.ToolInfo) (einomodel.ToolCallingChatModel, error) {
	f.withToolsCalled = true
	f.tools = tools
	return f, nil
}
