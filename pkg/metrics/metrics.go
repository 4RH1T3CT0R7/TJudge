package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	instance *Metrics
	once     sync.Once
)

// Metrics содержит все метрики приложения
type Metrics struct {
	// Match метрики
	MatchesTotal      *prometheus.CounterVec
	MatchDuration     *prometheus.HistogramVec
	MatchesInProgress prometheus.Gauge

	// Queue метрики
	QueueSize           *prometheus.GaugeVec
	QueueWaitTime       *prometheus.HistogramVec
	QueueDeadLetterSize prometheus.Gauge
	QueueDeadLetterPush *prometheus.CounterVec

	// Worker метрики
	ActiveWorkers        prometheus.Gauge
	WorkerPoolSize       prometheus.Gauge
	WorkerDraining       prometheus.Gauge
	WorkerDrainDuration  prometheus.Histogram
	WorkerInFlightOnStop prometheus.Gauge

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

// New создаёт или возвращает существующий экземпляр метрик (singleton)
func New() *Metrics {
	once.Do(func() {
		instance = newMetrics()
	})
	return instance
}

// newMetrics создаёт новый экземпляр метрик (внутренняя функция)
func newMetrics() *Metrics {
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
		QueueDeadLetterSize: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "tjudge_queue_deadletter_size",
				Help: "Current number of entries in the dead-letter queue (poison messages)",
			},
		),
		QueueDeadLetterPush: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "tjudge_queue_deadletter_push_total",
				Help: "Total number of items pushed to the dead-letter queue, labelled by reason",
			},
			[]string{"reason"},
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
		WorkerDraining: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "tjudge_worker_draining",
				Help: "1 when worker pool is in graceful shutdown (draining), 0 otherwise",
			},
		),
		WorkerDrainDuration: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "tjudge_worker_drain_duration_seconds",
				Help:    "Duration of worker pool graceful drain (from Stop() to all workers exited)",
				Buckets: prometheus.ExponentialBuckets(0.1, 2, 12), // 0.1s ... ~400s
			},
		),
		WorkerInFlightOnStop: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "tjudge_worker_in_flight_on_stop",
				Help: "Number of in-flight matches observed at the moment Stop() was called",
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

// SetQueueDeadLetterSize устанавливает текущий размер dead-letter очереди.
// Вызывается из background-поллера или при каждом push.
func (m *Metrics) SetQueueDeadLetterSize(size int64) {
	m.QueueDeadLetterSize.Set(float64(size))
}

// RecordQueueDeadLetterPush инкрементирует счётчик push-ов в dead-letter.
// reason — короткая метка причины (напр. "unmarshal_error", "poison_message").
func (m *Metrics) RecordQueueDeadLetterPush(reason string) {
	m.QueueDeadLetterPush.WithLabelValues(reason).Inc()
}

// SetActiveWorkers устанавливает количество активных воркеров
func (m *Metrics) SetActiveWorkers(count int) {
	m.ActiveWorkers.Set(float64(count))
}

// SetWorkerPoolSize устанавливает размер пула воркеров
func (m *Metrics) SetWorkerPoolSize(size int) {
	m.WorkerPoolSize.Set(float64(size))
}

// SetWorkerDraining выставляет флаг graceful drain (1=draining, 0=running).
func (m *Metrics) SetWorkerDraining(draining bool) {
	if draining {
		m.WorkerDraining.Set(1)
	} else {
		m.WorkerDraining.Set(0)
	}
}

// RecordWorkerDrainDuration фиксирует длительность graceful drain.
func (m *Metrics) RecordWorkerDrainDuration(duration time.Duration) {
	m.WorkerDrainDuration.Observe(duration.Seconds())
}

// SetWorkerInFlightOnStop фиксирует число in-flight матчей на момент Stop().
func (m *Metrics) SetWorkerInFlightOnStop(n int) {
	m.WorkerInFlightOnStop.Set(float64(n))
}

// SetDBConnections устанавливает количество соединений с БД
func (m *Metrics) SetDBConnections(inUse, idle, open int) {
	m.DBConnections.WithLabelValues("in_use").Set(float64(inUse))
	m.DBConnections.WithLabelValues("idle").Set(float64(idle))
	m.DBConnections.WithLabelValues("open").Set(float64(open))
}
