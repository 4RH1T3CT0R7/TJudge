package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/queue"
	"github.com/bmstu-itstech/tjudge/internal/storage"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"go.uber.org/zap"
)

// TODO: system-ручек тут набралось прилично (метрики, health, полный статус,
// recovery в соседнем файле) — стоит разложить по под-пакетам, пока не разрослось

// appStartTime — момент старта процесса API, отсюда считается uptime
var appStartTime = time.Now()

// SystemMetrics описывает метрики системных ресурсов
type SystemMetrics struct {
	CPU         CPUMetrics        `json:"cpu"`
	Memory      MemoryMetrics     `json:"memory"`
	Disk        DiskMetrics       `json:"disk"`
	Host        HostMetrics       `json:"host"`
	Go          GoMetrics         `json:"go"`
	Temperature []TemperatureInfo `json:"temperature,omitempty"`
}

// использование CPU
type CPUMetrics struct {
	UsagePercent float64   `json:"usage_percent"`
	Cores        int       `json:"cores"`
	ModelName    string    `json:"model_name,omitempty"`
	PerCore      []float64 `json:"per_core,omitempty"`
}

// использование памяти
type MemoryMetrics struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
}

// использование диска
type DiskMetrics struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
	Path        string  `json:"path"`
}

// информация о хосте
type HostMetrics struct {
	Hostname        string `json:"hostname"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platform_version"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
	Uptime          uint64 `json:"uptime"`
}

// метрики Go runtime
type GoMetrics struct {
	Version    string `json:"version"`
	Goroutines int    `json:"goroutines"`
	HeapAlloc  uint64 `json:"heap_alloc"`
	HeapSys    uint64 `json:"heap_sys"`
	NumGC      uint32 `json:"num_gc"`
	GOMAXPROCS int    `json:"gomaxprocs"`
}

// данные одного датчика температуры
type TemperatureInfo struct {
	SensorKey   string  `json:"sensor_key"`
	Temperature float64 `json:"temperature"`
}

// SystemHandler обрабатывает system-related API-запросы
type SystemHandler struct {
	log *logger.Logger
}

// NewSystemHandler создаёт новый system handler
func NewSystemHandler(log *logger.Logger) *SystemHandler {
	return &SystemHandler{
		log: log,
	}
}

// GetMetrics возвращает системные метрики
// @Summary Системные метрики
// @Description Возвращает метрики CPU, памяти, диска, Go runtime (только для админов)
// @Tags system
// @Produce json
// @Security BearerAuth
// @Success 200 {object} SystemMetrics
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /system/metrics [get]
func (h *SystemHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := SystemMetrics{}

	// Метрики CPU
	cpuPercent, err := cpu.Percent(100*time.Millisecond, false)
	if err == nil && len(cpuPercent) > 0 {
		metrics.CPU.UsagePercent = cpuPercent[0]
	}

	// загрузка по ядрам
	cpuPerCore, err := cpu.Percent(100*time.Millisecond, true)
	if err == nil {
		metrics.CPU.PerCore = cpuPerCore
	}

	metrics.CPU.Cores = runtime.NumCPU()

	// модель CPU
	cpuInfo, err := cpu.Info()
	if err == nil && len(cpuInfo) > 0 {
		metrics.CPU.ModelName = cpuInfo[0].ModelName
	}

	// Метрики памяти
	vmStat, err := mem.VirtualMemory()
	if err == nil {
		metrics.Memory.Total = vmStat.Total
		metrics.Memory.Used = vmStat.Used
		metrics.Memory.Free = vmStat.Free
		metrics.Memory.UsedPercent = vmStat.UsedPercent
	}

	// диск — проверяется несколько путей, чтобы найти основной системный
	diskPaths := []string{"/System/Volumes/Data", "/", os.Getenv("HOME")}
	if runtime.GOOS != "darwin" {
		diskPaths = []string{"/"}
	}

	for _, diskPath := range diskPaths {
		diskStat, err := disk.Usage(diskPath)
		if err == nil && diskStat.Total > 50*1024*1024*1024 { // минимум 50GB, чтобы считаться реальным диском
			metrics.Disk.Total = diskStat.Total
			metrics.Disk.Used = diskStat.Used
			metrics.Disk.Free = diskStat.Free
			metrics.Disk.UsedPercent = diskStat.UsedPercent
			metrics.Disk.Path = diskPath
			break
		}
	}

	// информация о хосте
	hostInfo, err := host.Info()
	if err == nil {
		metrics.Host.Hostname = hostInfo.Hostname
		metrics.Host.Platform = hostInfo.Platform
		metrics.Host.PlatformVersion = hostInfo.PlatformVersion
		metrics.Host.OS = hostInfo.OS
		metrics.Host.Arch = hostInfo.KernelArch
		metrics.Host.Uptime = hostInfo.Uptime
	}

	// метрки Go runtime
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	metrics.Go.Version = runtime.Version()
	metrics.Go.Goroutines = runtime.NumGoroutine()
	metrics.Go.HeapAlloc = memStats.HeapAlloc
	metrics.Go.HeapSys = memStats.HeapSys
	metrics.Go.NumGC = memStats.NumGC
	metrics.Go.GOMAXPROCS = runtime.GOMAXPROCS(0)

	// датчики температуры (есть не на всех системах)
	temps, err := host.SensorsTemperatures()
	if err == nil {
		for _, temp := range temps {
			if temp.Temperature > 0 {
				metrics.Temperature = append(metrics.Temperature, TemperatureInfo{
					SensorKey:   temp.SensorKey,
					Temperature: temp.Temperature,
				})
			}
		}
	}

	writeJSON(w, http.StatusOK, metrics)
}

// GetHealth возвращает статус здоровья системы
// @Summary Состояние системы
// @Description Возвращает статус здоровья системы (только для админов)
// @Tags system
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{status=string,timestamp=string,hostname=string,pid=int}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /system/health [get]
func (h *SystemHandler) GetHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]any{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"hostname":  "",
		"pid":       os.Getpid(),
	}

	if hostname, err := os.Hostname(); err == nil {
		health["hostname"] = hostname
	}

	// базовая проверка ресурсов
	vmStat, err := mem.VirtualMemory()
	if err == nil {
		if vmStat.UsedPercent > 90 {
			health["status"] = "warning"
			health["warning"] = "high memory usage"
		}
	}

	writeJSON(w, http.StatusOK, health)
}

// SystemStatusRepository — агрегированные показатели БД
type SystemStatusRepository interface {
	SchemaVersion(ctx context.Context) (int64, bool, error)
	MatchCountsByStatus(ctx context.Context) (map[string]int64, error)
	ProgramCountsByStatus(ctx context.Context) (map[string]int64, error)
	OutboxStats(ctx context.Context) (*storage.OutboxStatus, error)
	LastCompletedMatchAt(ctx context.Context) (*time.Time, error)
	StuckRunningCount(ctx context.Context, olderThan time.Duration) (int64, error)
	ConnectionStats() sql.DBStats
	Healthy(ctx context.Context) bool
}

// статистика очереди матчей
type StatusQueueManager interface {
	GetStats(ctx context.Context) (*queue.QueueStats, error)
	GetDeadLetterSize(ctx context.Context) (int64, error)
}

// размер очереди компиляции
type StatusCompileQueue interface {
	Size(ctx context.Context) (int64, error)
}

// статистика WebSocket-подключений
type StatusWSHub interface {
	GetStats() map[string]any
}

// проверка доступности Redis
type StatusRedisPinger interface {
	Health(ctx context.Context) error
}

// FullSystemStatus — полное состояние системы одним ответом:
// admin-панель, make status и внешние проверки читают его из
// GET /api/v1/system/status.
type FullSystemStatus struct {
	App       AppStatus             `json:"app"`
	Database  DatabaseStatus        `json:"database"`
	Redis     RedisStatus           `json:"redis"`
	Queues    QueueStatus           `json:"queues"`
	Matches   MatchesStatus         `json:"matches"`
	Programs  map[string]int64      `json:"programs"`
	Outbox    *storage.OutboxStatus `json:"outbox,omitempty"`
	WebSocket map[string]any        `json:"websocket"`
}

// версия и аптайм процесса API
type AppStatus struct {
	Version       string    `json:"version"`    // vcs.revision (короткий) или "dev"
	BuildTime     string    `json:"build_time"` // vcs.time, если вшит компилятором
	Dirty         bool      `json:"dirty"`      // сборка из грязного дерева
	GoVersion     string    `json:"go_version"`
	StartedAt     time.Time `json:"started_at"`
	UptimeSeconds int64     `json:"uptime_seconds"`
}

// здоровье и пул соединений PostgreSQL
type DatabaseStatus struct {
	Healthy         bool  `json:"healthy"`
	SchemaVersion   int64 `json:"schema_version"`
	SchemaDirty     bool  `json:"schema_dirty"`
	OpenConnections int   `json:"open_connections"`
	InUse           int   `json:"in_use"`
	Idle            int   `json:"idle"`
	MaxOpen         int   `json:"max_open"`
}

// здоровье Redis
type RedisStatus struct {
	Healthy bool `json:"healthy"`
}

// размеры всех очередей
type QueueStatus struct {
	High       int64 `json:"high"`
	Medium     int64 `json:"medium"`
	Low        int64 `json:"low"`
	Total      int64 `json:"total"`
	DeadLetter int64 `json:"dead_letter"`
	Compile    int64 `json:"compile"`
}

// матчи по статусам
type MatchesStatus struct {
	ByStatus map[string]int64 `json:"by_status"`
	// StuckRunning — матчи в running дольше порога зависания: признак умершего
	// worker'а; чинится кнопкой «Сбросить зависшие матчи».
	StuckRunning    int64      `json:"stuck_running"`
	LastCompletedAt *time.Time `json:"last_completed_at,omitempty"`
}

// SystemStatusHandler отдаёт полное состояние системы
type SystemStatusHandler struct {
	statusRepo   SystemStatusRepository
	queueManager StatusQueueManager
	compileQueue StatusCompileQueue
	wsHub        StatusWSHub
	redis        StatusRedisPinger
	// порог зависания running-матча, тот же что у recovery воркера
	stuckThreshold time.Duration
	log            *logger.Logger
}

// NewSystemStatusHandler создаёт handler полного статуса системы
func NewSystemStatusHandler(
	statusRepo SystemStatusRepository,
	queueManager StatusQueueManager,
	compileQueue StatusCompileQueue,
	wsHub StatusWSHub,
	redis StatusRedisPinger,
	stuckThreshold time.Duration,
	log *logger.Logger,
) *SystemStatusHandler {
	return &SystemStatusHandler{
		statusRepo:     statusRepo,
		queueManager:   queueManager,
		compileQueue:   compileQueue,
		wsHub:          wsHub,
		redis:          redis,
		stuckThreshold: stuckThreshold,
		log:            log,
	}
}

// injectedVersion вшивается линкером при сборке Docker-образа (тег релиза):
// в контексте сборки нет .git, поэтому vcs.revision там недоступен. путь пакета
// в -X должен совпадать с тем, что прописан в docker/api/Dockerfile.
//
//	go build -ldflags "-X github.com/bmstu-itstech/tjudge/internal/handlers.injectedVersion=v1.7.6"
var injectedVersion string

// appVersion достаёт версию: тег из ldflags, иначе VCS-метаданные go build
func appVersion() (revision, buildTime string, dirty bool) {
	revision = "dev"
	if injectedVersion != "" {
		revision = injectedVersion
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return revision, buildTime, dirty
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			// тег из ldflags точнее коммита — не перетирается
			if injectedVersion == "" && len(s.Value) >= 8 {
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
// а не валит весь ответ — статус нужен именно тогда, когда что-то сломано.
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

		if stuck, err := h.statusRepo.StuckRunningCount(ctx, h.stuckThreshold); err == nil {
			status.Matches.StuckRunning = stuck
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
