package handler

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/ut"

	llmbiz "github.com/castlexu/micro-service/services/llm/biz"
)

type fakeGenerateService struct {
	resp      *llmbiz.GenerateResp
	err       error
	req       llmbiz.GenerateReq
	streamReq llmbiz.GenerateReq
}

func (s *fakeGenerateService) Generate(_ context.Context, req llmbiz.GenerateReq) (*llmbiz.GenerateResp, error) {
	s.req = req
	return s.resp, s.err
}

func (s *fakeGenerateService) Stream(_ context.Context, req llmbiz.GenerateReq) (<-chan llmbiz.StreamEvent, error) {
	s.streamReq = req
	ch := make(chan llmbiz.StreamEvent)
	close(ch)
	return ch, nil
}

func TestGenerateHandlerGenerateReturnsResponse(t *testing.T) {
	service := &fakeGenerateService{resp: &llmbiz.GenerateResp{
		RequestID: "llmreq_test",
		Message:   llmbiz.Message{Role: "assistant", Content: "hello"},
		Usage:     llmbiz.Usage{TotalTokens: 3},
		ModelRef:  "deepseek/deepseek-chat",
	}}
	handler := NewGenerateHandler(service)
	body := `{"model_ref":"deepseek/deepseek-chat","messages":[{"role":"user","content":"hi"}]}`
	ctx := ut.CreateUtRequestContext(
		http.MethodPost,
		"/api/v1/llm/generate",
		&ut.Body{Body: strings.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)

	handler.Generate(context.Background(), ctx)

	if service.req.ModelRef != "deepseek/deepseek-chat" {
		t.Fatalf("model_ref = %q", service.req.ModelRef)
	}
	if got := ctx.Response.StatusCode(); got != http.StatusOK {
		t.Fatalf("status = %d", got)
	}
	if !bytes.Contains(ctx.Response.Body(), []byte(`"request_id":"llmreq_test"`)) {
		t.Fatalf("response body = %s", ctx.Response.Body())
	}
}

func TestGenerateHandlerGenerateBindsTrustedMetadataFromHeaders(t *testing.T) {
	service := &fakeGenerateService{resp: &llmbiz.GenerateResp{
		RequestID: "llmreq_test",
		Message:   llmbiz.Message{Role: "assistant", Content: "hello"},
		ModelRef:  "deepseek/deepseek-chat",
	}}
	handler := NewGenerateHandler(service)
	body := `{"model_ref":"deepseek/deepseek-chat","caller":"body","user_id":"body-user","tenant_id":"body-tenant","messages":[{"role":"user","content":"hi"}]}`
	ctx := ut.CreateUtRequestContext(
		http.MethodPost,
		"/api/v1/llm/generate",
		&ut.Body{Body: strings.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
		ut.Header{Key: "X-Caller", Value: "edge-api"},
		ut.Header{Key: "X-User-ID", Value: "server-user"},
		ut.Header{Key: "X-Tenant-ID", Value: "server-tenant"},
	)

	handler.Generate(context.Background(), ctx)

	if service.req.Caller != "edge-api" || service.req.UserID != "server-user" || service.req.TenantID != "server-tenant" {
		t.Fatalf("metadata = caller:%q user:%q tenant:%q", service.req.Caller, service.req.UserID, service.req.TenantID)
	}
}

func TestGenerateHandlerStreamBindsTrustedMetadataFromHeaders(t *testing.T) {
	service := &fakeGenerateService{}
	handler := NewGenerateHandler(service)
	body := `{"model_ref":"deepseek/deepseek-chat","caller":"body","user_id":"body-user","tenant_id":"body-tenant","messages":[{"role":"user","content":"hi"}]}`
	ctx := ut.CreateUtRequestContext(
		http.MethodPost,
		"/api/v1/llm/stream",
		&ut.Body{Body: strings.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
		ut.Header{Key: "X-Caller", Value: "edge-api"},
		ut.Header{Key: "X-User-ID", Value: "server-user"},
		ut.Header{Key: "X-Tenant-ID", Value: "server-tenant"},
	)

	handler.Stream(context.Background(), ctx)

	if service.streamReq.Caller != "edge-api" || service.streamReq.UserID != "server-user" || service.streamReq.TenantID != "server-tenant" {
		t.Fatalf("metadata = caller:%q user:%q tenant:%q", service.streamReq.Caller, service.streamReq.UserID, service.streamReq.TenantID)
	}
}

func TestWriteSSEEventFormatsNamedEvent(t *testing.T) {
	var buf bytes.Buffer
	err := writeSSEEvent(&buf, llmbiz.StreamEvent{
		Type:      llmbiz.StreamEventContentDelta,
		RequestID: "llmreq_test",
		Content:   "hello",
	})
	if err != nil {
		t.Fatalf("writeSSEEvent() error = %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "event: content_delta\n") {
		t.Fatalf("missing event name: %q", got)
	}
	if !strings.Contains(got, `"content":"hello"`) || !strings.HasSuffix(got, "\n\n") {
		t.Fatalf("unexpected SSE frame: %q", got)
	}
}
