package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/metrics"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const fullLeaderboardTTL = 10 * time.Second

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

// RatingUpdate - один рейтинг в пакетной операции.
type RatingUpdate struct {
	TournamentID uuid.UUID
	ProgramID    uuid.UUID
	Rating       int
}

// UpdateRatingsBatch обновляет рейтинги пакетом через Redis-пайплайн.
// Для пары участников одного матча экономит один RTT; при буферизации
// нескольких матчей экономия линейна по числу апдейтов.
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

// IncrementRating увеличивает рейтинг программы
func (lc *LeaderboardCache) IncrementRating(ctx context.Context, tournamentID, programID uuid.UUID, delta int) error {
	key := lc.getKey(tournamentID)
	return lc.cache.ZIncrBy(ctx, key, float64(delta), programID.String())
}

// GetTop получает топ N программ из leaderboard.
// Возвращает частичные данные: заполнены только Rank, ProgramID и Rating.
// Остальные поля LeaderboardEntry (ProgramName, TeamID, TeamName, Wins, Losses,
// Draws, TotalGames) имеют нулевые значения. Если нужны полные данные, вызывающий
// код должен запросить их напрямую из БД.
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

// Remove удаляет программу из leaderboard
func (lc *LeaderboardCache) Remove(ctx context.Context, tournamentID, programID uuid.UUID) error {
	key := lc.getKey(tournamentID)
	return lc.cache.ZRem(ctx, key, programID.String())
}

// Clear очищает весь leaderboard турнира (sorted set + all per-limit JSON caches + cross-game cache)
func (lc *LeaderboardCache) Clear(ctx context.Context, tournamentID uuid.UUID) error {
	key := lc.getKey(tournamentID)
	// Delete the sorted set
	if err := lc.cache.Del(ctx, key); err != nil {
		return err
	}
	// Delete all per-limit full JSON caches + cross-game cache via SCAN
	return lc.InvalidateFullLeaderboard(ctx, tournamentID)
}

// --- Full JSON leaderboard cache (short TTL, complete data) ---

func (lc *LeaderboardCache) getFullKey(tournamentID uuid.UUID) string {
	return fmt.Sprintf("leaderboard:full:%s", tournamentID.String())
}

func (lc *LeaderboardCache) getCrossGameKey(tournamentID uuid.UUID) string {
	return fmt.Sprintf("leaderboard:crossgame:%s", tournamentID.String())
}

// GetFullLeaderboard returns the cached full leaderboard JSON, or nil on miss.
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

// SetFullLeaderboard caches the complete leaderboard with short TTL.
func (lc *LeaderboardCache) SetFullLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int, entries []*domain.LeaderboardEntry) error {
	key := fmt.Sprintf("%s:%d", lc.getFullKey(tournamentID), limit)
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	return lc.cache.Set(ctx, key, string(data), fullLeaderboardTTL)
}

// GetFullCrossGameLeaderboard returns the cached cross-game leaderboard, or nil on miss.
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

// SetFullCrossGameLeaderboard caches the cross-game leaderboard with short TTL.
func (lc *LeaderboardCache) SetFullCrossGameLeaderboard(ctx context.Context, tournamentID uuid.UUID, entries []*domain.CrossGameLeaderboardEntry) error {
	key := lc.getCrossGameKey(tournamentID)
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	return lc.cache.Set(ctx, key, string(data), fullLeaderboardTTL)
}

// InvalidateFullLeaderboard deletes the full JSON caches for a tournament.
func (lc *LeaderboardCache) InvalidateFullLeaderboard(ctx context.Context, tournamentID uuid.UUID) error {
	// Use Scan to find all limit-specific keys
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
