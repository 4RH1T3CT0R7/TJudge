package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
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

// полные лидерборды турнира лежат в одном hash (поле = limit): так
// инвалидация - один DEL известных ключей, без SCAN по всему keyspace
func (lc *LeaderboardCache) GetFullLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*models.LeaderboardEntry, error) {
	key := lc.getFullKey(tournamentID)
	field := strconv.Itoa(limit)
	data, err := lc.cache.client.HGet(ctx, key, field).Result()
	if errors.Is(err, redis.Nil) {
		if lc.metrics != nil {
			lc.metrics.RecordCacheMiss("leaderboard_full")
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []*models.LeaderboardEntry
	if err := json.Unmarshal([]byte(data), &entries); err != nil {
		// битый json проще удалить и посчитать заново
		lc.cache.log.Warn("leaderboard cache: corrupt full leaderboard JSON, deleting field",
			zap.String("key", key), zap.String("field", field), zap.Error(err))
		_ = lc.cache.client.HDel(ctx, key, field).Err()
		return nil, nil
	}
	if lc.metrics != nil {
		lc.metrics.RecordCacheHit("leaderboard_full")
	}
	return entries, nil
}

func (lc *LeaderboardCache) SetFullLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int, entries []*models.LeaderboardEntry) error {
	key := lc.getFullKey(tournamentID)
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	// ttl ставится только новому hash (NX): иначе каждая запись с другим
	// limit продлевала бы жизнь старым полям. MULTI - чтобы hash не остался без ttl
	pipe := lc.cache.client.TxPipeline()
	pipe.HSet(ctx, key, strconv.Itoa(limit), data)
	pipe.ExpireNX(ctx, key, fullLeaderboardTTL)
	_, err = pipe.Exec(ctx)
	return err
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

// InvalidateFullLeaderboard выносит json-кэши турнира: hash по всем
// лимитам и кросс-гейм
func (lc *LeaderboardCache) InvalidateFullLeaderboard(ctx context.Context, tournamentID uuid.UUID) error {
	return lc.cache.Del(ctx, lc.getFullKey(tournamentID), lc.getCrossGameKey(tournamentID))
}
