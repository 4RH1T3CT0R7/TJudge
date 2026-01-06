package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/metrics"
	"github.com/google/uuid"
)

// MatchCache - кэш для результатов матчей
type MatchCache struct {
	cache   *Cache
	ttl     time.Duration
	metrics *metrics.Metrics
}

// NewMatchCache создаёт новый кэш для матчей
func NewMatchCache(cache *Cache) *MatchCache {
	return &MatchCache{
		cache:   cache,
		ttl:     24 * time.Hour, // кэшируем результаты на 24 часа
		metrics: nil,            // metrics опциональны
	}
}

// WithMetrics добавляет метрики в кэш
func (mc *MatchCache) WithMetrics(m *metrics.Metrics) *MatchCache {
	mc.metrics = m
	return mc
}

// getKey возвращает ключ для матча
func (mc *MatchCache) getKey(matchID uuid.UUID) string {
	return fmt.Sprintf("match:%s", matchID.String())
}

// Set сохраняет результат матча в кэш
func (mc *MatchCache) Set(ctx context.Context, matchID uuid.UUID, result *domain.MatchResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal match result: %w", err)
	}

	key := mc.getKey(matchID)
	return mc.cache.Set(ctx, key, data, mc.ttl)
}

// SetMatch сохраняет объект матча в кэш
func (mc *MatchCache) SetMatch(ctx context.Context, match *domain.Match) error {
	data, err := json.Marshal(match)
	if err != nil {
		return fmt.Errorf("failed to marshal match: %w", err)
	}

	key := mc.getKey(match.ID)
	// Для активных матчей используем короткий TTL
	ttl := 5 * time.Minute
	if match.Status == domain.MatchCompleted {
		ttl = mc.ttl // 24 часа для завершённых
	}
	return mc.cache.Set(ctx, key, data, ttl)
}

// GetMatch получает объект матча из кэша
func (mc *MatchCache) GetMatch(ctx context.Context, matchID uuid.UUID) (*domain.Match, error) {
	key := mc.getKey(matchID)
	data, err := mc.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if data == "" {
		// Cache miss
		if mc.metrics != nil {
			mc.metrics.RecordCacheMiss("match")
		}
		return nil, nil
	}

	// Cache hit
	if mc.metrics != nil {
		mc.metrics.RecordCacheHit("match")
	}

	var match domain.Match
	if err := json.Unmarshal([]byte(data), &match); err != nil {
		return nil, fmt.Errorf("failed to unmarshal match: %w", err)
	}

	return &match, nil
}

// Get получает результат матча из кэша
func (mc *MatchCache) Get(ctx context.Context, matchID uuid.UUID) (*domain.MatchResult, error) {
	key := mc.getKey(matchID)
	data, err := mc.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if data == "" {
		// Cache miss
		if mc.metrics != nil {
			mc.metrics.RecordCacheMiss("match_result")
		}
		return nil, nil
	}

	// Cache hit
	if mc.metrics != nil {
		mc.metrics.RecordCacheHit("match_result")
	}

	var result domain.MatchResult
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal match result: %w", err)
	}

	return &result, nil
}

// Delete удаляет результат матча из кэша
func (mc *MatchCache) Delete(ctx context.Context, matchID uuid.UUID) error {
	key := mc.getKey(matchID)
	return mc.cache.Del(ctx, key)
}

// Exists проверяет существование результата в кэше
func (mc *MatchCache) Exists(ctx context.Context, matchID uuid.UUID) (bool, error) {
	key := mc.getKey(matchID)
	return mc.cache.Exists(ctx, key)
}
