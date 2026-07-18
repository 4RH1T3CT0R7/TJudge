package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNew(t *testing.T) {
	log, err := New("info", "json")
	require.NoError(t, err)
	assert.NotNil(t, log)
	assert.NotNil(t, log.Logger)
}

func TestNew_InvalidLevelFallsBackToInfo(t *testing.T) {
	// кривой уровень не должен ронять создание логгера
	log, err := New("не-уровень", "json")
	require.NoError(t, err)
	assert.NotNil(t, log)
}

func TestNewWithOptions_AllFormats(t *testing.T) {
	for _, format := range []string{"json", "console", "text"} {
		opts := Options{Level: "info", Format: format, Async: false}
		log, err := NewWithOptions(opts)
		require.NoError(t, err)
		assert.NotNil(t, log)
	}
}

func TestNewWithOptions_Async(t *testing.T) {
	log, err := NewWithOptions(Options{Level: "debug", Format: "json", Async: true})
	require.NoError(t, err)
	assert.NotNil(t, log)
	_ = log.Sync() // sync на stdout иногда ругается, это ок
}

func TestLogger_LogError(t *testing.T) {
	log, err := New("debug", "json")
	require.NoError(t, err)

	// не должен паниковать
	log.LogError("test error", assert.AnError, zap.String("context", "test"))
}

func TestLogger_BasicLogging(t *testing.T) {
	log, err := New("debug", "json")
	require.NoError(t, err)

	log.Debug("debug message")
	log.Info("info message", zap.Int("n", 42))
	log.Warn("warn message")
}

func BenchmarkLogger_Info(b *testing.B) {
	log, _ := New("info", "json")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info("benchmark message", zap.Int("iteration", i))
	}
}
