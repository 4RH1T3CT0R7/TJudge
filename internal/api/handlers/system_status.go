package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/infrastructure/db"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/queue"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"go.uber.org/zap"
)

// appStartTime - время старта процесса API (для uptime).
var appStartTime = time.Now()

// SystemStatusRepository - агрегированные показатели БД.
type SystemStatusRepository interface {
	SchemaVersion(ctx context.Context) (int64, bool, error)
	MatchCountsByStatus(ctx context.Context) (map[string]int64, error)
	ProgramCountsByStatus(ctx context.Context) (map[string]int64, error)
	OutboxStats(ctx context.Context) (*db.OutboxStatus, error)
	LastCompletedMatchAt(ctx context.Context) (*time.Time, error)
	ConnectionStats() sql.DBStats
	Healthy(ctx context.Context) bool
}

// StatusQueueManager - статистика очереди матчей.
type StatusQueueManager interface {
	GetStats(ctx context.Context) (*queue.QueueStats, error)
	GetDeadLetterSize(ctx context.Context) (int64, error)
}

// StatusCompileQueue - размер очереди компиляции.
type StatusCompileQueue interface {
	Size(ctx context.Context) (int64, error)
}

// StatusWSHub - статистика WebSocket-подключений.
type StatusWSHub interface {
	GetStats() map[string]any
}

// StatusRedisPinger - проверка доступности Redis.
type StatusRedisPinger interface {
	Health(ctx context.Context) error
}

// FullSystemStatus - полное состояние системы одним ответом:
// admin-панель, make status и внешние проверки читают его из
// GET /api/v1/system/status.
type FullSystemStatus struct {
	App       AppStatus        `json:"app"`
	Database  DatabaseStatus   `json:"database"`
	Redis     RedisStatus      `json:"redis"`
	Queues    QueueStatus      `json:"queues"`
	Matches   MatchesStatus    `json:"matches"`
	Programs  map[string]int64 `json:"programs"`
	Outbox    *db.OutboxStatus `json:"outbox,omitempty"`
	WebSocket map[string]any   `json:"websocket"`
}

// AppStatus - версия и аптайм процесса API.
type AppStatus struct {
	Version       string    `json:"version"`    // vcs.revision (короткий) или "dev"
	BuildTime     string    `json:"build_time"` // vcs.time, если вшит компилятором
	Dirty         bool      `json:"dirty"`      // сборка из грязного дерева
	GoVersion     string    `json:"go_version"`
	StartedAt     time.Time `json:"started_at"`
	UptimeSeconds int64     `json:"uptime_seconds"`
}

// DatabaseStatus - здоровье и пул соединений PostgreSQL.
type DatabaseStatus struct {
	Healthy         bool  `json:"healthy"`
	SchemaVersion   int64 `json:"schema_version"`
	SchemaDirty     bool  `json:"schema_dirty"`
	OpenConnections int   `json:"open_connections"`
	InUse           int   `json:"in_use"`
	Idle            int   `json:"idle"`
	MaxOpen         int   `json:"max_open"`
}

// RedisStatus - здоровье Redis.
type RedisStatus struct {
	Healthy bool `json:"healthy"`
}

// QueueStatus - размеры всех очередей.
type QueueStatus struct {
	High       int64 `json:"high"`
	Medium     int64 `json:"medium"`
	Low        int64 `json:"low"`
	Total      int64 `json:"total"`
	DeadLetter int64 `json:"dead_letter"`
	Compile    int64 `json:"compile"`
}

// MatchesStatus - матчи по статусам.
type MatchesStatus struct {
	ByStatus        map[string]int64 `json:"by_status"`
	LastCompletedAt *time.Time       `json:"last_completed_at,omitempty"`
}

// SystemStatusHandler отдаёт полное состояние системы.
type SystemStatusHandler struct {
	statusRepo   SystemStatusRepository
	queueManager StatusQueueManager
	compileQueue StatusCompileQueue
	wsHub        StatusWSHub
	redis        StatusRedisPinger
	log          *logger.Logger
}

