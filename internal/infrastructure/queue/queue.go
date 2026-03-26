package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/cache"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/metrics"
	"go.uber.org/zap"
)

// QueueManager управляет очередями матчей с приоритетами.
// Использует weighted fair queueing для предотвращения starvation
// очередей с низким приоритетом (соотношение 5:3:1 для HIGH:MEDIUM:LOW).
type QueueManager struct {
	cache             *cache.Cache
	log               *logger.Logger
	metrics           *metrics.Metrics
	lastMetricsUpdate time.Time
	metricsMu         sync.Mutex

	// Weighted fair queueing: счётчик dequeue-операций для ротации приоритетов.
	// Каждые 5 подряд выборок из HIGH переключаемся на MEDIUM (3 выборки),
	// затем на LOW (1 выборка), после чего цикл повторяется.
	dequeueMu    sync.Mutex
	dequeueCount int
}

// NewQueueManager создаёт новый менеджер очередей
func NewQueueManager(cache *cache.Cache, log *logger.Logger, m *metrics.Metrics) *QueueManager {
	return &QueueManager{
		cache:   cache,
		log:     log,
		metrics: m,
	}
}

// getQueueKey возвращает ключ для очереди по приоритету
func (qm *QueueManager) getQueueKey(priority domain.MatchPriority) string {
	return fmt.Sprintf("queue:%s", priority)
}

// dedupPrefix is the key prefix for per-match deduplication keys.
// Each match gets its own key "queue:dedup:{matchID}" with an independent TTL,
// preventing the old shared-SET problem where active queues reset the TTL
// for ALL entries on every SADD, causing unbounded growth.
const dedupPrefix = "queue:dedup:"

// dedupTTL is the TTL for each individual deduplication key
const dedupTTL = 24 * time.Hour

// dedupKeyFor returns the per-match deduplication key
func dedupKeyFor(matchID string) string {
	return dedupPrefix + matchID
}

// Enqueue добавляет матч в очередь с учётом приоритета
func (qm *QueueManager) Enqueue(ctx context.Context, match *domain.Match) error {
	// Атомарно проверяем дедупликацию: SetNX создаёт ключ только если его нет
	matchIDStr := match.ID.String()
	isNew, err := qm.cache.SetNX(ctx, dedupKeyFor(matchIDStr), "1", dedupTTL)
	if err != nil {
		qm.log.LogError("Failed to check dedup key", err,
			zap.String("match_id", matchIDStr),
		)
		// Продолжаем даже при ошибке дедупликации — лучше дублировать, чем потерять
	} else if !isNew {
		qm.log.Info("Match already enqueued, skipping",
			zap.String("match_id", matchIDStr),
		)
		return nil
	}

	// Сериализуем матч
	data, err := json.Marshal(match)
	if err != nil {
		return fmt.Errorf("failed to marshal match: %w", err)
	}

	// Добавляем в соответствующую очередь
	queueKey := qm.getQueueKey(match.Priority)
	if err := qm.cache.LPush(ctx, queueKey, data); err != nil {
		// Откатываем запись в dedup, так как матч не был добавлен в очередь
		if delErr := qm.cache.Del(ctx, dedupKeyFor(matchIDStr)); delErr != nil {
			qm.log.LogError("Failed to rollback dedup entry on enqueue failure", delErr,
				zap.String("match_id", matchIDStr),
			)
		}
		return fmt.Errorf("failed to enqueue match: %w", err)
	}

	// Обновляем метрики
	qm.updateQueueSizeMetrics(ctx)

	qm.log.Info("Match enqueued",
		zap.String("match_id", match.ID.String()),
		zap.String("priority", string(match.Priority)),
	)

	return nil
}

// weightedQueueKeys возвращает ключи очередей, упорядоченные по приоритету
// с учётом weighted fair queueing для предотвращения starvation.
//
// Цикл из 9 итераций (5+3+1):
//   - Итерации 0-4: HIGH первый  → H, M, L
//   - Итерации 5-7: MEDIUM первый → M, H, L
//   - Итерация 8:   LOW первый   → L, H, M
//
// Это гарантирует, что MEDIUM/LOW очереди проверяются первыми
// как минимум 4/9 ≈ 44% времени, предотвращая starvation.
func (qm *QueueManager) weightedQueueKeys() []string {
	qm.dequeueMu.Lock()
	pos := qm.dequeueCount % 9
	qm.dequeueCount++
	qm.dequeueMu.Unlock()

	high := qm.getQueueKey(domain.PriorityHigh)
	medium := qm.getQueueKey(domain.PriorityMedium)
	low := qm.getQueueKey(domain.PriorityLow)

	switch {
	case pos < 5:
		return []string{high, medium, low}
	case pos < 8:
		return []string{medium, high, low}
	default:
		return []string{low, high, medium}
	}
}

