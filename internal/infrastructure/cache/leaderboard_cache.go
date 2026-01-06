package cache

import (
	"context"
	"fmt"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/metrics"
	"github.com/google/uuid"
)

// LeaderboardCache - кэш для таблицы лидеров
type LeaderboardCache struct {
	cache   *Cache
	metrics *metrics.Metrics
}

// NewLeaderboardCache создаёт новый кэш для leaderboard
func NewLeaderboardCache(cache *Cache) *LeaderboardCache {
	return &LeaderboardCache{
		cache:   cache,
		metrics: nil, // metrics опциональны
	}
}

// WithMetrics добавляет метрики в кэш
func (lc *LeaderboardCache) WithMetrics(m *metrics.Metrics) *LeaderboardCache {
	lc.metrics = m
	return lc
}

// getKey возвращает ключ для leaderboard турнира
func (lc *LeaderboardCache) getKey(tournamentID uuid.UUID) string {
	return fmt.Sprintf("leaderboard:%s", tournamentID.String())
}

// UpdateRating обновляет рейтинг программы в leaderboard
func (lc *LeaderboardCache) UpdateRating(ctx context.Context, tournamentID, programID uuid.UUID, rating int) error {
	key := lc.getKey(tournamentID)
	return lc.cache.ZAdd(ctx, key, float64(rating), programID.String())
}

// IncrementRating увеличивает рейтинг программы
func (lc *LeaderboardCache) IncrementRating(ctx context.Context, tournamentID, programID uuid.UUID, delta int) error {
	key := lc.getKey(tournamentID)
	return lc.cache.ZIncrBy(ctx, key, float64(delta), programID.String())
}

// GetTop получает топ N программ из leaderboard
func (lc *LeaderboardCache) GetTop(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*domain.LeaderboardEntry, error) {
	key := lc.getKey(tournamentID)
	results, err := lc.cache.ZRevRangeWithScores(ctx, key, 0, int64(limit-1))
	if err != nil {
		return nil, err
	}

	// Если пустой результат - cache miss
	if len(results) == 0 {
		if lc.metrics != nil {
			lc.metrics.RecordCacheMiss("leaderboard")
		}
		return nil, nil
	}

	// Cache hit
	if lc.metrics != nil {
		lc.metrics.RecordCacheHit("leaderboard")
	}

	entries := make([]*domain.LeaderboardEntry, 0, len(results))
	for i, result := range results {
		programID, err := uuid.Parse(result.Member.(string))
		if err != nil {
			continue
		}

		entries = append(entries, &domain.LeaderboardEntry{
			Rank:      i + 1,
			ProgramID: programID,
			Rating:    int(result.Score),
		})
	}

	return entries, nil
}

// Remove удаляет программу из leaderboard
func (lc *LeaderboardCache) Remove(ctx context.Context, tournamentID, programID uuid.UUID) error {
	key := lc.getKey(tournamentID)
	return lc.cache.ZRem(ctx, key, programID.String())
}

// Clear очищает весь leaderboard турнира
func (lc *LeaderboardCache) Clear(ctx context.Context, tournamentID uuid.UUID) error {
	key := lc.getKey(tournamentID)
	return lc.cache.Del(ctx, key)
}