// NewSystemStatusHandler создаёт handler полного статуса системы.
func NewSystemStatusHandler(
	statusRepo SystemStatusRepository,
	queueManager StatusQueueManager,
	compileQueue StatusCompileQueue,
	wsHub StatusWSHub,
	redis StatusRedisPinger,
	log *logger.Logger,
) *SystemStatusHandler {
	return &SystemStatusHandler{
		statusRepo:   statusRepo,
		queueManager: queueManager,
		compileQueue: compileQueue,
		wsHub:        wsHub,
		redis:        redis,
		log:          log,
	}
}

// appVersion извлекает версию из VCS-метаданных, вшитых go build.
func appVersion() (revision, buildTime string, dirty bool) {
	revision = "dev"
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return revision, buildTime, dirty
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) >= 8 {
				revision = s.Value[:8]
			}
		case "vcs.time":
			buildTime = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	return revision, buildTime, dirty
}

// GetFullStatus возвращает полное состояние системы.
// Деградирует мягко: недоступный компонент помечается unhealthy/нулями,
// а не валит весь ответ - статус нужен именно тогда, когда что-то сломано.
// @Summary Полное состояние системы
// @Description Версия, БД, Redis, очереди, матчи, программы, outbox, WebSocket (только для админов)
// @Tags system
// @Produce json
// @Security BearerAuth
// @Success 200 {object} FullSystemStatus
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /system/status [get]
func (h *SystemStatusHandler) GetFullStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	revision, buildTime, dirty := appVersion()
	status := &FullSystemStatus{
		App: AppStatus{
			Version:       revision,
			BuildTime:     buildTime,
			Dirty:         dirty,
			GoVersion:     runtime.Version(),
			StartedAt:     appStartTime,
			UptimeSeconds: int64(time.Since(appStartTime).Seconds()),
		},
		Programs:  map[string]int64{},
		WebSocket: map[string]any{},
	}

	// --- База данных ---
	status.Database.Healthy = h.statusRepo.Healthy(ctx)
	dbStats := h.statusRepo.ConnectionStats()
	status.Database.OpenConnections = dbStats.OpenConnections
	status.Database.InUse = dbStats.InUse
	status.Database.Idle = dbStats.Idle
	status.Database.MaxOpen = dbStats.MaxOpenConnections

	if status.Database.Healthy {
		if version, dirtySchema, err := h.statusRepo.SchemaVersion(ctx); err == nil {
			status.Database.SchemaVersion = version
			status.Database.SchemaDirty = dirtySchema
		} else {
			h.log.LogError("system status: schema version", err)
		}

		if counts, err := h.statusRepo.MatchCountsByStatus(ctx); err == nil {
			status.Matches.ByStatus = counts
		} else {
			h.log.LogError("system status: match counts", err)
		}

		if last, err := h.statusRepo.LastCompletedMatchAt(ctx); err == nil {
			status.Matches.LastCompletedAt = last
		}

		if counts, err := h.statusRepo.ProgramCountsByStatus(ctx); err == nil {
			status.Programs = counts
		} else {
			h.log.LogError("system status: program counts", err)
		}

		if outbox, err := h.statusRepo.OutboxStats(ctx); err == nil {
			status.Outbox = outbox
		} else {
			h.log.LogError("system status: outbox stats", err)
		}
	}

	// --- Redis и очереди ---
	status.Redis.Healthy = h.redis.Health(ctx) == nil
	if status.Redis.Healthy {
		if qs, err := h.queueManager.GetStats(ctx); err == nil {
			status.Queues.High = qs.High
			status.Queues.Medium = qs.Medium
			status.Queues.Low = qs.Low
			status.Queues.Total = qs.Total
		} else {
			h.log.LogError("system status: queue stats", err)
		}
		if dl, err := h.queueManager.GetDeadLetterSize(ctx); err == nil {
			status.Queues.DeadLetter = dl
		}
		if cq, err := h.compileQueue.Size(ctx); err == nil {
			status.Queues.Compile = cq
		}
	}

	// --- WebSocket ---
	if h.wsHub != nil {
		status.WebSocket = h.wsHub.GetStats()
	}

	h.log.Debug("system status served",
		zap.Bool("db_healthy", status.Database.Healthy),
		zap.Bool("redis_healthy", status.Redis.Healthy),
	)

	writeJSON(w, http.StatusOK, status)
}
