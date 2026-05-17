package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"

	"github.com/castlexu/micro-service/pkg/cloudwego"
	"github.com/castlexu/micro-service/pkg/logger"
	edgemw "github.com/castlexu/micro-service/services/edge-api/middleware"
)

const (
	llmHeaderCaller   = "X-Caller"
	llmHeaderUserID   = "X-User-ID"
	llmHeaderTenantID = "X-Tenant-ID"
	edgeLLMCaller     = "edge-api"
	authTenantIDKey   = "auth_tenant_id"
)

// LLMProxy 将 /api/v1/admin/llm/* 转发到 llm service。
type LLMProxy struct {
	resolver   *cloudwego.HertzServiceResolver
	httpClient *http.Client
	baseURL    func(context.Context) (string, error)
}

// NewLLMProxy 构造 LLMProxy。
func NewLLMProxy(resolver *cloudwego.HertzServiceResolver) *LLMProxy {
	return &LLMProxy{
		resolver: resolver,
		// LLM 响应可能需要 120s，proxy 超时需覆盖该时间
		httpClient: &http.Client{Timeout: 150 * time.Second},
		baseURL:    resolver.BaseURL,
	}
}

// ProxyLLM 转发 /api/v1/admin/llm/* → llm service /api/v1/llm/*
func (p *LLMProxy) ProxyLLM(c context.Context, ctx *app.RequestContext) {
	upstream := llmUpstreamPath(string(ctx.Path()))
	llmBaseURL, err := p.resolveBaseURL(c)
	if err != nil {
		logger.Ctx(c).Error("llm proxy: resolve upstream failed", zap.Error(err))
		ctx.JSON(http.StatusBadGateway, map[string]any{"code": 10007, "message": "llm service unavailable"})
		return
	}
	targetURL := llmBaseURL + upstream
	if qs := string(ctx.URI().QueryString()); qs != "" {
		targetURL += "?" + qs
	}

	bodyBytes := ctx.Request.Body()
	var bodyReader io.Reader
	if len(bodyBytes) > 0 {
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(c, string(ctx.Method()), targetURL, bodyReader)
	if err != nil {
		logger.Ctx(c).Error("llm proxy: build request failed", zap.Error(err))
		ctx.JSON(http.StatusBadGateway, map[string]any{"code": 10007, "message": "llm service unavailable"})
		return
	}
	copyProxyHeaders(ctx, req)
	applyTrustedLLMHeaders(ctx, req)
	if len(bodyBytes) > 0 {
		req.ContentLength = int64(len(bodyBytes))
	}

	proxyCtx, finishProxy := startLLMProxyRequest(c, req, upstream, false)
	req = req.WithContext(proxyCtx)
	resp, err := p.client().Do(req)
	if err != nil {
		finishProxy(0, err)
		logger.Ctx(c).Error("llm proxy: upstream failed", zap.Error(err), zap.String("url", targetURL))
		msg := fmt.Sprintf("llm service error: %v", err)
		if isTimeout(err) {
			msg = "LLM 请求超时，请稍后重试"
		}
		ctx.JSON(http.StatusGatewayTimeout, map[string]any{"code": 10007, "message": msg})
		return
	}
	defer resp.Body.Close()

	ctx.SetStatusCode(resp.StatusCode)
	for k, vs := range resp.Header {
		for _, v := range vs {
			ctx.Header(k, v)
		}
	}
	body, _ := io.ReadAll(resp.Body)
	finishProxy(resp.StatusCode, nil)
	ctx.Write(body) //nolint:errcheck
}

// ProxyStream 转发 SSE 流式请求，直接 pipe 上游 text/event-stream 响应给客户端。
func (p *LLMProxy) ProxyStream(c context.Context, ctx *app.RequestContext) {
	upstream := llmUpstreamPath(string(ctx.Path()))
	llmBaseURL, err := p.resolveBaseURL(c)
	if err != nil {
		logger.Ctx(c).Error("llm stream proxy: resolve upstream failed", zap.Error(err))
		ctx.JSON(http.StatusBadGateway, map[string]any{"code": 10007, "message": "llm service unavailable"})
		return
	}
	targetURL := llmBaseURL + upstream
	if qs := string(ctx.URI().QueryString()); qs != "" {
		targetURL += "?" + qs
	}

	bodyBytes := ctx.Request.Body()
	var bodyReader io.Reader
	if len(bodyBytes) > 0 {
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(c, string(ctx.Method()), targetURL, bodyReader)
	if err != nil {
		logger.Ctx(c).Error("llm stream proxy: build request failed", zap.Error(err))
		ctx.JSON(http.StatusBadGateway, map[string]any{"code": 10007, "message": "llm service unavailable"})
		return
	}
	copyProxyHeaders(ctx, req)
	applyTrustedLLMHeaders(ctx, req)
	if len(bodyBytes) > 0 {
		req.ContentLength = int64(len(bodyBytes))
	}

	proxyCtx, finishProxy := startLLMProxyRequest(c, req, upstream, true)
	req = req.WithContext(proxyCtx)
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		finishProxy(0, err)
		logger.Ctx(c).Error("llm stream proxy: upstream failed", zap.Error(err))
		ctx.JSON(http.StatusBadGateway, map[string]any{"code": 10007, "message": fmt.Sprintf("llm service error: %v", err)})
		return
	}

	ctx.SetStatusCode(resp.StatusCode)
	for k, vs := range resp.Header {
		for _, v := range vs {
			ctx.Header(k, v)
		}
	}
	ctx.Response.SetBodyStream(&llmProxyBody{ReadCloser: resp.Body, finish: finishProxy, status: resp.StatusCode}, -1)
}

func (p *LLMProxy) resolveBaseURL(ctx context.Context) (string, error) {
	if p.baseURL != nil {
		return p.baseURL(ctx)
	}
	return p.resolver.BaseURL(ctx)
}

func (p *LLMProxy) client() *http.Client {
	if p.httpClient != nil {
		return p.httpClient
	}
	return &http.Client{Timeout: 150 * time.Second}
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	type timeouter interface{ Timeout() bool }
	if t, ok := err.(timeouter); ok && t.Timeout() {
		return true
	}
	return false
}

func llmUpstreamPath(origPath string) string {
	const prefix = "/api/v1/admin/llm"
	upstream := "/api/v1/llm"
	if len(origPath) > len(prefix) {
		upstream += origPath[len(prefix):]
	}
	return upstream
}

func copyProxyHeaders(ctx *app.RequestContext, req *http.Request) {
	ctx.Request.Header.VisitAll(func(k, v []byte) {
		key := string(k)
		if skipProxyHeader(key) {
			return
		}
		req.Header.Set(key, string(v))
	})
}

func skipProxyHeader(key string) bool {
	return strings.EqualFold(key, "Host") ||
		strings.EqualFold(key, llmHeaderCaller) ||
		strings.EqualFold(key, llmHeaderUserID) ||
		strings.EqualFold(key, llmHeaderTenantID)
}

func applyTrustedLLMHeaders(ctx *app.RequestContext, req *http.Request) {
	req.Header.Set(llmHeaderCaller, edgeLLMCaller)
	if userID := strings.TrimSpace(edgemw.GetUserID(ctx)); userID != "" {
		req.Header.Set(llmHeaderUserID, userID)
	} else {
		req.Header.Del(llmHeaderUserID)
	}
	if tenantID := strings.TrimSpace(hertzContextString(ctx, authTenantIDKey)); tenantID != "" {
		req.Header.Set(llmHeaderTenantID, tenantID)
	} else {
		req.Header.Del(llmHeaderTenantID)
	}
}

func hertzContextString(ctx *app.RequestContext, key string) string {
	v, ok := ctx.Get(key)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}
