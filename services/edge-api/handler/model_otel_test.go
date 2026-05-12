package handler

import (
	"context"
	"net/http"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestStartModelProxyRequestCreatesClientSpanAndInjectsTraceparent(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := trace.NewTracerProvider(trace.WithSpanProcessor(sr))
	oldTP := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(oldTP)
	})

	ctx, parent := otel.Tracer("test").Start(context.Background(), "edge root")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://model:8080/api/v1/model/chat", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	_, finish := startModelProxyRequest(ctx, req, "/api/v1/model/chat", false)
	if got := req.Header.Get("traceparent"); got == "" {
		t.Fatal("traceparent header was not injected")
	}
	finish(http.StatusOK, nil)
	parent.End()

	spans := sr.Ended()
	if len(spans) != 2 {
		t.Fatalf("ended spans = %d, want 2", len(spans))
	}
	var proxySpan trace.ReadOnlySpan
	for _, span := range spans {
		if span.Name() == "HTTP POST /api/v1/model/chat" {
			proxySpan = span
			break
		}
	}
	if proxySpan == nil {
		t.Fatalf("proxy span not found: %#v", spans)
	}
	if proxySpan.SpanContext().TraceID() != parent.SpanContext().TraceID() {
		t.Fatal("proxy span did not use parent trace")
	}
	if proxySpan.Parent().SpanID() != parent.SpanContext().SpanID() {
		t.Fatal("proxy span parent is not the incoming edge span")
	}
}
