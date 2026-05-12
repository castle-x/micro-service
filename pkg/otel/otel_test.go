package otel_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/castlexu/micro-service/pkg/config"
	pkgotel "github.com/castlexu/micro-service/pkg/otel"
)

type testConfig struct {
	OTel pkgotel.Config `mapstructure:"otel"`
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

func TestConfigLoad_ParsesOTelFieldsAndEnvOverride(t *testing.T) {
	path := writeFile(t, t.TempDir(), "c.yaml", `
otel:
  enabled: true
  endpoint: "collector:4317"
  protocol: "grpc"
  environment: "local"
  sample_ratio: 0.25
`)
	t.Setenv("OTEL_ENVIRONMENT", "staging")

	var cfg testConfig
	require.NoError(t, config.Load(path, &cfg))

	assert.True(t, cfg.OTel.Enabled)
	assert.Equal(t, "collector:4317", cfg.OTel.Endpoint)
	assert.Equal(t, "grpc", cfg.OTel.Protocol)
	assert.Equal(t, "staging", cfg.OTel.Environment)
	assert.Equal(t, 0.25, cfg.OTel.SampleRatio)
}

func TestInit_DisabledReturnsNoopShutdown(t *testing.T) {
	shutdown, err := pkgotel.Init(context.Background(), "edge-api", pkgotel.Config{})
	require.NoError(t, err)
	require.NotNil(t, shutdown)
	assert.NoError(t, shutdown(context.Background()))
}

func TestInit_EnabledWithoutEndpointDegradesToNoop(t *testing.T) {
	shutdown, err := pkgotel.Init(context.Background(), "edge-api", pkgotel.Config{
		Enabled: true,
	})
	require.NoError(t, err)
	require.NotNil(t, shutdown)
	assert.NoError(t, shutdown(context.Background()))
}
