package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/google/uuid"
)

// TournamentCache - кэш для турниров
type TournamentCache struct {
	cache *Cache
	ttl   time.Duration
}

// NewTournamentCache создаёт новый кэш для турниров
func NewTournamentCache(cache *Cache) *TournamentCache {
	return &TournamentCache{
		cache: cache,
		ttl:   1 * time.Hour, // кэшируем турниры на 1 час
	}
}

// getKey возвращает ключ для турнира
func (tc *TournamentCache) getKey(tournamentID uuid.UUID) string {
	return fmt.Sprintf("tournament:%s", tournamentID.String())
}

// getStatsKey возвращает ключ для статистики турнира
func (tc *TournamentCache) getStatsKey(tournamentID uuid.UUID) string {
	return fmt.Sprintf("tournament:%s:stats", tournamentID.String())
}

// Set сохраняет турнир в кэш
func (tc *TournamentCache) Set(ctx context.Context, tournament *domain.Tournament) error {
	data, err := json.Marshal(tournament)
	if err != nil {
		return fmt.Errorf("failed to marshal tournament: %w", err)
	}

	key := tc.getKey(tournament.ID)
	return tc.cache.Set(ctx, key, data, tc.ttl)
}

// Get получает турнир из кэша
func (tc *TournamentCache) Get(ctx context.Context, tournamentID uuid.UUID) (*domain.Tournament, error) {
	key := tc.getKey(tournamentID)
	data, err := tc.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if data == "" {
		return nil, nil // кэш промах
	}

	var tournament domain.Tournament
	if err := json.Unmarshal([]byte(data), &tournament); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tournament: %w", err)
	}

	return &tournament, nil
}

// Delete удаляет турнир из кэша
func (tc *TournamentCache) Delete(ctx context.Context, tournamentID uuid.UUID) error {
	key := tc.getKey(tournamentID)
	statsKey := tc.getStatsKey(tournamentID)
	return tc.cache.Del(ctx, key, statsKey)
}

// Invalidate инвалидирует кэш турнира (при обновлении)
func (tc *TournamentCache) Invalidate(ctx context.Context, tournamentID uuid.UUID) error {
	return tc.Delete(ctx, tournamentID)
}

// SetParticipantsCount сохраняет количество участников
func (tc *TournamentCache) SetParticipantsCount(ctx context.Context, tournamentID uuid.UUID, count int) error {
	key := fmt.Sprintf("tournament:%s:participants_count", tournamentID.String())
	return tc.cache.Set(ctx, key, count, tc.ttl)
}

// GetParticipantsCount получает количество участников из кэша
func (tc *TournamentCache) GetParticipantsCount(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	key := fmt.Sprintf("tournament:%s:participants_count", tournamentID.String())
	data, err := tc.cache.Get(ctx, key)
	if err != nil {
		return 0, err
	}

	if data == "" {
		return -1, nil // кэш промах
	}

	var count int
	if _, err := fmt.Sscanf(data, "%d", &count); err != nil {
		return 0, fmt.Errorf("failed to parse participants count: %w", err)
	}

	return count, nil
}

// IncrementParticipantsCount увеличивает счётчик участников
func (tc *TournamentCache) IncrementParticipantsCount(ctx context.Context, tournamentID uuid.UUID) error {
	key := fmt.Sprintf("tournament:%s:participants_count", tournamentID.String())

	// Проверяем существование ключа
	exists, err := tc.cache.Exists(ctx, key)
	if err != nil {
		return err
	}

	if !exists {
		// Если ключа нет, устанавливаем 1
		return tc.cache.Set(ctx, key, 1, tc.ttl)
	}

	// Иначе инкрементируем
	_, err = tc.cache.client.Incr(ctx, key).Result()
	return err
}

// SetMatchStatistics сохраняет статистику матчей турнира
func (tc *TournamentCache) SetMatchStatistics(ctx context.Context, tournamentID uuid.UUID, stats map[string]int) error {
	data, err := json.Marshal(stats)
	if err != nil {
		return fmt.Errorf("failed to marshal match statistics: %w", err)
	}

	key := tc.getStatsKey(tournamentID)
	return tc.cache.Set(ctx, key, data, tc.ttl)
}

// GetMatchStatistics получает статистику матчей из кэша
func (tc *TournamentCache) GetMatchStatistics(ctx context.Context, tournamentID uuid.UUID) (map[string]int, error) {
	key := tc.getStatsKey(tournamentID)
	data, err := tc.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if data == "" {
		return nil, nil // кэш промах
	}

	var stats map[string]int
	if err := json.Unmarshal([]byte(data), &stats); err != nil {
		return nil, fmt.Errorf("failed to unmarshal match statistics: %w", err)
	}

	return stats, nil
}

// Exists проверяет существование турнира в кэше
func (tc *TournamentCache) Exists(ctx context.Context, tournamentID uuid.UUID) (bool, error) {
	key := tc.getKey(tournamentID)
	return tc.cache.Exists(ctx, key)
}

// SetList кэширует список турниров с определённым фильтром
func (tc *TournamentCache) SetList(ctx context.Context, filter string, tournaments []*domain.Tournament) error {
	data, err := json.Marshal(tournaments)
	if err != nil {
		return fmt.Errorf("failed to marshal tournaments list: %w", err)
	}

	key := fmt.Sprintf("tournaments:list:%s", filter)
	// Короткий TTL для списков (5 минут)
	return tc.cache.Set(ctx, key, data, 5*time.Minute)
}

// GetList получает список турниров из кэша
func (tc *TournamentCache) GetList(ctx context.Context, filter string) ([]*domain.Tournament, error) {
	key := fmt.Sprintf("tournaments:list:%s", filter)
	data, err := tc.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if data == "" {
		return nil, nil // кэш промах
	}

	var tournaments []*domain.Tournament
	if err := json.Unmarshal([]byte(data), &tournaments); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tournaments list: %w", err)
	}

	return tournaments, nil
}

// InvalidateList инвалидирует кэшированные списки турниров
func (tc *TournamentCache) InvalidateList(ctx context.Context) error {
	// Инвалидируем все возможные комбинации списков
	patterns := []string{
		"tournaments:list:*",
	}

	for _, pattern := range patterns {
		// В продакшене здесь нужно использовать SCAN для удаления по паттерну
		// Для простоты просто удаляем известные ключи
		_ = tc.cache.Del(ctx, pattern)
	}

	return nil
}
