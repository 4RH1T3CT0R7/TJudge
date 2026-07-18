package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNew_DefaultLevel(t *testing.T) {
	log, err := New("info", "json")

	require.NoError(t, err)
	assert.NotNil(t, log)
	assert.NotNil(t, log.Logger)
}

func TestNew_DebugLevel(t *testing.T) {
	log, err := New("debug", "json")

	require.NoError(t, err)
	assert.NotNil(t, log)
}

func TestNew_ErrorLevel(t *testing.T) {
	log, err := New("error", "json")

	require.NoError(t, err)
	assert.NotNil(t, log)
}

func TestNew_InvalidLevel(t *testing.T) {
	// Некорректный уровень должен откатиться к info
	log, err := New("invalid", "json")

	require.NoError(t, err)
	assert.NotNil(t, log)
}

func TestNew_ConsoleFormat(t *testing.T) {
	log, err := New("info", "console")

	require.NoError(t, err)
	assert.NotNil(t, log)
}

func TestNew_JSONFormat(t *testing.T) {
	log, err := New("info", "json")

	require.NoError(t, err)
	assert.NotNil(t, log)
}

func TestNewWithOptions(t *testing.T) {
	opts := Options{
		Level:  "debug",
		Format: "json",
		Async:  true,
	}

	log, err := NewWithOptions(opts)

	require.NoError(t, err)
	assert.NotNil(t, log)
	defer func() { _ = log.Sync() }()
}

func TestNewWithOptions_AllFormats(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{
		{"json", "json"},
		{"console", "console"},
		{"other", "text"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := Options{
				Level:  "info",
				Format: tc.format,
				Async:  false,
			}
			log, err := NewWithOptions(opts)
			require.NoError(t, err)
			assert.NotNil(t, log)
		})
	}
}

func TestLogger_Sync(t *testing.T) {
	log, err := New("info", "json")
	require.NoError(t, err)

	// Sync не должен паниковать
	err = log.Sync()
	// Sync в stdout может возвращать ошибку на некоторых системах
	_ = err
}

func TestLogger_LogError(t *testing.T) {
	log, err := New("debug", "json")
	require.NoError(t, err)

	// Не должен паниковать
	log.LogError("test error", assert.AnError, zap.String("context", "test"))
}

func TestLogger_BasicLogging(t *testing.T) {
	log, err := New("debug", "json")
	require.NoError(t, err)

	// Не должны паниковать
	log.Debug("debug message")
	log.Info("info message")
	log.Warn("warn message")
}

func TestLogger_LoggingWithFields(t *testing.T) {
	log, err := New("debug", "json")
	require.NoError(t, err)

	log.Info("message with fields",
		zap.String("key1", "value1"),
		zap.Int("key2", 42),
		zap.Bool("key3", true),
	)
}

func BenchmarkLogger_Info(b *testing.B) {
	log, _ := New("info", "json")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info("benchmark message", zap.Int("iteration", i))
	}
}