// EnqueueBatch добавляет несколько матчей в очереди.
// Использует Redis pipeline для batch dedup-проверки (один RTT вместо N).
// Финальный LPUSH также выполняется pipeline-запросом.
// При ошибке BatchLPush записи в dedup откатываются.
func (qm *QueueManager) EnqueueBatch(ctx context.Context, matches []*domain.Match) error {
	if len(matches) == 0 {
		return nil
	}

	// Batch dedup-проверка через pipeline (один RTT вместо N)
	dedupKeys := make(map[string]interface{}, len(matches))
	for _, match := range matches {
		dedupKeys[dedupKeyFor(match.ID.String())] = "1"
	}

	dedupResults, err := qm.cache.BatchSetNX(ctx, dedupKeys, dedupTTL)
	if err != nil {
		qm.log.LogError("Failed batch dedup check, enqueuing all matches", err)
		// Cleanup any partially-set dedup keys before falling through
		for key := range dedupKeys {
			_ = qm.cache.Del(ctx, key)
		}
		dedupResults = nil
	}

	grouped := make(map[string][]interface{})
	var addedToDedup []string
	var skipped int

	for _, match := range matches {
		key := dedupKeyFor(match.ID.String())
		// Если pipeline работал — проверяем результат, иначе пропускаем dedup
		if dedupResults != nil {
			isNew, ok := dedupResults[key]
			if ok && !isNew {
				skipped++
				continue
			}
			if ok && isNew {
				addedToDedup = append(addedToDedup, key)
			}
		}

		data, err := json.Marshal(match)
		if err != nil {
			return fmt.Errorf("failed to marshal match %s: %w", match.ID, err)
		}

		queueKey := qm.getQueueKey(match.Priority)
		grouped[queueKey] = append(grouped[queueKey], data)
	}

	if len(grouped) == 0 {
		qm.log.Info("All matches already enqueued, skipping batch",
			zap.Int("skipped", skipped),
		)
		return nil
	}

	// Batch LPUSH через pipeline
	if err := qm.cache.BatchLPush(ctx, grouped); err != nil {
		// Откатываем записи в dedup, иначе матчи будут считаться
		// "уже в очереди" хотя реально туда не попали
		for _, dedupKey := range addedToDedup {
			if delErr := qm.cache.Del(ctx, dedupKey); delErr != nil {
				qm.log.LogError("Failed to rollback dedup entry on batch enqueue failure", delErr,
					zap.String("dedup_key", dedupKey),
				)
			}
		}
		return fmt.Errorf("failed to batch enqueue matches: %w", err)
	}

	// Обновляем метрики
	qm.updateQueueSizeMetrics(ctx)

	enqueued := len(matches) - skipped
	qm.log.Info("Matches batch enqueued",
		zap.Int("enqueued", enqueued),
		zap.Int("skipped_duplicates", skipped),
	)

	return nil
}

