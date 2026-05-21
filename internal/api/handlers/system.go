package handlers

import (
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

// SystemMetrics описывает метрики системных ресурсов
type SystemMetrics struct {
	CPU         CPUMetrics        `json:"cpu"`
	Memory      MemoryMetrics     `json:"memory"`
	Disk        DiskMetrics       `json:"disk"`
	Host        HostMetrics       `json:"host"`
	Go          GoMetrics         `json:"go"`
	Temperature []TemperatureInfo `json:"temperature,omitempty"`
}

// CPUMetrics описывает метрики использования CPU
type CPUMetrics struct {
	UsagePercent float64   `json:"usage_percent"`
	Cores        int       `json:"cores"`
	ModelName    string    `json:"model_name,omitempty"`
	PerCore      []float64 `json:"per_core,omitempty"`
}

// MemoryMetrics описывает метрики использования памяти
type MemoryMetrics struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
}

// DiskMetrics описывает метрики использования диска
type DiskMetrics struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
	Path        string  `json:"path"`
}

// HostMetrics описывает информацию о хосте
type HostMetrics struct {
	Hostname        string `json:"hostname"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platform_version"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
	Uptime          uint64 `json:"uptime"`
}

// GoMetrics описывает метрики Go runtime
type GoMetrics struct {
	Version    string `json:"version"`
	Goroutines int    `json:"goroutines"`
	HeapAlloc  uint64 `json:"heap_alloc"`
	HeapSys    uint64 `json:"heap_sys"`
	NumGC      uint32 `json:"num_gc"`
	GOMAXPROCS int    `json:"gomaxprocs"`
}

// TemperatureInfo описывает данные датчика температуры
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

	// Загрузка CPU по ядрам
	cpuPerCore, err := cpu.Percent(100*time.Millisecond, true)
	if err == nil {
		metrics.CPU.PerCore = cpuPerCore
	}

	metrics.CPU.Cores = runtime.NumCPU()

	// Модель CPU
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

	// Метрики диска - пробуем несколько путей, чтобы найти основной системный диск
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

	// Информация о хосте
	hostInfo, err := host.Info()
	if err == nil {
		metrics.Host.Hostname = hostInfo.Hostname
		metrics.Host.Platform = hostInfo.Platform
		metrics.Host.PlatformVersion = hostInfo.PlatformVersion
		metrics.Host.OS = hostInfo.OS
		metrics.Host.Arch = hostInfo.KernelArch
		metrics.Host.Uptime = hostInfo.Uptime
	}

	// Метрики Go runtime
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	metrics.Go.Version = runtime.Version()
	metrics.Go.Goroutines = runtime.NumGoroutine()
	metrics.Go.HeapAlloc = memStats.HeapAlloc
	metrics.Go.HeapSys = memStats.HeapSys
	metrics.Go.NumGC = memStats.NumGC
	metrics.Go.GOMAXPROCS = runtime.GOMAXPROCS(0)

	// Датчики температуры (доступны не на всех системах)
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

	// Базовая проверка ресурсов
	vmStat, err := mem.VirtualMemory()
	if err == nil {
		if vmStat.UsedPercent > 90 {
			health["status"] = "warning"
			health["warning"] = "high memory usage"
		}
	}

	writeJSON(w, http.StatusOK, health)
}
