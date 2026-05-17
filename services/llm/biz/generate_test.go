package biz

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/castlexu/micro-service/pkg/errno"
	"github.com/castlexu/micro-service/services/llm/component"
	llmmodel "github.com/castlexu/micro-service/services/llm/dal/model"
)

type fakeChatModel struct {
	generateResp *schema.Message
	generateErr  error
	streamChunks []*schema.Message
	streamErr    error
	seenMessages []*schema.Message
	boundTools   []*schema.ToolInfo
}

func (m *fakeChatModel) Generate(_ context.Context, input []*schema.Message, _ ...einomodel.Option) (*schema.Message, error) {
	m.seenMessages = input
	return m.generateResp, m.generateErr
}

func (m *fakeChatModel) Stream(_ context.Context, input []*schema.Message, _ ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	m.seenMessages = input
	if m.streamErr != nil {
		return nil, m.streamErr
	}
	reader, writer := schema.Pipe[*schema.Message](len(m.streamChunks))
	go func() {
		defer writer.Close()
		for _, chunk := range m.streamChunks {
			if writer.Send(chunk, nil) {
				return
			}
		}
	}()
	return reader, nil
}

func (m *fakeChatModel) WithTools(tools []*schema.ToolInfo) (einomodel.ToolCallingChatModel, error) {
	m.boundTools = append([]*schema.ToolInfo(nil), tools...)
	return m, nil
}

type fakeGenerateFactory struct {
	model      *fakeChatModel
	resolved   *component.ResolvedModel
	seenRef    string
	buildCalls int
}

func (f *fakeGenerateFactory) Build(_ context.Context, modelRef string) (einomodel.ToolCallingChatModel, *component.ResolvedModel, error) {
	f.buildCalls++
	f.seenRef = modelRef
	return f.model, f.resolved, nil
}

func intPtr(v int) *int { return &v }

type requestLogFindCall struct {
	userID         string
	modelRef       string
	idempotencyKey string
}

type fakeRequestLogs struct {
	successful *llmmodel.RequestLog
	findCalls  []requestLogFindCall
	inserted   []*llmmodel.RequestLog
}

func (l *fakeRequestLogs) Insert(_ context.Context, log *llmmodel.RequestLog) error {
	l.inserted = append(l.inserted, log)
	return nil
}

func (l *fakeRequestLogs) FindSuccessful(_ context.Context, userID, modelRef, idempotencyKey string) (*llmmodel.RequestLog, error) {
	l.findCalls = append(l.findCalls, requestLogFindCall{userID: userID, modelRef: modelRef, idempotencyKey: idempotencyKey})
	if l.successful != nil {
		return l.successful, nil
	}
	return nil, errno.ErrNotFound
}

