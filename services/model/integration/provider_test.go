//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"

	"github.com/castlexu/micro-service/pkg/db"
	"github.com/castlexu/micro-service/pkg/logger"
	mwhertz "github.com/castlexu/micro-service/pkg/middleware/hertz"
	mdlbiz "github.com/castlexu/micro-service/services/model/biz"
	mdlmongo "github.com/castlexu/micro-service/services/model/dal/mongo"
	mdlhandler "github.com/castlexu/micro-service/services/model/handler"
)

// startServer 启动 model service 测试实例，返回 base URL 和 cleanup 函数。
func startServer(t *testing.T) (baseURL string, cleanup func()) {
	t.Helper()

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		t.Skip("MONGO_URI not set, skipping integration test")
	}

	_ = logger.Init(logger.Options{Service: "model-test"})

	mongoClient, err := db.InitMongo(db.MongoConfig{URI: mongoURI, DBName: "platform_test"})
	if err != nil {
		t.Skipf("mongo connect failed (%v), skipping integration test", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	providerRepo := mdlmongo.NewProviderRepo(mongoClient)
	if err := providerRepo.EnsureIndexes(ctx, mongoClient); err != nil {
		t.Logf("warn: ensure indexes: %v", err)
	}

	encryptKey := []byte("integration-test-key-32bytesXXXX")[:32]
	providerBiz := mdlbiz.NewProviderBiz(providerRepo, encryptKey)
	chatBiz := mdlbiz.NewChatBiz(providerBiz)
	providerHandler := mdlhandler.NewProviderHandler(providerBiz)
	chatHandler := mdlhandler.NewChatHandler(chatBiz)

	// 找一个空闲端口
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	h := server.New(
		server.WithHostPorts(addr),
		server.WithExitWaitTime(0),
	)
	h.Use(mwhertz.Recovery())

	v1 := h.Group("/api/v1/model")
	{
		providers := v1.Group("/providers")
		providers.GET("", providerHandler.ListProviders)
		providers.POST("", providerHandler.CreateProvider)
		providers.PATCH("/:id/enabled", providerHandler.SetEnabled)
		providers.PATCH("/:id/api_key", providerHandler.UpdateAPIKey)
		v1.POST("/chat", chatHandler.Chat)
	}

	go func() { h.Spin() }()

	// 等待服务就绪
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://" + addr + "/api/v1/model/providers")
		if err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	cleanup = func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer shutCancel()
		h.Shutdown(shutCtx) //nolint:errcheck

		// 清理测试数据
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanCancel()
		_ = mongoClient.Database().Collection("model_providers").Drop(cleanCtx)

		closeCtx, closeCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer closeCancel()
		_ = mongoClient.Close(closeCtx)
	}

	return "http://" + addr, cleanup
}

func jsonPost(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s failed: %v", url, err)
	}
	return resp
}

func jsonPatch(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPatch, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH %s failed: %v", url, err)
	}
	return resp
}

func readJSON(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode response body: %v\nraw: %s", err, raw)
	}
	return m
}

// TestProviderCRUD 端到端验证 Provider CRUD + api_key 不泄漏。
func TestProviderCRUD(t *testing.T) {
	base, cleanup := startServer(t)
	defer cleanup()

	providersURL := base + "/api/v1/model/providers"

	// 1. 创建 provider
	createResp := jsonPost(t, providersURL, map[string]any{
		"name":          "DeepSeek Test",
		"slug":          fmt.Sprintf("deepseek-test-%d", time.Now().UnixNano()),
		"type":          "llm",
		"base_url":      "https://api.deepseek.com",
		"api_key":       "sk-test-secret-key-12345",
		"default_model": "deepseek-chat",
	})
	if createResp.StatusCode != http.StatusOK {
		t.Fatalf("create provider: expected 200, got %d", createResp.StatusCode)
	}
	createBody := readJSON(t, createResp)
	if createBody["code"].(float64) != 0 {
		t.Fatalf("create provider: code=%v message=%v", createBody["code"], createBody["message"])
	}
	data, ok := createBody["data"].(map[string]any)
	if !ok {
		t.Fatalf("create provider: no data field, body=%v", createBody)
	}
	id, ok := data["id"].(string)
	if !ok || id == "" {
		t.Fatalf("create provider: missing id, data=%v", data)
	}
	t.Logf("created provider id=%s", id)

	// 2. 列出 providers，找到刚创建的
	listResp, err := http.Get(providersURL)
	if err != nil {
		t.Fatalf("list providers: %v", err)
	}
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list providers: expected 200, got %d", listResp.StatusCode)
	}
	listBody := readJSON(t, listResp)
	if listBody["code"].(float64) != 0 {
		t.Fatalf("list providers: code=%v", listBody["code"])
	}
	items, ok := listBody["data"].([]any)
	if !ok {
		t.Fatalf("list providers: data is not array, got %T", listBody["data"])
	}
	found := false
	for _, item := range items {
		m := item.(map[string]any)
		if m["id"] == id {
			found = true
			// 验证 api_key 不在响应中
			if _, hasKey := m["api_key"]; hasKey {
				if m["api_key"] != "" {
					t.Errorf("api_key LEAKED in list response: %v", m["api_key"])
				}
			}
			// 确认 enabled 初始为 true
			if m["enabled"] != true {
				t.Errorf("expected enabled=true, got %v", m["enabled"])
			}
			break
		}
	}
	if !found {
		t.Errorf("created provider id=%s not found in list", id)
	}

	// 3. 切换 enabled=false
	patchResp := jsonPatch(t, fmt.Sprintf("%s/%s/enabled", providersURL, id), map[string]any{"enabled": false})
	if patchResp.StatusCode != http.StatusOK {
		t.Fatalf("set enabled: expected 200, got %d", patchResp.StatusCode)
	}
	patchBody := readJSON(t, patchResp)
	if patchBody["code"].(float64) != 0 {
		t.Fatalf("set enabled: code=%v", patchBody["code"])
	}

	// 4. 再次列出，验证 enabled 已更新为 false
	listResp2, _ := http.Get(providersURL)
	listBody2 := readJSON(t, listResp2)
	items2 := listBody2["data"].([]any)
	for _, item := range items2 {
		m := item.(map[string]any)
		if m["id"] == id {
			if m["enabled"] != false {
				t.Errorf("expected enabled=false after patch, got %v", m["enabled"])
			}
			break
		}
	}
	t.Log("TestProviderCRUD PASSED")
}
