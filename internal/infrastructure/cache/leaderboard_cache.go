package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// полный лидерборд живёт всего 10 сек: он тяжёлый и его дёргают часто,
// короткий ttl режет нагрузку на бд, а данные при этом почти свежие
const fullLeaderboardTTL = 10 * time.Second

// кэш таблицы лидеров (sorted set + json на полные данные)
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
		m.PrimeCacheType("leaderboard", "leaderboard_full", "leaderboard_crossgame")
	}
	return lc
}

func (lc *LeaderboardCache) getKey(tournamentID uuid.UUID) string {
	return fmt.Sprintf("leaderboard:%s", tournamentID.String())
}

func (lc *LeaderboardCache) UpdateRating(ctx context.Context, tournamentID, programID uuid.UUID, rating int) error {
	key := lc.getKey(tournamentID)
	return lc.cache.ZAdd(ctx, key, float64(rating), programID.String())
}

// один рейтинг в пакетной операции
type RatingUpdate struct {
	TournamentID uuid.UUID
	ProgramID    uuid.UUID
	Rating       int
}

// UpdateRatingsBatch пишет рейтинги одним пайплайном.
// на паре участников матча экономит один RTT, а если буферизуем
// несколько матчей — экономия растёт линейно по числу апдейтов
func (lc *LeaderboardCache) UpdateRatingsBatch(ctx context.Context, updates []RatingUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	members := make([]ZAddBatchMember, 0, len(updates))
	for _, u := range updates {
		members = append(members, ZAddBatchMember{
			Key:    lc.getKey(u.TournamentID),
			Score:  float64(u.Rating),
			Member: u.ProgramID.String(),
		})
	}
	return lc.cache.BatchZAdd(ctx, members)
}

func (lc *LeaderboardCache) IncrementRating(ctx context.Context, tournamentID, programID uuid.UUID, delta int) error {
	key := lc.getKey(tournamentID)
	return lc.cache.ZIncrBy(ctx, key, float64(delta), programID.String())
}

// GetTop отдаёт топ N из sorted set.
// данные неполные: заполнены только Rank, ProgramID и Rating,
// остальное (имя, команда, w/l/d) нулевое — кому надо, дотянет из бд
func (lc *LeaderboardCache) GetTop(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*domain.LeaderboardEntry, error) {
	key := lc.getKey(tournamentID)
	results, err := lc.cache.ZRevRangeWithScores(ctx, key, 0, int64(limit-1))
	if err != nil {
		return nil, err
	}

	// пусто = промах
	if len(results) == 0 {
		if lc.metrics != nil {
			lc.metrics.RecordCacheMiss("leaderboard")
		}
		return nil, nil
	}

	if lc.metrics != nil {
		lc.metrics.RecordCacheHit("leaderboard")
	}

	entries := make([]*domain.LeaderboardEntry, 0, len(results))
	for i, result := range results {
		memberStr, ok := result.Member.(string)
		if !ok {
			lc.cache.log.Warn("leaderboard cache: non-string member in sorted set")
			continue
		}
		programID, err := uuid.Parse(memberStr)
		if err != nil {
			lc.cache.log.Warn("leaderboard cache: corrupt program ID in sorted set",
				zap.String("member", memberStr), zap.Error(err))
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

func (lc *LeaderboardCache) Remove(ctx context.Context, tournamentID, programID uuid.UUID) error {
	key := lc.getKey(tournamentID)
	return lc.cache.ZRem(ctx, key, programID.String())
}

// Clear сносит весь лидерборд турнира: sorted set + все json по лимитам + кросс-гейм
func (lc *LeaderboardCache) Clear(ctx context.Context, tournamentID uuid.UUID) error {
	key := lc.getKey(tournamentID)
	if err := lc.cache.Del(ctx, key); err != nil {
		return err
	}
	// json-кэши и кросс-гейм добиваем сканом
	return lc.InvalidateFullLeaderboard(ctx, tournamentID)
}

// --- полный json-лидерборд (короткий ttl, все поля на месте) ---

func (lc *LeaderboardCache) getFullKey(tournamentID uuid.UUID) string {
	return fmt.Sprintf("leaderboard:full:%s", tournamentID.String())
}

func (lc *LeaderboardCache) getCrossGameKey(tournamentID uuid.UUID) string {
	return fmt.Sprintf("leaderboard:crossgame:%s", tournamentID.String())
}

func (lc *LeaderboardCache) GetFullLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*domain.LeaderboardEntry, error) {
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
	var entries []*domain.LeaderboardEntry
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

func (lc *LeaderboardCache) SetFullLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int, entries []*domain.LeaderboardEntry) error {
	key := fmt.Sprintf("%s:%d", lc.getFullKey(tournamentID), limit)
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	return lc.cache.Set(ctx, key, string(data), fullLeaderboardTTL)
}

func (lc *LeaderboardCache) GetFullCrossGameLeaderboard(ctx context.Context, tournamentID uuid.UUID) ([]*domain.CrossGameLeaderboardEntry, error) {
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
	var entries []*domain.CrossGameLeaderboardEntry
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

func (lc *LeaderboardCache) SetFullCrossGameLeaderboard(ctx context.Context, tournamentID uuid.UUID, entries []*domain.CrossGameLeaderboardEntry) error {
	key := lc.getCrossGameKey(tournamentID)
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	return lc.cache.Set(ctx, key, string(data), fullLeaderboardTTL)
}

// InvalidateFullLeaderboard выносит json-кэши турнира.
// ключей несколько (по каждому лимиту свой), поэтому идём сканом,
// в конце добавляем кросс-гейм ключ и удаляем всё пачкой
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
