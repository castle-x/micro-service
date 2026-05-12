package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestOpenAIAdapterChatCreatesLLMSpanWithoutSensitiveContent(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := trace.NewTracerProvider(trace.WithSpanProcessor(sr))
	oldTP := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(oldTP)
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret-api-key" {
			t.Fatalf("Authorization = %q, want bearer key", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "completion-secret"}},
			},
			"usage": map[string]int{
				"prompt_tokens":     9,
				"completion_tokens": 4,
				"total_tokens":      13,
			},
		})
	}))
	defer server.Close()

	adp := newOpenAI("deepseek", server.URL, "secret-api-key", "deepseek-chat")
	got, err := adp.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "prompt-secret"}},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if got != "completion-secret" {
		t.Fatalf("Chat = %q, want completion-secret", got)
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(spans))
	}
	span := spans[0]
	if span.Name() != "LLM chat.completions" {
		t.Fatalf("span name = %q, want LLM chat.completions", span.Name())
	}
	attrs := attrsByKey(span.Attributes())
	wantAttrs := map[attribute.Key]attribute.Value{
		"gen_ai.system":         attribute.StringValue("deepseek"),
		"gen_ai.request.model":  attribute.StringValue("deepseek-chat"),
		"gen_ai.operation.name": attribute.StringValue("chat.completions"),
		"stream":                attribute.BoolValue(false),
		"llm.token.input":       attribute.IntValue(9),
		"llm.token.output":      attribute.IntValue(4),
		"llm.token.total":       attribute.IntValue(13),
	}
	for key, want := range wantAttrs {
		if got := attrs[key]; got != want {
			t.Fatalf("attr %s = %v, want %v", key, got.AsInterface(), want.AsInterface())
		}
	}
	for key, val := range attrs {
		if s := fmt.Sprint(val.AsInterface()); s == "prompt-secret" || s == "completion-secret" || s == "secret-api-key" {
			t.Fatalf("sensitive value leaked in attr %s", key)
		}
	}
}

func TestOpenAIAdapterChatStreamKeepsLLMSpanOpenUntilDoneAndRecordsUsage(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := trace.NewTracerProvider(trace.WithSpanProcessor(sr))
	oldTP := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(oldTP)
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(t, w, map[string]any{
			"choices": []map[string]any{
				{"delta": map[string]any{"content": "stream-secret"}},
			},
		})
		writeSSE(t, w, map[string]any{
			"choices": []map[string]any{
				{"delta": map[string]any{}, "finish_reason": "stop"},
			},
		})
		writeSSE(t, w, map[string]any{
			"choices": []any{},
			"usage": map[string]int{
				"prompt_tokens":     6,
				"completion_tokens": 2,
				"total_tokens":      8,
			},
		})
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	adp := newOpenAI("openai", server.URL, "secret-api-key", "gpt-test")
	ch, err := adp.ChatStream(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "prompt-secret"}},
	})
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}
	if len(sr.Ended()) != 0 {
		t.Fatal("stream span ended before the response channel was consumed")
	}
	for range ch {
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(spans))
	}
	attrs := attrsByKey(spans[0].Attributes())
	wantAttrs := map[attribute.Key]attribute.Value{
		"gen_ai.system":         attribute.StringValue("openai"),
		"gen_ai.request.model":  attribute.StringValue("gpt-test"),
		"gen_ai.operation.name": attribute.StringValue("chat.completions"),
		"stream":                attribute.BoolValue(true),
		"llm.token.input":       attribute.IntValue(6),
		"llm.token.output":      attribute.IntValue(2),
		"llm.token.total":       attribute.IntValue(8),
	}
	for key, want := range wantAttrs {
		if got := attrs[key]; got != want {
			t.Fatalf("attr %s = %v, want %v", key, got.AsInterface(), want.AsInterface())
		}
	}
	if _, ok := attrs["llm.first_token.duration_ms"]; !ok {
		t.Fatal("missing llm.first_token.duration_ms attr")
	}
	for key, val := range attrs {
		if s := fmt.Sprint(val.AsInterface()); s == "prompt-secret" || s == "stream-secret" || s == "secret-api-key" {
			t.Fatalf("sensitive value leaked in attr %s", key)
		}
	}
}

