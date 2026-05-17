package logger

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestIngestStdLogRedirectsDefaultLoggerToZap(t *testing.T) {
	recorded := newObservedLogger(t, zapcore.DebugLevel)
	restore := IngestStdLog()
	t.Cleanup(restore)

	log.Print("std hello")

	entries := recorded.All()
	if !assert.Len(t, entries, 1) {
		return
	}
	assert.Equal(t, "std hello", entries[0].Message)
	assert.Equal(t, zapcore.InfoLevel, entries[0].Level)
}

func TestIngestStdLogWritesJSONWithMinimumFields(t *testing.T) {
	prevStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = prevStdout
		_ = r.Close()
		_ = w.Close()
		_ = Init(Options{})
	})

	require.NoError(t, Init(Options{Service: "stdlog-json", DisableCaller: true}))
	restore := IngestStdLog()
	log.Print("json hello")
	restore()

	require.NoError(t, w.Close())
	os.Stdout = prevStdout

	data, err := io.ReadAll(r)
	require.NoError(t, err)
	var record map[string]any
	require.NoError(t, json.Unmarshal(data, &record))

	assert.NotEmpty(t, record["time"])
	assert.Equal(t, "info", record["level"])
	assert.Equal(t, "stdlog-json", record["service"])
	assert.Equal(t, "json hello", record["msg"])
}

func TestIngestStdLogUsesCurrentGlobalLogger(t *testing.T) {
	core1, recorded1 := observer.New(zapcore.DebugLevel)
	gLogger.Store(zap.New(core1))
	restore := IngestStdLog()
	t.Cleanup(func() {
		restore()
		_ = Init(Options{})
	})

	core2, recorded2 := observer.New(zapcore.DebugLevel)
	gLogger.Store(zap.New(core2, zap.Fields(zap.String("service", "stdlog-test"))))

	log.Print("after reinit")

	assert.Equal(t, 0, recorded1.Len())
	entries := recorded2.All()
	if !assert.Len(t, entries, 1) {
		return
	}
	assert.Equal(t, "after reinit", entries[0].Message)
	assert.Equal(t, "stdlog-test", entries[0].ContextMap()["service"])
}
