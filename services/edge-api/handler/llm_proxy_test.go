package handler

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/ut"
)

func TestLLMProxyProxyLLMRewritesAdminPathAndForwardsRequest(t *testing.T) {
	var captured *http.Request
	proxy := &LLMProxy{
		baseURL: func(context.Context) (string, error) {
			return "http://llm.test", nil
		},
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				captured = req
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("read upstream body: %v", err)
				}
				if got := string(body); got != `{"name":"openai"}` {
					t.Fatalf("upstream body = %q", got)
				}
				return &http.Response{
					StatusCode: http.StatusCreated,
					Header:     http.Header{"Content-Type": []string{"application/json"}, "X-Upstream": []string{"ok"}},
					Body:       io.NopCloser(strings.NewReader(`{"code":0}`)),
				}, nil
			}),
		},
	}
	body := `{"name":"openai"}`
	ctx := ut.CreateUtRequestContext(
		http.MethodPost,
		"/api/v1/admin/llm/providers?workspace=default",
		&ut.Body{Body: strings.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	ctx.Request.Header.Set("Authorization", "Bearer token")
	ctx.Request.Header.Set("Host", "edge.test")
	ctx.Request.Header.Set("X-Caller", "attacker")
	ctx.Request.Header.Set("X-User-ID", "attacker-user")
	ctx.Request.Header.Set("X-Tenant-ID", "attacker-tenant")
	ctx.Set("auth_user_id", "server-user")
	ctx.Set("auth_tenant_id", "server-tenant")

	proxy.ProxyLLM(context.Background(), ctx)

	if captured == nil {
		t.Fatal("upstream was not called")
	}
	if got := captured.Method; got != http.MethodPost {
		t.Fatalf("method = %s, want POST", got)
	}
	if got := captured.URL.String(); got != "http://llm.test/api/v1/llm/providers?workspace=default" {
		t.Fatalf("url = %s", got)
	}
	if got := captured.Header.Get("Authorization"); got != "Bearer token" {
		t.Fatalf("authorization header = %q", got)
	}
	if got := captured.Header.Get("X-Caller"); got != "edge-api" {
		t.Fatalf("X-Caller = %q", got)
	}
	if got := captured.Header.Get("X-User-ID"); got != "server-user" {
		t.Fatalf("X-User-ID = %q", got)
	}
	if got := captured.Header.Get("X-Tenant-ID"); got != "server-tenant" {
		t.Fatalf("X-Tenant-ID = %q", got)
	}
	if got := captured.Host; got != "llm.test" {
		t.Fatalf("upstream host = %q", got)
	}
	if got := captured.ContentLength; got != int64(len(body)) {
		t.Fatalf("content length = %d, want %d", got, len(body))
	}
	if got := ctx.Response.StatusCode(); got != http.StatusCreated {
		t.Fatalf("response status = %d", got)
	}
	if got := string(ctx.Response.Body()); got != `{"code":0}` {
		t.Fatalf("response body = %q", got)
	}
	if got := string(ctx.Response.Header.Peek("X-Upstream")); got != "ok" {
		t.Fatalf("response header X-Upstream = %q", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
