package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"

	"github.com/castlexu/micro-service/pkg/logger"
)

// ModelProxy 将 /api/v1/admin/models/* 转发到 model service。
type ModelProxy struct {
	modelBaseURL string
	httpClient   *http.Client
}

// NewModelProxy 构造 ModelProxy。modelAddr 示例："127.0.0.1:38083"。
func NewModelProxy(modelAddr string) *ModelProxy {
	return &ModelProxy{
		modelBaseURL: "http://" + modelAddr,
		// chat 接口 LLM 响应可能需要 120s，proxy 超时需覆盖该时间
		httpClient: &http.Client{Timeout: 150 * time.Second},
	}
}

// ProxyModels 转发 /api/v1/admin/models/* → model service /api/v1/model/*
func (p *ModelProxy) ProxyModels(c context.Context, ctx *app.RequestContext) {
	origPath := string(ctx.Path())
	const prefix = "/api/v1/admin/models"
	upstream := "/api/v1/model"
	if len(origPath) > len(prefix) {
		upstream += origPath[len(prefix):]
	}

	targetURL := p.modelBaseURL + upstream
	if qs := string(ctx.URI().QueryString()); qs != "" {
		targetURL += "?" + qs
	}

	// 必须先把 body 读进内存缓冲，Hertz RequestBodyStream 不能直接作为
	// http.NewRequest body（netpoll stream 不实现 io.ReadCloser 且 Content-Length=-1）
	bodyBytes := ctx.Request.Body()
	var bodyReader io.Reader
	if len(bodyBytes) > 0 {
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(c, string(ctx.Method()), targetURL, bodyReader)
	if err != nil {
		logger.Ctx(c).Error("model proxy: build request failed", zap.Error(err))
		ctx.JSON(http.StatusBadGateway, map[string]any{"code": 10007, "message": "model service unavailable"})
		return
	}

	ctx.Request.Header.VisitAll(func(k, v []byte) {
		key := string(k)
		if key == "Host" || key == "host" {
			return
		}
		req.Header.Set(key, string(v))
	})
	if len(bodyBytes) > 0 {
		req.ContentLength = int64(len(bodyBytes))
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		logger.Ctx(c).Error("model proxy: upstream failed", zap.Error(err), zap.String("url", targetURL))
		msg := fmt.Sprintf("model service error: %v", err)
		if isTimeout(err) {
			msg = "AI 模型请求超时，请稍后重试"
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
	ctx.Write(body) //nolint:errcheck
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

// ProxyStream 转发 SSE 流式请求，直接 pipe 上游 text/event-stream 响应给客户端。
// 不缓冲响应 body，确保 token 实时到达。
func (p *ModelProxy) ProxyStream(c context.Context, ctx *app.RequestContext) {
	origPath := string(ctx.Path())
	const prefix = "/api/v1/admin/models"
	upstream := "/api/v1/model"
	if len(origPath) > len(prefix) {
		upstream += origPath[len(prefix):]
	}

	targetURL := p.modelBaseURL + upstream

	bodyBytes := ctx.Request.Body()
	var bodyReader io.Reader
	if len(bodyBytes) > 0 {
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(c, http.MethodPost, targetURL, bodyReader)
	if err != nil {
		logger.Ctx(c).Error("model stream proxy: build request failed", zap.Error(err))
		ctx.JSON(http.StatusBadGateway, map[string]any{"code": 10007, "message": "model service unavailable"})
		return
	}
	ctx.Request.Header.VisitAll(func(k, v []byte) {
		key := string(k)
		if key == "Host" || key == "host" {
			return
		}
		req.Header.Set(key, string(v))
	})
	if len(bodyBytes) > 0 {
		req.ContentLength = int64(len(bodyBytes))
	}

	// 流式请求不设全局超时，依赖 ctx 取消
	streamClient := &http.Client{}
	resp, err := streamClient.Do(req)
	if err != nil {
		logger.Ctx(c).Error("model stream proxy: upstream failed", zap.Error(err))
		ctx.JSON(http.StatusBadGateway, map[string]any{"code": 10007, "message": fmt.Sprintf("model service error: %v", err)})
		return
	}

	// 透传上游响应头（含 Content-Type: text/event-stream, X-Accel-Buffering: no）
	ctx.SetStatusCode(resp.StatusCode)
	for k, vs := range resp.Header {
		for _, v := range vs {
			ctx.Header(k, v)
		}
	}

	// 直接 pipe：不缓冲，上游每写一个 SSE event，框架立即推给客户端
	ctx.Response.SetBodyStream(resp.Body, -1)
}