func TestOpenAIAdapterChatStreamAttachesUsageWhenFinishPrecedesUsageChunk(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %s, want /v1/chat/completions", r.URL.Path)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(t, w, map[string]any{
			"choices": []map[string]any{
				{"delta": map[string]any{"content": "hello"}},
			},
		})
		writeSSE(t, w, map[string]any{
			"choices": []map[string]any{
				{"delta": map[string]any{}, "finish_reason": "stop"},
			},
		})
		writeSSE(t, w, map[string]any{
			"choices": []any{},
			"usage": map[string]int{
				"prompt_tokens":     7,
				"completion_tokens": 3,
				"total_tokens":      10,
			},
		})
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	adp := newOpenAI("openai", server.URL, "test-key", "test-model")
	ch, err := adp.ChatStream(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}

	var chunks []ChatChunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	if len(chunks) != 2 {
		t.Fatalf("len(chunks) = %d, want 2: %#v", len(chunks), chunks)
	}
	if chunks[0].Content != "hello" || chunks[0].Done {
		t.Fatalf("first chunk = %#v, want content chunk", chunks[0])
	}

	var doneCount int
	var done ChatChunk
	for _, chunk := range chunks {
		if chunk.Done {
			doneCount++
			done = chunk
		}
	}
	if doneCount != 1 {
		t.Fatalf("done count = %d, want 1: %#v", doneCount, chunks)
	}
	if done.PromptTokens != 7 || done.CompletionTokens != 3 || done.TotalTokens != 10 {
		t.Fatalf("done usage = (%d, %d, %d), want (7, 3, 10)", done.PromptTokens, done.CompletionTokens, done.TotalTokens)
	}
}

func TestOpenAIAdapterChatStreamRequestsUsage(t *testing.T) {
	requestsUsage := make(chan bool, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		var body struct {
			StreamOptions *StreamOptions `json:"stream_options"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		requestsUsage <- body.StreamOptions != nil && body.StreamOptions.IncludeUsage

		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	adp := newOpenAI("openai", server.URL, "test-key", "test-model")
	ch, err := adp.ChatStream(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}
	for range ch {
	}
	if !<-requestsUsage {
		t.Fatal("upstream request did not include stream_options.include_usage=true")
	}
}

func TestOpenAIAdapterChatStreamAttachesUsageFromFinishChunk(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(t, w, map[string]any{
			"choices": []map[string]any{
				{"delta": map[string]any{"content": "hello"}},
			},
		})
		writeSSE(t, w, map[string]any{
			"choices": []map[string]any{
				{"delta": map[string]any{}, "finish_reason": "stop"},
			},
			"usage": map[string]int{
				"prompt_tokens":     11,
				"completion_tokens": 5,
				"total_tokens":      16,
			},
		})
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	adp := newOpenAI("openai", server.URL, "test-key", "test-model")
	ch, err := adp.ChatStream(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}

	var doneCount int
	var done ChatChunk
	for chunk := range ch {
		if chunk.Done {
			doneCount++
			done = chunk
		}
	}
	if doneCount != 1 {
		t.Fatalf("done count = %d, want 1", doneCount)
	}
	if done.PromptTokens != 11 || done.CompletionTokens != 5 || done.TotalTokens != 16 {
		t.Fatalf("done usage = (%d, %d, %d), want (11, 5, 16)", done.PromptTokens, done.CompletionTokens, done.TotalTokens)
	}
}

func attrsByKey(attrs []attribute.KeyValue) map[attribute.Key]attribute.Value {
	out := make(map[attribute.Key]attribute.Value, len(attrs))
	for _, attr := range attrs {
		out[attr.Key] = attr.Value
	}
	return out
}

func writeSSE(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal SSE payload: %v", err)
	}
	fmt.Fprintf(w, "data: %s\n\n", raw)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}
