package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics содержит все метрики приложения
type Metrics struct {
	// Match метрики
	MatchesTotal      *prometheus.CounterVec
	MatchDuration     *prometheus.HistogramVec
	MatchesInProgress prometheus.Gauge

	// Queue метрики
	QueueSize     *prometheus.GaugeVec
	QueueWaitTime *prometheus.HistogramVec

	// Worker метрики
	ActiveWorkers  prometheus.Gauge
	WorkerPoolSize prometheus.Gauge

	// HTTP метрики
	HTTPRequestsTotal    *prometheus.CounterVec
	HTTPRequestDuration  *prometheus.HistogramVec
	HTTPRequestsInFlight prometheus.Gauge

	// Database метрики
	DBQueryDuration *prometheus.HistogramVec
	DBConnections   *prometheus.GaugeVec

	// Cache метрики
	CacheHits   *prometheus.CounterVec
	CacheMisses *prometheus.CounterVec
}

// New создаёт новый экземпляр метрик
func New() *Metrics {
	return &Metrics{
		// Match метрики
		MatchesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "tjudge_matches_total",
				Help: "Total number of matches processed",
			},
			[]string{"status", "game_type"},
		),
		MatchDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "tjudge_match_duration_seconds",
				Help:    "Match execution duration in seconds",
				Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 0.1s to ~100s
			},
			[]string{"game_type"},
		),
		MatchesInProgress: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "tjudge_matches_in_progress",
				Help: "Number of matches currently being processed",
			},
		),

		// Queue метрики
		QueueSize: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "tjudge_queue_size",
				Help: "Current queue size",
			},
			[]string{"priority"},
		),
		QueueWaitTime: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "tjudge_queue_wait_time_seconds",
				Help:    "Time spent waiting in queue",
				Buckets: prometheus.ExponentialBuckets(0.01, 2, 10),
			},
			[]string{"priority"},
		),

		// Worker метрики
		ActiveWorkers: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "tjudge_active_workers",
				Help: "Number of active workers",
			},
		),
		WorkerPoolSize: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "tjudge_worker_pool_size",
				Help: "Total size of worker pool",
			},
		),

		// HTTP метрики
		HTTPRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "tjudge_http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		HTTPRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "tjudge_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		HTTPRequestsInFlight: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "tjudge_http_requests_in_flight",
				Help: "Number of HTTP requests currently being served",
			},
		),

		// Database метрики
		DBQueryDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "tjudge_db_query_duration_seconds",
				Help:    "Database query duration in seconds",
				Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
			},
			[]string{"query_type"},
		),
		DBConnections: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "tjudge_db_connections",
				Help: "Number of database connections",
			},
			[]string{"state"}, // "in_use", "idle", "open"
		),

		// Cache метрики
		CacheHits: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "tjudge_cache_hits_total",
				Help: "Total number of cache hits",
			},
			[]string{"cache_type"},
		),
		CacheMisses: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "tjudge_cache_misses_total",
				Help: "Total number of cache misses",
			},
			[]string{"cache_type"},
		),
	}
}

// RecordMatchStart записывает начало матча
func (m *Metrics) RecordMatchStart() {
	m.MatchesInProgress.Inc()
}

// RecordMatchComplete записывает завершение матча
func (m *Metrics) RecordMatchComplete(gameType string, status string, duration time.Duration) {
	m.MatchesInProgress.Dec()
	m.MatchesTotal.WithLabelValues(status, gameType).Inc()
	m.MatchDuration.WithLabelValues(gameType).Observe(duration.Seconds())
}

// RecordHTTPRequest записывает HTTP запрос
func (m *Metrics) RecordHTTPRequest(method, path, status string, duration time.Duration) {
	m.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
	m.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
}

// RecordDBQuery записывает запрос к БД
func (m *Metrics) RecordDBQuery(queryType string, duration time.Duration) {
	m.DBQueryDuration.WithLabelValues(queryType).Observe(duration.Seconds())
}

// RecordCacheHit записывает попадание в кэш
func (m *Metrics) RecordCacheHit(cacheType string) {
	m.CacheHits.WithLabelValues(cacheType).Inc()
}

// RecordCacheMiss записывает промах кэша
func (m *Metrics) RecordCacheMiss(cacheType string) {
	m.CacheMisses.WithLabelValues(cacheType).Inc()
}

// SetQueueSize устанавливает размер очереди
func (m *Metrics) SetQueueSize(priority string, size int) {
	m.QueueSize.WithLabelValues(priority).Set(float64(size))
}

// SetActiveWorkers устанавливает количество активных воркеров
func (m *Metrics) SetActiveWorkers(count int) {
	m.ActiveWorkers.Set(float64(count))
}

// SetWorkerPoolSize устанавливает размер пула воркеров
func (m *Metrics) SetWorkerPoolSize(size int) {
	m.WorkerPoolSize.Set(float64(size))
}

// SetDBConnections устанавливает количество соединений с БД
func (m *Metrics) SetDBConnections(inUse, idle, open int) {
	m.DBConnections.WithLabelValues("in_use").Set(float64(inUse))
	m.DBConnections.WithLabelValues("idle").Set(float64(idle))
	m.DBConnections.WithLabelValues("open").Set(float64(open))
}
