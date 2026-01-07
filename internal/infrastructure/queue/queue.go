package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/cache"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/metrics"
	"go.uber.org/zap"
)

// QueueManager управляет очередями матчей с приоритетами
type QueueManager struct {
	cache   *cache.Cache
	log     *logger.Logger
	metrics *metrics.Metrics
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

// Enqueue добавляет матч в очередь с учётом приоритета
func (qm *QueueManager) Enqueue(ctx context.Context, match *domain.Match) error {
	// Сериализуем матч
	data, err := json.Marshal(match)
	if err != nil {
		return fmt.Errorf("failed to marshal match: %w", err)
	}

	// Добавляем в соответствующую очередь
	queueKey := qm.getQueueKey(match.Priority)
	if err := qm.cache.LPush(ctx, queueKey, data); err != nil {
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

// Dequeue извлекает матч из очереди с учётом приоритета
// Проверяет очереди в порядке: HIGH -> MEDIUM -> LOW
func (qm *QueueManager) Dequeue(ctx context.Context) (*domain.Match, error) {
	// Используем multi-key BRPOP для эффективного ожидания на всех очередях
	// Redis вернёт первый доступный элемент из любой очереди (в порядке приоритета)
	queueKeys := []string{
		qm.getQueueKey(domain.PriorityHigh),
		qm.getQueueKey(domain.PriorityMedium),
		qm.getQueueKey(domain.PriorityLow),
	}

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
		qm.log.LogError("Failed to unmarshal match", err)
		return nil, fmt.Errorf("failed to unmarshal match: %w", err)
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

// updateQueueSizeMetrics обновляет метрики размеров очередей
func (qm *QueueManager) updateQueueSizeMetrics(ctx context.Context) {
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

	qm.log.Info("All queues cleared")
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

// purgeQueueInvalidMatches очищает одну очередь от невалидных матчей
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

	// Очищаем очередь и добавляем только валидные матчи
	if err := qm.cache.Del(ctx, queueKey); err != nil {
		return 0, fmt.Errorf("failed to clear queue: %w", err)
	}

	// Добавляем валидные матчи обратно (в обратном порядке для сохранения очерёдности)
	for i := len(validMatches) - 1; i >= 0; i-- {
		if err := qm.cache.LPush(ctx, queueKey, validMatches[i]); err != nil {
			qm.log.LogError("Failed to re-enqueue valid match", err)
		}
	}

	return purgedCount, nil
}
