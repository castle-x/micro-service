// Package health exposes standard admin health endpoints for services.
package health

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/castlexu/micro-service/pkg/logger"
)

const defaultCheckTimeout = time.Second

// Commit and BuiltAt can be overridden by build ldflags.
var (
	Commit  = "unknown"
	BuiltAt = "unknown"
)

// CheckFunc is a lightweight dependency readiness probe.
type CheckFunc func(context.Context) error

// Config configures the admin health server.
type Config struct {
	Service      string
	Addr         string
	Commit       string
	BuiltAt      string
	CheckTimeout time.Duration
}

// Server owns the admin HTTP listener and registered readiness checks.
type Server struct {
	service      string
	addr         string
	commit       string
	builtAt      string
	checkTimeout time.Duration

	mu         sync.RWMutex
	checks     []namedCheck
	httpServer *http.Server
}

type namedCheck struct {
	name string
	fn   CheckFunc
}

// NewServer creates a health server. Call Start to listen on Addr.
func NewServer(cfg Config) *Server {
	checkTimeout := cfg.CheckTimeout
	if checkTimeout <= 0 {
		checkTimeout = defaultCheckTimeout
	}
	addr := strings.TrimSpace(cfg.Addr)
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	commit := cfg.Commit
	if commit == "" {
		commit = Commit
	}
	builtAt := cfg.BuiltAt
	if builtAt == "" {
		builtAt = BuiltAt
	}
	return &Server{
		service:      cfg.Service,
		addr:         addr,
		commit:       commit,
		builtAt:      builtAt,
		checkTimeout: checkTimeout,
	}
}

// AdminAddr returns the admin listener address using service-specific env,
// generic env, then the default localhost port.
func AdminAddr(service string, defaultPort int) string {
	serviceKey := strings.ToUpper(strings.NewReplacer("-", "_", ".", "_").Replace(service)) + "_ADMIN_ADDR"
	if value := strings.TrimSpace(os.Getenv(serviceKey)); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("SERVICE_ADMIN_ADDR")); value != "" {
		return value
	}
	return fmt.Sprintf("127.0.0.1:%d", defaultPort)
}

// Check registers a readiness dependency check.
func (s *Server) Check(name string, fn CheckFunc) {
	name = strings.TrimSpace(name)
	if s == nil || name == "" || fn == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checks = append(s.checks, namedCheck{name: name, fn: fn})
}

// Handler returns the admin HTTP mux. It is useful for tests and embedding.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/readyz", s.handleReadyz)
	mux.HandleFunc("/version", s.handleVersion)
	return mux
}

// Start starts the admin HTTP listener in the background. Listen failures are
// logged as warnings and do not stop the business server.
func (s *Server) Start() {
	if s == nil {
		return
	}
	httpServer := &http.Server{
		Addr:              s.addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 2 * time.Second,
	}
	s.mu.Lock()
	s.httpServer = httpServer
	s.mu.Unlock()

	go func() {
		logger.L().Info("admin health listening", zap.String("addr", s.addr), zap.String("service", s.service))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.L().Warn("admin health server failed", zap.String("addr", s.addr), zap.Error(err))
		}
	}()
}

// Shutdown gracefully stops the admin listener.
func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	httpServer := s.httpServer
	s.mu.RUnlock()
	if httpServer == nil {
		return nil
	}
	return httpServer.Shutdown(ctx)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	statusCode := http.StatusOK
	bodyStatus := "ready"
	deps := make(map[string]string)

	for _, check := range s.snapshotChecks() {
		result := s.runCheck(r.Context(), check.fn)
		deps[check.name] = result
		if result != "ok" {
			statusCode = http.StatusServiceUnavailable
			bodyStatus = "not_ready"
		}
	}

	writeJSON(w, statusCode, map[string]any{
		"status": bodyStatus,
		"deps":   deps,
	})
}

func (s *Server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service":    s.service,
		"commit":     s.commit,
		"built_at":   s.builtAt,
		"go_version": runtime.Version(),
	})
}

func (s *Server) snapshotChecks() []namedCheck {
	s.mu.RLock()
	defer s.mu.RUnlock()
	checks := make([]namedCheck, len(s.checks))
	copy(checks, s.checks)
	return checks
}

func (s *Server) runCheck(parent context.Context, fn CheckFunc) string {
	ctx, cancel := context.WithTimeout(parent, s.checkTimeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- fn(ctx)
	}()

	select {
	case err := <-done:
		if err == nil {
			return "ok"
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "timeout"
		}
		return "error"
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "timeout"
		}
		return "error"
	}
}

func writeJSON(w http.ResponseWriter, statusCode int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(body)
}
