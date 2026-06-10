package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/infrastructure/db"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/queue"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Стабы ---

type stubStatusRepo struct {
	healthy bool
}

func (s *stubStatusRepo) SchemaVersion(_ context.Context) (int64, bool, error) {
	return 41, false, nil
}

func (s *stubStatusRepo) MatchCountsByStatus(_ context.Context) (map[string]int64, error) {
	return map[string]int64{"pending": 3, "completed": 100, "failed": 2}, nil
}

func (s *stubStatusRepo) ProgramCountsByStatus(_ context.Context) (map[string]int64, error) {
	return map[string]int64{"ready": 10, "compiling": 1}, nil
}

func (s *stubStatusRepo) OutboxStats(_ context.Context) (*db.OutboxStatus, error) {
	return &db.OutboxStatus{Pending: 1, Errors: 0, DoneLast24h: 50}, nil
}

func (s *stubStatusRepo) LastCompletedMatchAt(_ context.Context) (*time.Time, error) {
	ts := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	return &ts, nil
}

func (s *stubStatusRepo) StuckRunningCount(_ context.Context, _ time.Duration) (int64, error) {
	return 2, nil
}

func (s *stubStatusRepo) ConnectionStats() sql.DBStats {
	return sql.DBStats{OpenConnections: 5, InUse: 2, Idle: 3, MaxOpenConnections: 50}
}

func (s *stubStatusRepo) Healthy(_ context.Context) bool { return s.healthy }

type stubStatusQueue struct{}

func (s *stubStatusQueue) GetStats(_ context.Context) (*queue.QueueStats, error) {
	return &queue.QueueStats{High: 1, Medium: 2, Low: 3, Total: 6}, nil
}

func (s *stubStatusQueue) GetDeadLetterSize(_ context.Context) (int64, error) { return 4, nil }

type stubCompileQueue struct{}

func (s *stubCompileQueue) Size(_ context.Context) (int64, error) { return 7, nil }

type stubWSHub struct{}

func (s *stubWSHub) GetStats() map[string]any {
	return map[string]any{"tournaments": 2, "total_clients": 9}
}

type stubRedis struct{ err error }

func (s *stubRedis) Health(_ context.Context) error { return s.err }

func newStatusHandler(t *testing.T, dbHealthy bool, redisErr error) *SystemStatusHandler {
	t.Helper()
	log, _ := logger.New("error", "json")
	return NewSystemStatusHandler(
		&stubStatusRepo{healthy: dbHealthy},
		&stubStatusQueue{},
		&stubCompileQueue{},
		&stubWSHub{},
		&stubRedis{err: redisErr},
		log,
	)
}

// --- Тесты ---

func TestSystemStatus_FullStatusHealthy(t *testing.T) {
	h := newStatusHandler(t, true, nil)

	req := httptest.NewRequest(http.MethodGet, "/system/status", nil)
	rec := httptest.NewRecorder()
	h.GetFullStatus(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var envelope struct {
		Data FullSystemStatus `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	status := envelope.Data

	assert.True(t, status.Database.Healthy)
	assert.Equal(t, int64(41), status.Database.SchemaVersion)
	assert.Equal(t, 50, status.Database.MaxOpen)
	assert.True(t, status.Redis.Healthy)
	assert.Equal(t, int64(6), status.Queues.Total)
	assert.Equal(t, int64(4), status.Queues.DeadLetter)
	assert.Equal(t, int64(7), status.Queues.Compile)
	assert.Equal(t, int64(100), status.Matches.ByStatus["completed"])
	assert.Equal(t, int64(2), status.Matches.StuckRunning)
	assert.NotNil(t, status.Matches.LastCompletedAt)
	assert.Equal(t, int64(10), status.Programs["ready"])
	require.NotNil(t, status.Outbox)
	assert.Equal(t, int64(50), status.Outbox.DoneLast24h)
	assert.NotEmpty(t, status.App.GoVersion)
	assert.GreaterOrEqual(t, status.App.UptimeSeconds, int64(0))
}

func TestSystemStatus_DegradesGracefully(t *testing.T) {
	// БД и Redis лежат: ответ всё равно 200, компоненты помечены unhealthy.
	// Статус нужен именно тогда, когда что-то сломано.
	h := newStatusHandler(t, false, context.DeadlineExceeded)

	req := httptest.NewRequest(http.MethodGet, "/system/status", nil)
	rec := httptest.NewRecorder()
	h.GetFullStatus(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var envelope struct {
		Data FullSystemStatus `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	status := envelope.Data

	assert.False(t, status.Database.Healthy)
	assert.False(t, status.Redis.Healthy)
	// При нездоровой БД агрегаты не запрашиваются - нули, а не ошибка.
	assert.Equal(t, int64(0), status.Database.SchemaVersion)
	assert.Empty(t, status.Matches.ByStatus)
	// При нездоровом Redis очереди не запрашиваются.
	assert.Equal(t, int64(0), status.Queues.Total)
}