func TestGenerateReturnsAssistantUsageAndToolCalls(t *testing.T) {
	toolIndex := 0
	model := &fakeChatModel{
		generateResp: &schema.Message{
			Role:             schema.Assistant,
			Content:          "hello",
			ReasoningContent: "thinking",
			ToolCalls: []schema.ToolCall{{
				Index: &toolIndex,
				ID:    "call_1",
				Type:  "function",
				Function: schema.FunctionCall{
					Name:      "asset.get_asset",
					Arguments: `{"asset_id":"a1"}`,
				},
			}},
			ResponseMeta: &schema.ResponseMeta{
				FinishReason: "tool_calls",
				Usage: &schema.TokenUsage{
					PromptTokens:     12,
					CompletionTokens: 8,
					TotalTokens:      20,
				},
			},
		},
	}
	factory := &fakeGenerateFactory{
		model: model,
		resolved: &component.ResolvedModel{
			Ref:             "deepseek/deepseek-chat",
			Model:           "deepseek-chat",
			Capabilities:    []string{component.CapabilityToolCalling},
			MaxOutputTokens: 2048,
		},
	}
	biz := NewGenerateBiz(factory, nil)

	resp, err := biz.Generate(context.Background(), GenerateReq{
		ModelRef: "deepseek/deepseek-chat",
		Messages: []Message{{Role: "user", Content: "hi"}},
		Tools: []Tool{{
			Type: "function",
			Function: ToolFunction{
				Name:        "asset.get_asset",
				Description: "Get one asset by id",
				Parameters:  []byte(`{"type":"object","properties":{"asset_id":{"type":"string"}}}`),
			},
		}},
		MaxTokens: intPtr(128),
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if factory.seenRef != "deepseek/deepseek-chat" {
		t.Fatalf("factory saw model ref %q", factory.seenRef)
	}
	if len(model.seenMessages) != 1 || model.seenMessages[0].Role != schema.User || model.seenMessages[0].Content != "hi" {
		t.Fatalf("unexpected Eino messages: %#v", model.seenMessages)
	}
	if len(model.boundTools) != 1 || model.boundTools[0].Name != "asset.get_asset" {
		t.Fatalf("tool was not bound: %#v", model.boundTools)
	}
	if resp.Message.Content != "hello" || resp.Message.ReasoningContent != "thinking" {
		t.Fatalf("unexpected message: %#v", resp.Message)
	}
	if len(resp.Message.ToolCalls) != 1 || resp.Message.ToolCalls[0].Function.Arguments != `{"asset_id":"a1"}` {
		t.Fatalf("unexpected tool calls: %#v", resp.Message.ToolCalls)
	}
	if resp.Usage.TotalTokens != 20 || resp.FinishReason != "tool_calls" {
		t.Fatalf("unexpected usage/finish: %#v %q", resp.Usage, resp.FinishReason)
	}
}

func TestGenerateIdempotencyHitIsScopedToUserAndSkipsUpstream(t *testing.T) {
	cached := &GenerateResp{
		RequestID: "llmreq_cached",
		Message:   Message{Role: "assistant", Content: "cached"},
		ModelRef:  "deepseek/deepseek-chat",
	}
	raw, err := json.Marshal(cached)
	if err != nil {
		t.Fatalf("marshal cached response: %v", err)
	}
	logs := &fakeRequestLogs{successful: &llmmodel.RequestLog{ResponseJSON: string(raw)}}
	factory := &fakeGenerateFactory{}
	biz := NewGenerateBiz(factory, logs)

	resp, err := biz.Generate(context.Background(), GenerateReq{
		Caller:         "edge-api",
		UserID:         "user-a",
		TenantID:       "tenant-a",
		ModelRef:       "deepseek/deepseek-chat",
		IdempotencyKey: "idem-1",
		Messages:       []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.RequestID != "llmreq_cached" || resp.Message.Content != "cached" {
		t.Fatalf("unexpected cached response: %#v", resp)
	}
	if factory.buildCalls != 0 {
		t.Fatalf("upstream was called %d times", factory.buildCalls)
	}
	if len(logs.findCalls) != 1 || logs.findCalls[0].userID != "user-a" {
		t.Fatalf("FindSuccessful calls = %#v", logs.findCalls)
	}
}

func TestGenerateSkipsIdempotencyLookupWithoutUserID(t *testing.T) {
	logs := &fakeRequestLogs{}
	model := &fakeChatModel{generateResp: &schema.Message{Role: schema.Assistant, Content: "fresh"}}
	biz := NewGenerateBiz(&fakeGenerateFactory{
		model: model,
		resolved: &component.ResolvedModel{
			Ref:   "deepseek/deepseek-chat",
			Model: "deepseek-chat",
		},
	}, logs)

	_, err := biz.Generate(context.Background(), GenerateReq{
		ModelRef:       "deepseek/deepseek-chat",
		IdempotencyKey: "idem-1",
		Messages:       []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if len(logs.findCalls) != 0 {
		t.Fatalf("FindSuccessful should not be called without user_id: %#v", logs.findCalls)
	}
}

func TestGenerateRequestLogRecordsTrustedMetadata(t *testing.T) {
	logs := &fakeRequestLogs{}
	model := &fakeChatModel{generateResp: &schema.Message{
		Role:    schema.Assistant,
		Content: "hello",
		ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{
			PromptTokens:     1,
			CompletionTokens: 2,
			TotalTokens:      3,
		}},
	}}
	biz := NewGenerateBiz(&fakeGenerateFactory{
		model: model,
		resolved: &component.ResolvedModel{
			Ref:          "deepseek/deepseek-chat",
			Model:        "deepseek-chat",
			ProviderSlug: "deepseek",
		},
	}, logs)

	_, err := biz.Generate(context.Background(), GenerateReq{
		Caller:         "edge-api",
		UserID:         "user-a",
		TenantID:       "tenant-a",
		ModelRef:       "deepseek/deepseek-chat",
		IdempotencyKey: "idem-1",
		Messages:       []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if len(logs.inserted) != 1 {
		t.Fatalf("inserted logs = %#v", logs.inserted)
	}
	log := logs.inserted[0]
	if log.Caller != "edge-api" || log.UserID != "user-a" || log.TenantID != "tenant-a" {
		t.Fatalf("metadata = caller:%q user:%q tenant:%q", log.Caller, log.UserID, log.TenantID)
	}
	if log.ModelRef != "deepseek/deepseek-chat" || log.ProviderSlug != "deepseek" || log.Stream || log.Status != "success" || log.IdempotencyKey != "idem-1" {
		t.Fatalf("unexpected request log: %#v", log)
	}
}

func TestGenerateFailureRequestLogRecordsTrustedMetadata(t *testing.T) {
	logs := &fakeRequestLogs{}
	biz := NewGenerateBiz(&fakeGenerateFactory{
		model: &fakeChatModel{generateErr: io.ErrUnexpectedEOF},
		resolved: &component.ResolvedModel{
			Ref:          "deepseek/deepseek-chat",
			Model:        "deepseek-chat",
			ProviderSlug: "deepseek",
		},
	}, logs)

	_, err := biz.Generate(context.Background(), GenerateReq{
		Caller:         "edge-api",
		UserID:         "user-a",
		TenantID:       "tenant-a",
		ModelRef:       "deepseek/deepseek-chat",
		IdempotencyKey: "idem-1",
		Messages:       []Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if len(logs.inserted) != 1 {
		t.Fatalf("inserted logs = %#v", logs.inserted)
	}
	log := logs.inserted[0]
	if log.Caller != "edge-api" || log.UserID != "user-a" || log.TenantID != "tenant-a" {
		t.Fatalf("metadata = caller:%q user:%q tenant:%q", log.Caller, log.UserID, log.TenantID)
	}
	if log.ModelRef != "deepseek/deepseek-chat" || log.ProviderSlug != "deepseek" || log.Stream || log.Status != "failed" || log.IdempotencyKey != "idem-1" {
		t.Fatalf("unexpected request log: %#v", log)
	}
}

func TestGenerateUpstreamErrorIsRedacted(t *testing.T) {
	logs := &fakeRequestLogs{}
	biz := NewGenerateBiz(&fakeGenerateFactory{
		model: &fakeChatModel{generateErr: errors.New(`upstream Authorization: Bearer sk-live api_key=sk-other password: p@ss token "jwt-value"`)},
		resolved: &component.ResolvedModel{
			Ref:          "deepseek/deepseek-chat",
			Model:        "deepseek-chat",
			ProviderSlug: "deepseek",
		},
	}, logs)

	_, err := biz.Generate(context.Background(), GenerateReq{
		Caller:   "edge-api",
		UserID:   "user-a",
		ModelRef: "deepseek/deepseek-chat",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	for _, leaked := range []string{"sk-live", "sk-other", "p@ss", "jwt-value", "Bearer"} {
		if strings.Contains(err.Error(), leaked) {
			t.Fatalf("returned error leaked %q: %v", leaked, err)
		}
		if len(logs.inserted) == 1 && strings.Contains(logs.inserted[0].ErrorMessage, leaked) {
			t.Fatalf("request log leaked %q: %#v", leaked, logs.inserted[0])
		}
	}
}

func TestGenerateRejectsMaxTokensAboveModelLimit(t *testing.T) {
	factory := &fakeGenerateFactory{
		model: &fakeChatModel{},
		resolved: &component.ResolvedModel{
			Ref:             "deepseek/deepseek-chat",
			Model:           "deepseek-chat",
			MaxOutputTokens: 128,
		},
	}
	biz := NewGenerateBiz(factory, nil)

	_, err := biz.Generate(context.Background(), GenerateReq{
		ModelRef:  "deepseek/deepseek-chat",
		Messages:  []Message{{Role: "user", Content: "hi"}},
		MaxTokens: intPtr(129),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errno.ErrLLMInvalidMessage) {
		t.Fatalf("expected ErrLLMInvalidMessage, got %v", err)
	}
}

func TestStreamEmitsDeltasCompletedUsageAndDone(t *testing.T) {
	toolIndex := 0
	model := &fakeChatModel{
		streamChunks: []*schema.Message{
			{Role: schema.Assistant, ReasoningContent: "think"},
			{Role: schema.Assistant, Content: "hello"},
			{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{
				Index: &toolIndex,
				ID:    "call_1",
				Type:  "function",
				Function: schema.FunctionCall{
					Name:      "asset.get_asset",
					Arguments: `{"asset_id":`,
				},
			}}},
			{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{
				Index: &toolIndex,
				Function: schema.FunctionCall{
					Arguments: `"a1"}`,
				},
			}}},
			{Role: schema.Assistant, ResponseMeta: &schema.ResponseMeta{
				FinishReason: "tool_calls",
				Usage: &schema.TokenUsage{
					PromptTokens:     1,
					CompletionTokens: 2,
					TotalTokens:      3,
				},
			}},
		},
	}
	biz := NewGenerateBiz(&fakeGenerateFactory{
		model: model,
		resolved: &component.ResolvedModel{
			Ref:          "deepseek/deepseek-chat",
			Model:        "deepseek-chat",
			Capabilities: []string{component.CapabilityToolCalling},
		},
	}, nil)

	events, err := biz.Stream(context.Background(), GenerateReq{
		ModelRef: "deepseek/deepseek-chat",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var got []StreamEvent
	for event := range events {
		got = append(got, event)
	}
	if len(got) != 7 {
		t.Fatalf("expected 7 events, got %#v", got)
	}
	if got[0].Type != StreamEventReasoningDelta || got[0].Content != "think" {
		t.Fatalf("unexpected first event: %#v", got[0])
	}
	if got[1].Type != StreamEventContentDelta || got[1].Content != "hello" {
		t.Fatalf("unexpected content event: %#v", got[1])
	}
	if got[2].Type != StreamEventToolCallDelta || got[3].Type != StreamEventToolCallDelta {
		t.Fatalf("unexpected tool events: %#v", got)
	}
	if got[4].Type != StreamEventMessageCompleted || got[4].Message.ToolCalls[0].Function.Arguments != `{"asset_id":"a1"}` {
		t.Fatalf("unexpected completed event: %#v", got[4])
	}
	if got[5].Type != StreamEventUsage || got[5].Usage.TotalTokens != 3 {
		t.Fatalf("unexpected usage event: %#v", got[5])
	}
	if got[6].Type != StreamEventDone || got[6].FinishReason != "tool_calls" {
		t.Fatalf("unexpected done event: %#v", got[6])
	}
}

func TestStreamReturnsUpstreamErrorEvent(t *testing.T) {
	upstreamErr := io.ErrUnexpectedEOF
	biz := NewGenerateBiz(&fakeGenerateFactory{
		model: &fakeChatModel{streamErr: upstreamErr},
		resolved: &component.ResolvedModel{
			Ref:   "deepseek/deepseek-chat",
			Model: "deepseek-chat",
		},
	}, nil)

	events, err := biz.Stream(context.Background(), GenerateReq{
		ModelRef: "deepseek/deepseek-chat",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream() init error = %v", err)
	}
	event, ok := <-events
	if !ok {
		t.Fatal("expected one error event")
	}
	if event.Type != StreamEventError || event.Error == nil {
		t.Fatalf("unexpected event: %#v", event)
	}
}
