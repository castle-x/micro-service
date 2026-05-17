package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"
)

func TestHandlersReturnHealthReadyAndVersion(t *testing.T) {
	s := NewServer(Config{
		Service:      "iam",
		Addr:         "127.0.0.1:48082",
		Commit:       "abc123",
		BuiltAt:      "2026-05-14T00:00:00Z",
		CheckTimeout: 50 * time.Millisecond,
	})
	s.Check("mongo", func(context.Context) error { return nil })

	healthResp := requestJSON(t, s.Handler(), http.MethodGet, "/healthz")
	if healthResp.Code != http.StatusOK {
		t.Fatalf("healthz status = %d, want %d", healthResp.Code, http.StatusOK)
	}
	assertBody(t, healthResp.Body.Bytes(), map[string]any{"status": "ok"})

	readyResp := requestJSON(t, s.Handler(), http.MethodGet, "/readyz")
	if readyResp.Code != http.StatusOK {
		t.Fatalf("readyz status = %d, want %d", readyResp.Code, http.StatusOK)
	}
	assertBody(t, readyResp.Body.Bytes(), map[string]any{
		"status": "ready",
		"deps": map[string]any{
			"mongo": "ok",
		},
	})

	versionResp := requestJSON(t, s.Handler(), http.MethodGet, "/version")
	if versionResp.Code != http.StatusOK {
		t.Fatalf("version status = %d, want %d", versionResp.Code, http.StatusOK)
	}
	assertBody(t, versionResp.Body.Bytes(), map[string]any{
		"service":    "iam",
		"commit":     "abc123",
		"built_at":   "2026-05-14T00:00:00Z",
		"go_version": runtime.Version(),
	})
}

func TestReadyzReturnsNotReadyForFailedCheck(t *testing.T) {
	s := NewServer(Config{
		Service:      "idp",
		Addr:         "127.0.0.1:48081",
		CheckTimeout: 50 * time.Millisecond,
	})
	s.Check("mongo", func(context.Context) error { return nil })
	s.Check("redis", func(context.Context) error { return errors.New("redis unavailable") })

	resp := requestJSON(t, s.Handler(), http.MethodGet, "/readyz")
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz status = %d, want %d", resp.Code, http.StatusServiceUnavailable)
	}
	assertBody(t, resp.Body.Bytes(), map[string]any{
		"status": "not_ready",
		"deps": map[string]any{
			"mongo": "ok",
			"redis": "error",
		},
	})
}

func TestReadyzTimesOutSlowChecks(t *testing.T) {
	s := NewServer(Config{
		Service:      "asset",
		Addr:         "127.0.0.1:48084",
		CheckTimeout: 10 * time.Millisecond,
	})
	s.Check("mongo", func(context.Context) error { return nil })
	s.Check("redis", func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	resp := requestJSON(t, s.Handler(), http.MethodGet, "/readyz")
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz status = %d, want %d", resp.Code, http.StatusServiceUnavailable)
	}
	assertBody(t, resp.Body.Bytes(), map[string]any{
		"status": "not_ready",
		"deps": map[string]any{
			"mongo": "ok",
			"redis": "timeout",
		},
	})
}

func TestAdminAddrPrefersServiceSpecificOverride(t *testing.T) {
	t.Setenv("SERVICE_ADMIN_ADDR", "127.0.0.1:49999")
	t.Setenv("EDGE_API_ADMIN_ADDR", "127.0.0.1:48888")

	if got := AdminAddr("edge-api", 48080); got != "127.0.0.1:48888" {
		t.Fatalf("AdminAddr service override = %q, want %q", got, "127.0.0.1:48888")
	}
	if got := AdminAddr("iam", 48082); got != "127.0.0.1:49999" {
		t.Fatalf("AdminAddr generic override = %q, want %q", got, "127.0.0.1:49999")
	}
}

func requestJSON(t *testing.T, h http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if got := resp.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("%s content-type = %q, want application/json", target, got)
	}
	return resp
}

func assertBody(t *testing.T, raw []byte, want map[string]any) {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal body: %v; body=%s", err, raw)
	}
	if !mapsEqual(got, want) {
		t.Fatalf("body = %#v, want %#v", got, want)
	}
}

func mapsEqual(got, want map[string]any) bool {
	if len(got) != len(want) {
		return false
	}
	for key, wantValue := range want {
		gotValue, ok := got[key]
		if !ok {
			return false
		}
		wantMap, wantIsMap := wantValue.(map[string]any)
		gotMap, gotIsMap := gotValue.(map[string]any)
		if wantIsMap || gotIsMap {
			if !wantIsMap || !gotIsMap || !mapsEqual(gotMap, wantMap) {
				return false
			}
			continue
		}
		if gotValue != wantValue {
			return false
		}
	}
	return true
}
