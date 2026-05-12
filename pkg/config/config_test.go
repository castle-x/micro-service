package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/castlexu/micro-service/pkg/errno"
)

type testCfg struct {
	Mongo struct {
		URI string `mapstructure:"uri"`
		DB  string `mapstructure:"db"`
	} `mapstructure:"mongo"`
	Log struct {
		Level  string `mapstructure:"level"`
		Format string `mapstructure:"format"`
	} `mapstructure:"log"`
	JWT struct {
		Secret    string `mapstructure:"secret"`
		AccessTTL int    `mapstructure:"access_ttl"`
	} `mapstructure:"jwt"`
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

func TestLoad_YAMLOnly(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "c.yaml", `
mongo:
  uri: mongodb://localhost:27017
  db: platform
log:
  level: info
  format: json
`)
	var c testCfg
	require.NoError(t, Load(path, &c))
	assert.Equal(t, "mongodb://localhost:27017", c.Mongo.URI)
	assert.Equal(t, "platform", c.Mongo.DB)
	assert.Equal(t, "info", c.Log.Level)
}

func TestLoad_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "c.yaml", `
log:
  level: info
`)
	t.Setenv("LOG_LEVEL", "debug")

	var c testCfg
	require.NoError(t, Load(path, &c))
	assert.Equal(t, "debug", c.Log.Level, "env LOG_LEVEL should override log.level")
}

func TestLoad_VarExpansion(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "c.yaml", `
jwt:
  secret: ${JWT_SECRET}
  access_ttl: 3600
`)
	t.Setenv("JWT_SECRET", "super-secret-abcdefghijklmnopqrst")

	var c testCfg
	require.NoError(t, Load(path, &c))
	assert.Equal(t, "super-secret-abcdefghijklmnopqrst", c.JWT.Secret)
	assert.Equal(t, 3600, c.JWT.AccessTTL)
}

func TestLoad_VarExpansionWithDefault(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "c.yaml", `
mongo:
  uri: ${MONGO_URI:mongodb://localhost:27017/platform}
  db: ${MONGO_DB:platform}
`)

	var c testCfg
	require.NoError(t, Load(path, &c))
	assert.Equal(t, "mongodb://localhost:27017/platform", c.Mongo.URI)
	assert.Equal(t, "platform", c.Mongo.DB)
}

func TestLoad_FileNotExist_OK(t *testing.T) {
	// 文件不存在：不报错，仅走 env + 零值。
	t.Setenv("LOG_LEVEL", "warn")
	var c testCfg
	require.NoError(t, Load("/tmp/definitely-not-exist.yaml", &c))
	assert.Equal(t, "warn", c.Log.Level)
	assert.Empty(t, c.Mongo.URI)
}

func TestLoad_EmptyPath_OK(t *testing.T) {
	t.Setenv("LOG_LEVEL", "error")
	var c testCfg
	require.NoError(t, Load("", &c))
	assert.Equal(t, "error", c.Log.Level)
}

func TestLoad_NilOut(t *testing.T) {
	err := Load[testCfg]("", nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errno.ErrInvalidParam))
}

func TestLoad_ParseError(t *testing.T) {
	dir := t.TempDir()
	// 非法 yaml（用 tab 缩进且结构错乱）
	path := writeFile(t, dir, "bad.yaml", "mongo: [not: yaml")
	var c testCfg
	err := Load(path, &c)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errno.ErrInternal))
}

func TestRequireEnv_AllPresent(t *testing.T) {
	t.Setenv("FOO", "1")
	t.Setenv("BAR", "2")
	assert.NoError(t, RequireEnv("FOO", "BAR"))
}

func TestRequireEnv_Missing(t *testing.T) {
	t.Setenv("FOO", "1")
	// 强制清空 MISSING_KEY，避免外部环境污染
	require.NoError(t, os.Unsetenv("MISSING_KEY"))
	err := RequireEnv("FOO", "MISSING_KEY")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errno.ErrInvalidParam))
}

func TestMustLoad_Panic(t *testing.T) {
	defer func() {
		r := recover()
		assert.NotNil(t, r)
	}()
	MustLoad[testCfg]("", nil)
}
