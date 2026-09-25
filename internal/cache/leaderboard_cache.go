package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// полный лидерборд живёт всего 10 сек: он тяжёлый и его дёргают часто,
// короткий ttl режет нагрузку на бд, а данные при этом почти свежие
const fullLeaderboardTTL = 10 * time.Second

// кэш таблицы лидеров: готовый json ответа api с коротким ttl
type LeaderboardCache struct {
	cache   *Cache
	metrics *metrics.Metrics
}

func NewLeaderboardCache(cache *Cache) *LeaderboardCache {
	return &LeaderboardCache{
		cache:   cache,
		metrics: nil, // метрики опциональны
	}
}

func (lc *LeaderboardCache) WithMetrics(m *metrics.Metrics) *LeaderboardCache {
	lc.metrics = m
	if m != nil {
		m.PrimeCacheType("leaderboard_full", "leaderboard_crossgame")
	}
	return lc
}

func (lc *LeaderboardCache) getFullKey(tournamentID uuid.UUID) string {
	return fmt.Sprintf("leaderboard:full:%s", tournamentID.String())
}

func (lc *LeaderboardCache) getCrossGameKey(tournamentID uuid.UUID) string {
	return fmt.Sprintf("leaderboard:crossgame:%s", tournamentID.String())
}

func (lc *LeaderboardCache) GetFullLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*models.LeaderboardEntry, error) {
	key := fmt.Sprintf("%s:%d", lc.getFullKey(tournamentID), limit)
	data, err := lc.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if data == "" {
		if lc.metrics != nil {
			lc.metrics.RecordCacheMiss("leaderboard_full")
		}
		return nil, nil
	}
	var entries []*models.LeaderboardEntry
	if err := json.Unmarshal([]byte(data), &entries); err != nil {
		// битый json проще удалить и посчитать заново
		lc.cache.log.Warn("leaderboard cache: corrupt full leaderboard JSON, deleting key",
			zap.String("key", key), zap.String("tournament_id", tournamentID.String()), zap.Error(err))
		_ = lc.cache.Del(ctx, key)
		return nil, nil
	}
	if lc.metrics != nil {
		lc.metrics.RecordCacheHit("leaderboard_full")
	}
	return entries, nil
}

func (lc *LeaderboardCache) SetFullLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int, entries []*models.LeaderboardEntry) error {
	key := fmt.Sprintf("%s:%d", lc.getFullKey(tournamentID), limit)
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	return lc.cache.Set(ctx, key, string(data), fullLeaderboardTTL)
}

func (lc *LeaderboardCache) GetFullCrossGameLeaderboard(ctx context.Context, tournamentID uuid.UUID) ([]*models.CrossGameLeaderboardEntry, error) {
	key := lc.getCrossGameKey(tournamentID)
	data, err := lc.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if data == "" {
		if lc.metrics != nil {
			lc.metrics.RecordCacheMiss("leaderboard_crossgame")
		}
		return nil, nil
	}
	var entries []*models.CrossGameLeaderboardEntry
	if err := json.Unmarshal([]byte(data), &entries); err != nil {
		lc.cache.log.Warn("leaderboard cache: corrupt cross-game leaderboard JSON, deleting key",
			zap.String("key", key), zap.String("tournament_id", tournamentID.String()), zap.Error(err))
		_ = lc.cache.Del(ctx, key)
		return nil, nil
	}
	if lc.metrics != nil {
		lc.metrics.RecordCacheHit("leaderboard_crossgame")
	}
	return entries, nil
}

func (lc *LeaderboardCache) SetFullCrossGameLeaderboard(ctx context.Context, tournamentID uuid.UUID, entries []*models.CrossGameLeaderboardEntry) error {
	key := lc.getCrossGameKey(tournamentID)
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	return lc.cache.Set(ctx, key, string(data), fullLeaderboardTTL)
}

// InvalidateFullLeaderboard выносит json-кэши турнира.
// ключей несколько (по каждому лимиту свой), поэтому чистка идёт сканом,
// в конце добавляется кросс-гейм ключ и удаляется всё пачкой
func (lc *LeaderboardCache) InvalidateFullLeaderboard(ctx context.Context, tournamentID uuid.UUID) error {
	pattern := fmt.Sprintf("leaderboard:full:%s:*", tournamentID.String())
	crossKey := lc.getCrossGameKey(tournamentID)

	var allKeys []string
	var cursor uint64
	for {
		keys, nextCursor, err := lc.cache.Scan(ctx, cursor, pattern, 100)
		if err != nil {
			return err
		}
		allKeys = append(allKeys, keys...)
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	allKeys = append(allKeys, crossKey)

	if len(allKeys) > 0 {
		return lc.cache.Del(ctx, allKeys...)
	}
	return nil
}
