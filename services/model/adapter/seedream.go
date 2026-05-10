package adapter

import (
	"context"
	"fmt"

	"github.com/castlexu/micro-service/pkg/errno"
)

// ImageRequest 是文生图请求。
type ImageRequest struct {
	Prompt string `json:"prompt"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// ImageResult 是文生图结果。
type ImageResult struct {
	URL string `json:"url"`
}

// ImageAdapter 是图像生成适配器接口。
type ImageAdapter interface {
	Generate(ctx context.Context, req ImageRequest) (*ImageResult, error)
}

// seedreamAdapter 是 Seedream 图像生成适配器（占位实现，待正式 API 文档后补全）。
type seedreamAdapter struct {
	baseURL string
	apiKey  string
}

// NewSeedream 构造 Seedream 适配器。
func NewSeedream(baseURL, apiKey string) ImageAdapter {
	return &seedreamAdapter{baseURL: baseURL, apiKey: apiKey}
}

func (a *seedreamAdapter) Generate(_ context.Context, req ImageRequest) (*ImageResult, error) {
	// TODO: Seedream 正式 API 接入后替换此占位实现
	_ = req
	_ = fmt.Sprintf("baseURL=%s", a.baseURL)
	return nil, errno.ErrNotImplemented.WithMessage("seedream adapter not yet implemented")
}
