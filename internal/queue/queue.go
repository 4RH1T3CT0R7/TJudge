package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/cache"
	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"go.uber.org/zap"
)

// QueueManager - очереди матчей по приоритетам с ротацией 5:3:1,
// чтобы low не голодала когда high постоянно забита
type QueueManager struct {
	cache             *cache.Cache
	log               *logger.Logger
	metrics           *metrics.Metrics
	lastMetricsUpdate time.Time
	metricsMu         sync.Mutex

	// счётчик выборок для ротации приоритетов
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
func (qm *QueueManager) getQueueKey(priority models.MatchPriority) string {
	return fmt.Sprintf("queue:%s", priority)
}

// на каждый матч свой ключ дедупа со своим ttl.
// раньше был один общий SET и ttl обновлялся сразу на всех при каждом SADD,
// ключи копились без конца. теперь у каждого матча свой ключ
const dedupPrefix = "queue:dedup:"

const dedupTTL = 24 * time.Hour

func dedupKeyFor(matchID string) string {
	return dedupPrefix + matchID
}

// queuedMatch - запись очереди: матч плюс время постановки для QueueWaitTime.
// у записей без enqueued_at (старый формат) время ожидания не пишется
type queuedMatch struct {
	models.Match
	EnqueuedAt time.Time `json:"enqueued_at"`
}

// Enqueue кладёт матч в очередь по его приоритету
func (qm *QueueManager) Enqueue(ctx context.Context, match *models.Match) error {
	// setnx создаёт ключ дедупа только если его ещё нет
	matchIDStr := match.ID.String()
	isNew, err := qm.cache.SetNX(ctx, dedupKeyFor(matchIDStr), "1", dedupTTL)
	if err != nil {
		qm.log.LogError("Failed to check dedup key", err,
			zap.String("match_id", matchIDStr),
		)
		// на ошибке дедупа падать не стоит - лучше дубль чем потерять матч
	} else if !isNew {
		qm.log.Debug("Match already enqueued, skipping",
			zap.String("match_id", matchIDStr),
		)
		return nil
	}

	data, err := json.Marshal(queuedMatch{Match: *match, EnqueuedAt: time.Now()})
	if err != nil {
		return fmt.Errorf("failed to marshal match: %w", err)
	}

	queueKey := qm.getQueueKey(match.Priority)
	if err := qm.cache.LPush(ctx, queueKey, data); err != nil {
		// lpush упал - дедуп откатывается, иначе матч навсегда "в очереди" и не переставится
		if delErr := qm.cache.Del(ctx, dedupKeyFor(matchIDStr)); delErr != nil {
			qm.log.LogError("Failed to rollback dedup entry on enqueue failure", delErr,
				zap.String("match_id", matchIDStr),
			)
		}
		return fmt.Errorf("failed to enqueue match: %w", err)
	}

	qm.updateQueueSizeMetrics(ctx)

	qm.log.Debug("Match enqueued",
		zap.String("match_id", match.ID.String()),
		zap.String("priority", string(match.Priority)),
	)

	return nil
}

// простая ротация чтобы low очередь не голодала:
// 5 раз подряд сначала берётся high, потом 3 раза medium, потом 1 раз low
func (qm *QueueManager) weightedQueueKeys() []string {
	qm.dequeueMu.Lock()
	pos := qm.dequeueCount % 9
	qm.dequeueCount++
	qm.dequeueMu.Unlock()

	high := qm.getQueueKey(models.PriorityHigh)
	medium := qm.getQueueKey(models.PriorityMedium)
	low := qm.getQueueKey(models.PriorityLow)

	switch {
	case pos < 5:
		return []string{high, medium, low}
	case pos < 8:
		return []string{medium, high, low}
	default:
		return []string{low, high, medium}
	}
}

// EnqueueBatch - то же самое но пачкой, дедуп и lpush одним пайплайном
func (qm *QueueManager) EnqueueBatch(ctx context.Context, matches []*models.Match) error {
	if len(matches) == 0 {
		return nil
	}

	// batch-дедуп одним RTT вместо N
	dedupKeys := make(map[string]any, len(matches))
	for _, match := range matches {
		dedupKeys[dedupKeyFor(match.ID.String())] = "1"
	}

	dedupResults, err := qm.cache.BatchSetNX(ctx, dedupKeys, dedupTTL)
	if err != nil {
		qm.log.LogError("Failed batch dedup check, enqueuing all matches", err)
		// подчищается что успело выставиться и всё валится в очередь без дедупа
		for key := range dedupKeys {
			_ = qm.cache.Del(ctx, key)
		}
		dedupResults = nil
	}

	grouped := make(map[string][]any)
	var addedToDedup []string
	var skipped int
	now := time.Now()

	for _, match := range matches {
		key := dedupKeyFor(match.ID.String())
		// если пайплайн отработал - берётся результат, иначе дедуп пропускается
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

		data, err := json.Marshal(queuedMatch{Match: *match, EnqueuedAt: now})
		if err != nil {
			return fmt.Errorf("failed to marshal match %s: %w", match.ID, err)
		}

		queueKey := qm.getQueueKey(match.Priority)
		grouped[queueKey] = append(grouped[queueKey], data)
	}

	if len(grouped) == 0 {
		qm.log.Debug("All matches already enqueued, skipping batch",
			zap.Int("skipped", skipped),
		)
		return nil
	}

	if err := qm.cache.BatchLPush(ctx, grouped); err != nil {
		// тот же откат что в Enqueue, только пачкой
		for _, dedupKey := range addedToDedup {
			if delErr := qm.cache.Del(ctx, dedupKey); delErr != nil {
				qm.log.LogError("Failed to rollback dedup entry on batch enqueue failure", delErr,
					zap.String("dedup_key", dedupKey),
				)
			}
		}
		return fmt.Errorf("failed to batch enqueue matches: %w", err)
	}

	// обновление метрик
	qm.updateQueueSizeMetrics(ctx)

	enqueued := len(matches) - skipped
	qm.log.Info("Matches batch enqueued",
		zap.Int("enqueued", enqueued),
		zap.Int("skipped_duplicates", skipped),
	)

	return nil
}

// Dequeue достаёт матч с учётом ротации 5:3:1
func (qm *QueueManager) Dequeue(ctx context.Context) (*models.Match, error) {
	queueKeys := qm.weightedQueueKeys()

	// таймаут 2с (не 1) чтобы реже дёргать редис на пустой очереди.
	// brpop блокируется на стороне редиса, cpu воркера не жрёт,
	// прерывается по ctx-cancel и любым lpush в один из ключей
	result, err := qm.cache.BRPop(ctx, 2*time.Second, queueKeys...)
	if err != nil {
		return nil, fmt.Errorf("failed to dequeue match: %w", err)
	}

	// все очереди пустые: гейдж размера тоже должен дойти до нуля
	if result == nil {
		qm.updateQueueSizeMetrics(ctx)
		return nil, nil
	}

	// элемент из Redis уже снят. go-redis не прерывает BRPOP по отмене ctx,
	// и воркер, снятый во время ожидания, всё равно получает матч: учёт после
	// выборки (dedup-ключ, dead-letter, метрики) идёт без отмены
	ctx = context.WithoutCancel(ctx)

	// result[0] - имя очереди, result[1] - данные
	var entry queuedMatch
	if err := json.Unmarshal([]byte(result[1]), &entry); err != nil {
		// битый json - в dead-letter, разбор руками потом
		deadLetterKey := "queue:dead_letter"
		if dlErr := qm.cache.LPush(ctx, deadLetterKey, result[1]); dlErr != nil {
			qm.log.Error("Failed to push to dead-letter queue", zap.Error(dlErr))
		} else {
			qm.metrics.RecordQueueDeadLetterPush("unmarshal_error")
			// dead-letter живёт не больше 1000 записей и 7 дней
			if trimErr := qm.cache.LTrim(ctx, deadLetterKey, 0, 999); trimErr != nil {
				qm.log.Error("Failed to LTRIM dead-letter queue",
					zap.Error(trimErr),
					zap.String("key", deadLetterKey),
				)
			}
			if expErr := qm.cache.Expire(ctx, deadLetterKey, 7*24*time.Hour); expErr != nil {
				qm.log.Error("Failed to set EXPIRE on dead-letter queue",
					zap.Error(expErr),
					zap.String("key", deadLetterKey),
				)
			}
			if size, llErr := qm.cache.LLen(ctx, deadLetterKey); llErr == nil {
				qm.metrics.SetQueueDeadLetterSize(size)
			}
		}
		// сырые данные в логе обрезаются, а то мало ли что там (log injection)
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
	match := entry.Match
	if !entry.EnqueuedAt.IsZero() {
		qm.metrics.RecordQueueWait(string(match.Priority), time.Since(entry.EnqueuedAt))
	}

	// удаление dedup-ключа, чтобы матч мог быть повторно поставлен в очередь в будущем
	if err := qm.cache.Del(ctx, dedupKeyFor(match.ID.String())); err != nil {
		qm.log.LogError("Failed to remove dedup key after dequeue", err,
			zap.String("match_id", match.ID.String()),
		)
	}

	// обновление метрик
	qm.updateQueueSizeMetrics(ctx)

	qm.log.Debug("Match dequeued",
		zap.String("match_id", match.ID.String()),
		zap.String("priority", string(match.Priority)),
	)

	return &match, nil
}

// GetQueueSize получает размер очереди по приоритету
func (qm *QueueManager) GetQueueSize(ctx context.Context, priority models.MatchPriority) (int64, error) {
	queueKey := qm.getQueueKey(priority)
	return qm.cache.LLen(ctx, queueKey)
}

// GetTotalQueueSize получает общий размер всех очередей
func (qm *QueueManager) GetTotalQueueSize(ctx context.Context) (int64, error) {
	var total int64

	priorities := []models.MatchPriority{
		models.PriorityHigh,
		models.PriorityMedium,
		models.PriorityLow,
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

// updateQueueSizeMetrics - обновляет гейджи размеров, не чаще раза в секунду
func (qm *QueueManager) updateQueueSizeMetrics(ctx context.Context) {
	// воркер, снятый автоскейлером, выходит из пустого BRPOP с уже отменённым
	// ctx: LLEN по нему только засоряет лог ошибками
	if ctx.Err() != nil {
		return
	}
	qm.metricsMu.Lock()
	if time.Since(qm.lastMetricsUpdate) < time.Second {
		qm.metricsMu.Unlock()
		return
	}
	qm.lastMetricsUpdate = time.Now()
	qm.metricsMu.Unlock()

	priorities := []models.MatchPriority{
		models.PriorityHigh,
		models.PriorityMedium,
		models.PriorityLow,
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

	// заодно размер dead-letter, если не вышло - ну и ладно
	if dlSize, err := qm.cache.LLen(ctx, "queue:dead_letter"); err == nil {
		qm.metrics.SetQueueDeadLetterSize(dlSize)
	}
}

// Clear - снести все очереди (админка)
func (qm *QueueManager) Clear(ctx context.Context) error {
	priorities := []models.MatchPriority{
		models.PriorityHigh,
		models.PriorityMedium,
		models.PriorityLow,
	}

	for _, priority := range priorities {
		queueKey := qm.getQueueKey(priority)
		if err := qm.cache.Del(ctx, queueKey); err != nil {
			return fmt.Errorf("failed to clear queue %s: %w", priority, err)
		}
	}

	// и dedup-ключи заодно
	if err := qm.clearDedupKeys(ctx); err != nil {
		return fmt.Errorf("failed to clear dedup keys: %w", err)
	}

	qm.log.Info("All queues cleared")
	return nil
}

// clearDedupKeys сносит все ключи queue:dedup:* сканом.
// TODO: почистить старые dedup ключи? вроде ttl (24h) и так справляется,
// метод дёргается только из Clear() (админка). cap 10000 итераций от greedy-цикла
func (qm *QueueManager) clearDedupKeys(ctx context.Context) error {
	const maxIterations = 10000
	var cursor uint64
	for range maxIterations {
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
			return nil
		}
	}
	// упёрлись в лимит итераций, что удалили - удалили, остальное само протухнет по ttl
	qm.log.Warn("clearDedupKeys hit max iterations, stopping",
		zap.Int("max_iterations", maxIterations),
	)
	return nil
}

func (qm *QueueManager) Health(ctx context.Context) error {
	_, err := qm.GetTotalQueueSize(ctx)
	return err
}

// QueueStats - размеры очередей для админки
type QueueStats struct {
	High   int64 `json:"high"`
	Medium int64 `json:"medium"`
	Low    int64 `json:"low"`
	Total  int64 `json:"total"`
}

func (qm *QueueManager) GetStats(ctx context.Context) (*QueueStats, error) {
	stats := &QueueStats{}

	high, err := qm.GetQueueSize(ctx, models.PriorityHigh)
	if err != nil {
		return nil, err
	}
	stats.High = high

	medium, err := qm.GetQueueSize(ctx, models.PriorityMedium)
	if err != nil {
		return nil, err
	}
	stats.Medium = medium

	low, err := qm.GetQueueSize(ctx, models.PriorityLow)
	if err != nil {
		return nil, err
	}
	stats.Low = low

	stats.Total = stats.High + stats.Medium + stats.Low
	return stats, nil
}

func (qm *QueueManager) GetDeadLetterSize(ctx context.Context) (int64, error) {
	return qm.cache.LLen(ctx, "queue:dead_letter")
}

// ClearDeadLetter чистит dead-letter, возвращает сколько удалили
func (qm *QueueManager) ClearDeadLetter(ctx context.Context) (int64, error) {
	size, err := qm.cache.LLen(ctx, "queue:dead_letter")
	if err != nil {
		return 0, err
	}
	if err := qm.cache.Del(ctx, "queue:dead_letter"); err != nil {
		return 0, err
	}
	qm.metrics.SetQueueDeadLetterSize(0)
	return size, nil
}

// PurgeInvalidMatches выкидывает из очередей матчи которых уже нет в бд (валидатор проверяет)
func (qm *QueueManager) PurgeInvalidMatches(ctx context.Context, validator func(matchID string) bool) (int64, error) {
	var purged int64

	priorities := []models.MatchPriority{
		models.PriorityHigh,
		models.PriorityMedium,
		models.PriorityLow,
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

// purgeQueueInvalidMatches чистит одну очередь.
// между LRange и ReplaceList есть окно где новые элементы могут потеряться,
// но purge это админка и не гоняется во время активной обработки, так что ок
func (qm *QueueManager) purgeQueueInvalidMatches(ctx context.Context, priority models.MatchPriority, validator func(matchID string) bool) (int64, error) {
	queueKey := qm.getQueueKey(priority)

	items, err := qm.cache.LRange(ctx, queueKey, 0, -1)
	if err != nil {
		return 0, fmt.Errorf("failed to get queue items: %w", err)
	}

	if len(items) == 0 {
		return 0, nil
	}

	// остаются только те что есть в бд, записи переносятся как есть
	var validMatches [][]byte
	var purgedCount int64

	for _, item := range items {
		var match models.Match
		if err := json.Unmarshal([]byte(item), &match); err != nil {
			// невалидный JSON - пропускается
			purgedCount++
			continue
		}

		if validator(match.ID.String()) {
			validMatches = append(validMatches, []byte(item))
		} else {
			purgedCount++
		}
	}

	// ничего не выкинули - и очередь не трогается
	if purgedCount == 0 {
		return 0, nil
	}

	// очередь заменяется целиком в одной транзакции. порядок разворачивается,
	// чтобы после lpush он остался как был
	reversed := make([][]byte, len(validMatches))
	for i, v := range validMatches {
		reversed[len(validMatches)-1-i] = v
	}
	if err := qm.cache.ReplaceList(ctx, queueKey, reversed); err != nil {
		return 0, fmt.Errorf("failed to atomically replace queue: %w", err)
	}

	return purgedCount, nil
}
