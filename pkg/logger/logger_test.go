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
	// Invalid level should default to info
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

func TestNewAsync(t *testing.T) {
	log, err := NewAsync("info", "json")

	require.NoError(t, err)
	assert.NotNil(t, log)
	defer func() { _ = log.Sync() }()
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

func TestLogger_WithFields(t *testing.T) {
	log, err := New("info", "json")
	require.NoError(t, err)

	enriched := log.WithFields(
		zap.String("key1", "value1"),
		zap.Int("key2", 42),
	)

	assert.NotNil(t, enriched)
	assert.NotSame(t, log, enriched)
}

func TestLogger_WithRequestID(t *testing.T) {
	log, err := New("info", "json")
	require.NoError(t, err)

	enriched := log.WithRequestID("req-12345")

	assert.NotNil(t, enriched)
	assert.NotSame(t, log, enriched)
}

func TestLogger_WithUserID(t *testing.T) {
	log, err := New("info", "json")
	require.NoError(t, err)

	enriched := log.WithUserID("user-12345")

	assert.NotNil(t, enriched)
	assert.NotSame(t, log, enriched)
}

func TestLogger_WithMatchID(t *testing.T) {
	log, err := New("info", "json")
	require.NoError(t, err)

	enriched := log.WithMatchID("match-12345")

	assert.NotNil(t, enriched)
	assert.NotSame(t, log, enriched)
}

func TestLogger_WithTournamentID(t *testing.T) {
	log, err := New("info", "json")
	require.NoError(t, err)

	enriched := log.WithTournamentID("tournament-12345")

	assert.NotNil(t, enriched)
	assert.NotSame(t, log, enriched)
}

func TestLogger_ChainedWithMethods(t *testing.T) {
	log, err := New("info", "json")
	require.NoError(t, err)

	enriched := log.
		WithRequestID("req-1").
		WithUserID("user-1").
		WithMatchID("match-1").
		WithTournamentID("tournament-1")

	assert.NotNil(t, enriched)
	assert.NotSame(t, log, enriched)
}

func TestLogger_Sync(t *testing.T) {
	log, err := New("info", "json")
	require.NoError(t, err)

	// Sync should not panic
	err = log.Sync()
	// Sync to stdout may return error on some systems
	_ = err
}

func TestLogger_LogError(t *testing.T) {
	log, err := New("debug", "json")
	require.NoError(t, err)

	// Should not panic
	log.LogError("test error", assert.AnError, zap.String("context", "test"))
}

func TestLogger_BasicLogging(t *testing.T) {
	log, err := New("debug", "json")
	require.NoError(t, err)

	// These should not panic
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

func TestLogger_AsyncFlush(t *testing.T) {
	log, err := NewAsync("info", "json")
	require.NoError(t, err)

	log.Info("async message 1")
	log.Info("async message 2")

	// Should flush buffered messages
	err = log.Sync()
	_ = err // ignore sync errors
}

func BenchmarkLogger_Info(b *testing.B) {
	log, _ := New("info", "json")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info("benchmark message", zap.Int("iteration", i))
	}
}

func BenchmarkLogger_WithFields(b *testing.B) {
	log, _ := New("info", "json")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.WithFields(zap.String("key", "value"))
	}
}

func BenchmarkLogger_ChainedWith(b *testing.B) {
	log, _ := New("info", "json")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.WithRequestID("req-1").WithUserID("user-1")
	}
}

func BenchmarkLoggerAsync_Info(b *testing.B) {
	log, _ := NewAsync("info", "json")
	defer func() { _ = log.Sync() }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info("benchmark message", zap.Int("iteration", i))
	}
}
