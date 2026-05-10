package biz_test

import (
	"context"
	"testing"

	"github.com/castlexu/micro-service/services/model/adapter"
	"github.com/castlexu/micro-service/services/model/biz"
)

// mockLLMAdapter 是测试用 mock LLM 适配器。
type mockLLMAdapter struct {
	response string
	err      error
}

func (m *mockLLMAdapter) Chat(_ context.Context, _ adapter.ChatRequest) (string, error) {
	return m.response, m.err
}

func (m *mockLLMAdapter) ChatStream(_ context.Context, _ adapter.ChatRequest) (<-chan adapter.ChatChunk, error) {
	ch := make(chan adapter.ChatChunk, 1)
	ch <- adapter.ChatChunk{Content: m.response, Done: true}
	close(ch)
	return ch, m.err
}

// TestChatBiz_MockAdapter 验证 ChatBiz.Chat 返回 mock adapter 的固定字符串。
func TestChatBiz_MockAdapter(t *testing.T) {
	cb := biz.NewChatBiz(nil)

	content, err := cb.ChatWithAdapter(context.Background(), &mockLLMAdapter{response: "hello from mock"}, []biz.ChatMessage{
		{Role: "user", Content: "hi"},
	})
	if err != nil {
		t.Fatalf("ChatWithAdapter failed: %v", err)
	}
	if content != "hello from mock" {
		t.Errorf("expected 'hello from mock', got %q", content)
	}
}
