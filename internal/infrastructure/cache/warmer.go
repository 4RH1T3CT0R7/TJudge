package cache

import (
	"context"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TournamentRepository интерфейс для получения данных турниров
type TournamentRepository interface {
	List(ctx context.Context, filter domain.TournamentFilter) ([]*domain.Tournament, error)
	GetLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*domain.LeaderboardEntry, error)
}

// MatchRepository интерфейс для получения данных матчей
type MatchRepository interface {
	List(ctx context.Context, filter domain.MatchFilter) ([]*domain.Match, error)
}

// CacheWarmer - сервис для прогрева кэша
type CacheWarmer struct {
	cache            *Cache
	leaderboardCache *LeaderboardCache
	matchCache       *MatchCache
	tournamentCache  *TournamentCache
	tournamentRepo   TournamentRepository
	matchRepo        MatchRepository
	log              *logger.Logger
	warmupInterval   time.Duration
	stopChan         chan struct{}
}

// NewCacheWarmer создаёт новый warmer
func NewCacheWarmer(
	cache *Cache,
	leaderboardCache *LeaderboardCache,
	matchCache *MatchCache,
	tournamentCache *TournamentCache,
	tournamentRepo TournamentRepository,
	matchRepo MatchRepository,
	log *logger.Logger,
	warmupInterval time.Duration,
) *CacheWarmer {
	return &CacheWarmer{
		cache:            cache,
		leaderboardCache: leaderboardCache,
		matchCache:       matchCache,
		tournamentCache:  tournamentCache,
		tournamentRepo:   tournamentRepo,
		matchRepo:        matchRepo,
		log:              log,
		warmupInterval:   warmupInterval,
		stopChan:         make(chan struct{}),
	}
}

// Start запускает периодический прогрев кэша
func (cw *CacheWarmer) Start(ctx context.Context) {
	// Первый прогрев сразу при старте
	if err := cw.WarmUp(ctx); err != nil {
		cw.log.LogError("Initial cache warmup failed", err)
	}

	// Периодический прогрев
	ticker := time.NewTicker(cw.warmupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := cw.WarmUp(ctx); err != nil {
				cw.log.LogError("Scheduled cache warmup failed", err)
			}
		case <-cw.stopChan:
			cw.log.Info("Cache warmer stopped")
			return
		case <-ctx.Done():
			cw.log.Info("Cache warmer context cancelled")
			return
		}
	}
}

// Stop останавливает прогрев кэша
func (cw *CacheWarmer) Stop() {
	close(cw.stopChan)
}

// WarmUp выполняет полный прогрев кэша
func (cw *CacheWarmer) WarmUp(ctx context.Context) error {
	start := time.Now()
	cw.log.Info("Starting cache warmup")

	// Прогреваем активные турниры
	if err := cw.warmActiveTournaments(ctx); err != nil {
		cw.log.LogError("Failed to warm active tournaments", err)
	}

	// Прогреваем предстоящие турниры
	if err := cw.warmPendingTournaments(ctx); err != nil {
		cw.log.LogError("Failed to warm pending tournaments", err)
	}

	// Прогреваем leaderboards активных турниров
	if err := cw.warmLeaderboards(ctx); err != nil {
		cw.log.LogError("Failed to warm leaderboards", err)
	}

	// Прогреваем активные матчи
	if err := cw.warmActiveMatches(ctx); err != nil {
		cw.log.LogError("Failed to warm active matches", err)
	}

	duration := time.Since(start)
	cw.log.Info("Cache warmup completed",
		zap.Duration("duration", duration),
	)

	return nil
}

// warmActiveTournaments прогревает кэш активных турниров
func (cw *CacheWarmer) warmActiveTournaments(ctx context.Context) error {
	tournaments, err := cw.tournamentRepo.List(ctx, domain.TournamentFilter{
		Status: domain.TournamentActive,
		Limit:  100,
	})
	if err != nil {
		return err
	}

	for _, tournament := range tournaments {
		if err := cw.tournamentCache.Set(ctx, tournament); err != nil {
			cw.log.LogError("Failed to cache active tournament", err,
				zap.String("tournament_id", tournament.ID.String()),
			)
		}
	}

	cw.log.Info("Warmed up active tournaments",
		zap.Int("count", len(tournaments)),
	)

	return nil
}

// warmPendingTournaments прогревает кэш предстоящих турниров
func (cw *CacheWarmer) warmPendingTournaments(ctx context.Context) error {
	tournaments, err := cw.tournamentRepo.List(ctx, domain.TournamentFilter{
		Status: domain.TournamentPending,
		Limit:  50,
	})
	if err != nil {
		return err
	}

	for _, tournament := range tournaments {
		if err := cw.tournamentCache.Set(ctx, tournament); err != nil {
			cw.log.LogError("Failed to cache pending tournament", err,
				zap.String("tournament_id", tournament.ID.String()),
			)
		}
	}

	cw.log.Info("Warmed up pending tournaments",
		zap.Int("count", len(tournaments)),
	)

	return nil
}

// warmLeaderboards прогревает leaderboards для активных турниров
func (cw *CacheWarmer) warmLeaderboards(ctx context.Context) error {
	tournaments, err := cw.tournamentRepo.List(ctx, domain.TournamentFilter{
		Status: domain.TournamentActive,
		Limit:  100,
	})
	if err != nil {
		return err
	}

	totalEntries := 0
	for _, tournament := range tournaments {
		entries, err := cw.tournamentRepo.GetLeaderboard(ctx, tournament.ID, 100)
		if err != nil {
			cw.log.LogError("Failed to get leaderboard", err,
				zap.String("tournament_id", tournament.ID.String()),
			)
			continue
		}

		// Загружаем в Redis sorted set
		for _, entry := range entries {
			if err := cw.leaderboardCache.UpdateRating(ctx, tournament.ID, entry.ProgramID, entry.Rating); err != nil {
				cw.log.LogError("Failed to cache leaderboard entry", err)
			}
		}

		totalEntries += len(entries)
	}

	cw.log.Info("Warmed up leaderboards",
		zap.Int("tournaments", len(tournaments)),
		zap.Int("entries", totalEntries),
	)

	return nil
}

// warmActiveMatches прогревает кэш активных матчей
func (cw *CacheWarmer) warmActiveMatches(ctx context.Context) error {
	// Прогреваем running матчи
	runningMatches, err := cw.matchRepo.List(ctx, domain.MatchFilter{
		Status: domain.MatchRunning,
		Limit:  200,
	})
	if err != nil {
		return err
	}

	for _, match := range runningMatches {
		if err := cw.matchCache.SetMatch(ctx, match); err != nil {
			cw.log.LogError("Failed to cache running match", err,
				zap.String("match_id", match.ID.String()),
			)
		}
	}

	// Прогреваем pending матчи (следующие в очереди)
	pendingMatches, err := cw.matchRepo.List(ctx, domain.MatchFilter{
		Status: domain.MatchPending,
		Limit:  500, // Загружаем больше pending для очереди
	})
	if err != nil {
		return err
	}

	for _, match := range pendingMatches {
		if err := cw.matchCache.SetMatch(ctx, match); err != nil {
			cw.log.LogError("Failed to cache pending match", err,
				zap.String("match_id", match.ID.String()),
			)
		}
	}

	cw.log.Info("Warmed up active matches",
		zap.Int("running", len(runningMatches)),
		zap.Int("pending", len(pendingMatches)),
	)

	return nil
}
