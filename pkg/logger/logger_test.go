package logger

import (
	"os"
	"strings"
	"testing"
	"time"

	apperrors "github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
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

// асинхронный логгер сбрасывает буфер сам, без Sync: иначе при падении
// процесса хвост логов пропадает
func TestNewWithOptions_AsyncFlushesWithoutSync(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	stdout := os.Stdout
	os.Stdout = w
	log, err := NewWithOptions(Options{Level: "info", Format: "json", Async: true})
	os.Stdout = stdout
	require.NoError(t, err)
	defer func() { _ = log.Sync() }()

	log.Info("flush me")

	got := make(chan string, 1)
	go func() {
		buf := make([]byte, 1024)
		n, _ := r.Read(buf)
		got <- string(buf[:n])
	}()
	select {
	case line := <-got:
		assert.Contains(t, line, "flush me")
	case <-time.After(3 * time.Second):
		t.Fatal("буфер не сброшен без Sync")
	}
}

func TestLogger_LogError(t *testing.T) {
	log, err := New("debug", "json")
	require.NoError(t, err)

	// не должен паниковать
	log.LogError("test error", assert.AnError, zap.String("context", "test"))
}

// ошибка клиента (4xx) идёт в warn, остальные - в error; caller - место вызова LogError
func TestLogger_LogErrorLevelAndCaller(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	log := &Logger{Logger: zap.New(core, zap.AddCaller())}

	log.LogError("client", apperrors.ErrValidation)
	log.LogError("server", assert.AnError)

	entries := logs.All()
	require.Len(t, entries, 2)
	assert.Equal(t, zap.WarnLevel, entries[0].Level)
	assert.Equal(t, zap.ErrorLevel, entries[1].Level)
	assert.True(t, strings.HasSuffix(entries[1].Caller.File, "logger_test.go"), entries[1].Caller.File)
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
