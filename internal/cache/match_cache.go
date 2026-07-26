package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/google/uuid"
)

// кэш результатов матчей в редисе
type MatchCache struct {
	cache   *Cache
	ttl     time.Duration
	metrics *metrics.Metrics
}

func NewMatchCache(cache *Cache) *MatchCache {
	return &MatchCache{
		cache:   cache,
		ttl:     24 * time.Hour, // результаты держим сутки
		metrics: nil,            // метрики опциональны
	}
}

func (mc *MatchCache) WithMetrics(m *metrics.Metrics) *MatchCache {
	mc.metrics = m
	if m != nil {
		m.PrimeCacheType("match", "match_result")
	}
	return mc
}

func (mc *MatchCache) getKey(matchID uuid.UUID) string {
	return fmt.Sprintf("match:%s", matchID.String())
}

func (mc *MatchCache) Set(ctx context.Context, matchID uuid.UUID, result *domain.MatchResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal match result: %w", err)
	}

	key := mc.getKey(matchID)
	return mc.cache.Set(ctx, key, data, mc.ttl)
}

// SetMatch кладёт сам матч, а не его результат
func (mc *MatchCache) SetMatch(ctx context.Context, match *domain.Match) error {
	data, err := json.Marshal(match)
	if err != nil {
		return fmt.Errorf("failed to marshal match: %w", err)
	}

	key := mc.getKey(match.ID)
	// активный матч ещё поменяется, поэтому держим недолго
	ttl := 5 * time.Minute
	if match.Status == domain.MatchCompleted {
		ttl = mc.ttl // завершённый уже не изменится, храним сутки
	}
	return mc.cache.Set(ctx, key, data, ttl)
}

func (mc *MatchCache) GetMatch(ctx context.Context, matchID uuid.UUID) (*domain.Match, error) {
	key := mc.getKey(matchID)
	data, err := mc.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if data == "" {
		// промах
		if mc.metrics != nil {
			mc.metrics.RecordCacheMiss("match")
		}
		return nil, nil
	}

	if mc.metrics != nil {
		mc.metrics.RecordCacheHit("match")
	}

	var match domain.Match
	if err := json.Unmarshal([]byte(data), &match); err != nil {
		return nil, fmt.Errorf("failed to unmarshal match: %w", err)
	}

	return &match, nil
}

func (mc *MatchCache) Get(ctx context.Context, matchID uuid.UUID) (*domain.MatchResult, error) {
	key := mc.getKey(matchID)
	data, err := mc.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if data == "" {
		if mc.metrics != nil {
			mc.metrics.RecordCacheMiss("match_result")
		}
		return nil, nil
	}

	if mc.metrics != nil {
		mc.metrics.RecordCacheHit("match_result")
	}

	var result domain.MatchResult
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal match result: %w", err)
	}

	return &result, nil
}

func (mc *MatchCache) Delete(ctx context.Context, matchID uuid.UUID) error {
	key := mc.getKey(matchID)
	return mc.cache.Del(ctx, key)
}

func (mc *MatchCache) Exists(ctx context.Context, matchID uuid.UUID) (bool, error) {
	key := mc.getKey(matchID)
	return mc.cache.Exists(ctx, key)
}