// Dequeue извлекает матч из очереди с учётом приоритета.
// Использует weighted fair queueing (5:3:1) для предотвращения starvation
// очередей MEDIUM и LOW при постоянно заполненной HIGH.
func (qm *QueueManager) Dequeue(ctx context.Context) (*domain.Match, error) {
	// Используем multi-key BRPOP с ротируемым порядком ключей
	queueKeys := qm.weightedQueueKeys()

	// Блокирующее чтение с таймаутом 1 секунда на все очереди сразу
	result, err := qm.cache.BRPop(ctx, time.Second, queueKeys...)
	if err != nil {
		return nil, fmt.Errorf("failed to dequeue match: %w", err)
	}

	// Если все очереди пустые
	if result == nil {
		return nil, nil
	}

	// result[0] содержит имя очереди, result[1] - данные
	var match domain.Match
	if err := json.Unmarshal([]byte(result[1]), &match); err != nil {
		// Push to dead-letter queue for manual inspection
		deadLetterKey := "queue:dead_letter"
		if dlErr := qm.cache.LPush(ctx, deadLetterKey, result[1]); dlErr != nil {
			qm.log.Error("Failed to push to dead-letter queue", zap.Error(dlErr))
		} else {
			// Cap dead-letter queue to 1000 entries and set 7-day TTL
			_ = qm.cache.LTrim(ctx, deadLetterKey, 0, 999)
			_ = qm.cache.Expire(ctx, deadLetterKey, 7*24*time.Hour)
		}
		// Truncate raw data for logging to prevent log injection
		rawData := result[1]
		if len(rawData) > 1024 {
			rawData = rawData[:1024] + "...(truncated)"
		}
		qm.log.Error("Failed to unmarshal match, moved to dead-letter queue",
			zap.Error(err),
			zap.String("raw_data", rawData),
			zap.String("queue_key", result[0]),
		)
		return nil, fmt.Errorf("failed to unmarshal match: %w", err)
	}

	// Удаляем dedup-ключ, чтобы матч мог быть повторно поставлен в очередь в будущем
	if err := qm.cache.Del(ctx, dedupKeyFor(match.ID.String())); err != nil {
		qm.log.LogError("Failed to remove dedup key after dequeue", err,
			zap.String("match_id", match.ID.String()),
		)
	}

	// Обновляем метрики
	qm.updateQueueSizeMetrics(ctx)

	qm.log.Info("Match dequeued",
		zap.String("match_id", match.ID.String()),
		zap.String("priority", string(match.Priority)),
	)

	return &match, nil
}

// GetQueueSize получает размер очереди по приоритету
func (qm *QueueManager) GetQueueSize(ctx context.Context, priority domain.MatchPriority) (int64, error) {
	queueKey := qm.getQueueKey(priority)
	return qm.cache.LLen(ctx, queueKey)
}

// GetTotalQueueSize получает общий размер всех очередей
func (qm *QueueManager) GetTotalQueueSize(ctx context.Context) (int64, error) {
	var total int64

	priorities := []domain.MatchPriority{
		domain.PriorityHigh,
		domain.PriorityMedium,
		domain.PriorityLow,
	}

	for _, priority := range priorities {
		size, err := qm.GetQueueSize(ctx, priority)
		if err != nil {
			return 0, err
		}
		total += size
	}

	return total, nil
}

// updateQueueSizeMetrics обновляет метрики размеров очередей (max once per second)
func (qm *QueueManager) updateQueueSizeMetrics(ctx context.Context) {
	qm.metricsMu.Lock()
	if time.Since(qm.lastMetricsUpdate) < time.Second {
		qm.metricsMu.Unlock()
		return
	}
	qm.lastMetricsUpdate = time.Now()
	qm.metricsMu.Unlock()

	priorities := []domain.MatchPriority{
		domain.PriorityHigh,
		domain.PriorityMedium,
		domain.PriorityLow,
	}

	for _, priority := range priorities {
		size, err := qm.GetQueueSize(ctx, priority)
		if err != nil {
			qm.log.LogError("Failed to get queue size", err,
				zap.String("priority", string(priority)),
			)
			continue
		}
		qm.metrics.SetQueueSize(string(priority), int(size))
	}
}

// Clear очищает все очереди
func (qm *QueueManager) Clear(ctx context.Context) error {
	priorities := []domain.MatchPriority{
		domain.PriorityHigh,
		domain.PriorityMedium,
		domain.PriorityLow,
	}

	for _, priority := range priorities {
		queueKey := qm.getQueueKey(priority)
		if err := qm.cache.Del(ctx, queueKey); err != nil {
			return fmt.Errorf("failed to clear queue %s: %w", priority, err)
		}
	}

	// Очищаем dedup-ключи по паттерну
	if err := qm.clearDedupKeys(ctx); err != nil {
		return fmt.Errorf("failed to clear dedup keys: %w", err)
	}

	qm.log.Info("All queues cleared")
	return nil
}

