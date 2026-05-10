// Package httpclient 提供轻量出站 HTTP 封装，支持普通 JSON 请求和 SSE 流式读取。
package httpclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client 是出站 HTTP 客户端。
type Client struct {
	hc      *http.Client
	baseURL string
	headers map[string]string
}

// New 构造 Client。baseURL 会拼接在每次请求路径前；timeout <= 0 则使用 30s。
func New(baseURL string, timeout time.Duration, headers map[string]string) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	h := make(map[string]string, len(headers))
	for k, v := range headers {
		h[k] = v
	}
	return &Client{
		hc:      &http.Client{Timeout: timeout},
		baseURL: baseURL,
		headers: h,
	}
}

// Do 发起 HTTP 请求并将响应体反序列化到 dst（dst 为 nil 则忽略 body）。
// method 为 "GET"/"POST" 等，path 会拼接在 baseURL 后，body 可为 nil。
func (c *Client) Do(ctx context.Context, method, path string, body any, dst any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("httpclient: marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("httpclient: new request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("httpclient: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("httpclient: upstream %d: %s", resp.StatusCode, string(raw))
	}
	if dst == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("httpclient: decode response: %w", err)
	}
	return nil
}

// DoStream 发起流式请求（适用于 SSE），返回未关闭的 *http.Response。
// 调用方负责关闭 resp.Body。
func (c *Client) DoStream(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("httpclient: marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("httpclient: new request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "text/event-stream")
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	// 流式请求不设全局 timeout，依赖 ctx 取消
	streamClient := &http.Client{}
	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpclient: do stream request: %w", err)
	}
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("httpclient: upstream %d: %s", resp.StatusCode, string(raw))
	}
	return resp, nil
}

// ReadSSELines 从 SSE 响应体中逐行读取 "data: ..." 内容，
// 每读到一行非空 data 就调用 fn(data)；遇到 "[DONE]" 停止。
// fn 返回 error 时立即终止并返回该 error。
func ReadSSELines(r io.Reader, fn func(data string) error) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		// SSE 格式：以 "data: " 为前缀
		const prefix = "data: "
		if len(line) < len(prefix) || line[:len(prefix)] != prefix {
			continue
		}
		data := line[len(prefix):]
		if data == "[DONE]" {
			return nil
		}
		if err := fn(data); err != nil {
			return err
		}
	}
	return scanner.Err()
}
