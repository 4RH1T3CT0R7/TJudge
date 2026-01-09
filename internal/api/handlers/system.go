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

// SystemMetrics represents system resource metrics
type SystemMetrics struct {
	CPU         CPUMetrics        `json:"cpu"`
	Memory      MemoryMetrics     `json:"memory"`
	Disk        DiskMetrics       `json:"disk"`
	Host        HostMetrics       `json:"host"`
	Go          GoMetrics         `json:"go"`
	Temperature []TemperatureInfo `json:"temperature,omitempty"`
}

// CPUMetrics represents CPU usage metrics
type CPUMetrics struct {
	UsagePercent float64   `json:"usage_percent"`
	Cores        int       `json:"cores"`
	ModelName    string    `json:"model_name,omitempty"`
	PerCore      []float64 `json:"per_core,omitempty"`
}

// MemoryMetrics represents memory usage metrics
type MemoryMetrics struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
}

// DiskMetrics represents disk usage metrics
type DiskMetrics struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
	Path        string  `json:"path"`
}

// HostMetrics represents host information
type HostMetrics struct {
	Hostname        string `json:"hostname"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platform_version"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
	Uptime          uint64 `json:"uptime"`
}

// GoMetrics represents Go runtime metrics
type GoMetrics struct {
	Version    string `json:"version"`
	Goroutines int    `json:"goroutines"`
	HeapAlloc  uint64 `json:"heap_alloc"`
	HeapSys    uint64 `json:"heap_sys"`
	NumGC      uint32 `json:"num_gc"`
	GOMAXPROCS int    `json:"gomaxprocs"`
}

// TemperatureInfo represents temperature sensor data
type TemperatureInfo struct {
	SensorKey   string  `json:"sensor_key"`
	Temperature float64 `json:"temperature"`
}

// SystemHandler handles system-related API requests
type SystemHandler struct {
	log *logger.Logger
}

// NewSystemHandler creates a new system handler
func NewSystemHandler(log *logger.Logger) *SystemHandler {
	return &SystemHandler{
		log: log,
	}
}

// GetMetrics returns system metrics
// GET /api/v1/system/metrics
func (h *SystemHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := SystemMetrics{}

	// CPU metrics
	cpuPercent, err := cpu.Percent(100*time.Millisecond, false)
	if err == nil && len(cpuPercent) > 0 {
		metrics.CPU.UsagePercent = cpuPercent[0]
	}

	// Per-core CPU usage
	cpuPerCore, err := cpu.Percent(100*time.Millisecond, true)
	if err == nil {
		metrics.CPU.PerCore = cpuPerCore
	}

	metrics.CPU.Cores = runtime.NumCPU()

	// CPU model name
	cpuInfo, err := cpu.Info()
	if err == nil && len(cpuInfo) > 0 {
		metrics.CPU.ModelName = cpuInfo[0].ModelName
	}

	// Memory metrics
	vmStat, err := mem.VirtualMemory()
	if err == nil {
		metrics.Memory.Total = vmStat.Total
		metrics.Memory.Used = vmStat.Used
		metrics.Memory.Free = vmStat.Free
		metrics.Memory.UsedPercent = vmStat.UsedPercent
	}

	// Disk metrics - try multiple paths to find the main system disk
	diskPaths := []string{"/System/Volumes/Data", "/", os.Getenv("HOME")}
	if runtime.GOOS != "darwin" {
		diskPaths = []string{"/"}
	}

	for _, diskPath := range diskPaths {
		diskStat, err := disk.Usage(diskPath)
		if err == nil && diskStat.Total > 50*1024*1024*1024 { // At least 50GB to be considered real disk
			metrics.Disk.Total = diskStat.Total
			metrics.Disk.Used = diskStat.Used
			metrics.Disk.Free = diskStat.Free
			metrics.Disk.UsedPercent = diskStat.UsedPercent
			metrics.Disk.Path = diskPath
			break
		}
	}

	// Host information
	hostInfo, err := host.Info()
	if err == nil {
		metrics.Host.Hostname = hostInfo.Hostname
		metrics.Host.Platform = hostInfo.Platform
		metrics.Host.PlatformVersion = hostInfo.PlatformVersion
		metrics.Host.OS = hostInfo.OS
		metrics.Host.Arch = hostInfo.KernelArch
		metrics.Host.Uptime = hostInfo.Uptime
	}

	// Go runtime metrics
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	metrics.Go.Version = runtime.Version()
	metrics.Go.Goroutines = runtime.NumGoroutine()
	metrics.Go.HeapAlloc = memStats.HeapAlloc
	metrics.Go.HeapSys = memStats.HeapSys
	metrics.Go.NumGC = memStats.NumGC
	metrics.Go.GOMAXPROCS = runtime.GOMAXPROCS(0)

	// Temperature sensors (may not be available on all systems)
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

// GetHealth returns system health status
// GET /api/v1/system/health
func (h *SystemHandler) GetHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"hostname":  "",
		"pid":       os.Getpid(),
	}

	if hostname, err := os.Hostname(); err == nil {
		health["hostname"] = hostname
	}

	// Basic resource check
	vmStat, err := mem.VirtualMemory()
	if err == nil {
		if vmStat.UsedPercent > 90 {
			health["status"] = "warning"
			health["warning"] = "high memory usage"
		}
	}

	writeJSON(w, http.StatusOK, health)
}