// clearDedupKeys удаляет все dedup-ключи по паттерну queue:dedup:*
func (qm *QueueManager) clearDedupKeys(ctx context.Context) error {
	var cursor uint64
	for {
		keys, nextCursor, err := qm.cache.Scan(ctx, cursor, dedupPrefix+"*", 100)
		if err != nil {
			return fmt.Errorf("failed to scan dedup keys: %w", err)
		}
		if len(keys) > 0 {
			if err := qm.cache.Del(ctx, keys...); err != nil {
				return fmt.Errorf("failed to delete dedup keys: %w", err)
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

// Health проверяет здоровье очередей
func (qm *QueueManager) Health(ctx context.Context) error {
	// Проверяем, что можем получить размеры очередей
	_, err := qm.GetTotalQueueSize(ctx)
	return err
}

// QueueStats статистика очередей
type QueueStats struct {
	High   int64 `json:"high"`
	Medium int64 `json:"medium"`
	Low    int64 `json:"low"`
	Total  int64 `json:"total"`
}

// GetStats возвращает статистику всех очередей
func (qm *QueueManager) GetStats(ctx context.Context) (*QueueStats, error) {
	stats := &QueueStats{}

	high, err := qm.GetQueueSize(ctx, domain.PriorityHigh)
	if err != nil {
		return nil, err
	}
	stats.High = high

	medium, err := qm.GetQueueSize(ctx, domain.PriorityMedium)
	if err != nil {
		return nil, err
	}
	stats.Medium = medium

	low, err := qm.GetQueueSize(ctx, domain.PriorityLow)
	if err != nil {
		return nil, err
	}
	stats.Low = low

	stats.Total = stats.High + stats.Medium + stats.Low
	return stats, nil
}

// PurgeInvalidMatches удаляет из очереди матчи, которых нет в БД
// Принимает функцию-валидатор, которая проверяет существование матча
// Возвращает количество удалённых матчей
func (qm *QueueManager) PurgeInvalidMatches(ctx context.Context, validator func(matchID string) bool) (int64, error) {
	var purged int64

	priorities := []domain.MatchPriority{
		domain.PriorityHigh,
		domain.PriorityMedium,
		domain.PriorityLow,
	}

	for _, priority := range priorities {
		count, err := qm.purgeQueueInvalidMatches(ctx, priority, validator)
		if err != nil {
			qm.log.LogError("Failed to purge queue", err,
				zap.String("priority", string(priority)),
			)
			continue
		}
		purged += count
	}

	qm.log.Info("Purged invalid matches from queues",
		zap.Int64("purged_count", purged),
	)

	return purged, nil
}

// purgeQueueInvalidMatches очищает одну очередь от невалидных матчей.
// NOTE: There is a small window between LRange and ReplaceList where
// newly enqueued items could be lost. This is acceptable because purge
// is an admin-only operation that should not run during active match processing.
func (qm *QueueManager) purgeQueueInvalidMatches(ctx context.Context, priority domain.MatchPriority, validator func(matchID string) bool) (int64, error) {
	queueKey := qm.getQueueKey(priority)

	// Получаем все элементы очереди
	items, err := qm.cache.LRange(ctx, queueKey, 0, -1)
	if err != nil {
		return 0, fmt.Errorf("failed to get queue items: %w", err)
	}

	if len(items) == 0 {
		return 0, nil
	}

	// Собираем валидные матчи
	var validMatches [][]byte
	var purgedCount int64

	for _, item := range items {
		var match domain.Match
		if err := json.Unmarshal([]byte(item), &match); err != nil {
			// Невалидный JSON - пропускаем
			purgedCount++
			continue
		}

		// Проверяем существование матча
		if validator(match.ID.String()) {
			data, _ := json.Marshal(match)
			validMatches = append(validMatches, data)
		} else {
			purgedCount++
		}
	}

	// Если ничего не изменилось - выходим
	if purgedCount == 0 {
		return 0, nil
	}

	// Atomically replace the queue: DEL + LPUSH in a single MULTI/EXEC transaction.
	// Reverse the order so that after LPUSH the queue preserves the original ordering.
	reversed := make([][]byte, len(validMatches))
	for i, v := range validMatches {
		reversed[len(validMatches)-1-i] = v
	}
	if err := qm.cache.ReplaceList(ctx, queueKey, reversed); err != nil {
		return 0, fmt.Errorf("failed to atomically replace queue: %w", err)
	}

	return purgedCount, nil
}
