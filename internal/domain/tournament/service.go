package tournament

//go:generate mockgen -destination=../../mocks/mock_tournament.go -package=mocks github.com/bmstu-itstech/tjudge/internal/domain/tournament TournamentCacher,LeaderboardCacher,TournamentRepository,MatchRepository,QueueManager

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

// TournamentCacher интерфейс для кэширования турниров
type TournamentCacher interface {
	Set(ctx context.Context, tournament *domain.Tournament) error
	Get(ctx context.Context, tournamentID uuid.UUID) (*domain.Tournament, error)
	Invalidate(ctx context.Context, tournamentID uuid.UUID) error
}

// LeaderboardCacher — full interface for leaderboard cache used by the tournament service.
// Used for cache-aside reads in GetLeaderboard/GetCrossGameLeaderboard.
type LeaderboardCacher interface {
	GetTop(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*domain.LeaderboardEntry, error)
	UpdateRating(ctx context.Context, tournamentID, programID uuid.UUID, rating int) error
	Clear(ctx context.Context, tournamentID uuid.UUID) error

	// Full JSON leaderboard cache (short TTL, complete data for API responses)
	GetFullLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*domain.LeaderboardEntry, error)
	SetFullLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int, entries []*domain.LeaderboardEntry) error
	GetFullCrossGameLeaderboard(ctx context.Context, tournamentID uuid.UUID) ([]*domain.CrossGameLeaderboardEntry, error)
	SetFullCrossGameLeaderboard(ctx context.Context, tournamentID uuid.UUID, entries []*domain.CrossGameLeaderboardEntry) error
	InvalidateFullLeaderboard(ctx context.Context, tournamentID uuid.UUID) error
}

// TournamentRepository интерфейс для работы с турнирами
type TournamentRepository interface {
	Create(ctx context.Context, tournament *domain.Tournament) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error)
	List(ctx context.Context, filter domain.TournamentFilter) ([]*domain.Tournament, error)
	Update(ctx context.Context, tournament *domain.Tournament) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.TournamentStatus) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetParticipantsCount(ctx context.Context, tournamentID uuid.UUID) (int, error)
	GetTeamsCount(ctx context.Context, tournamentID uuid.UUID) (int, error)
	GetParticipants(ctx context.Context, tournamentID uuid.UUID) ([]*domain.TournamentParticipant, error)
	GetLatestParticipants(ctx context.Context, tournamentID uuid.UUID) ([]*domain.TournamentParticipant, error)
	GetLatestParticipantsGroupedByGame(ctx context.Context, tournamentID uuid.UUID) (map[string][]*domain.TournamentParticipant, error)
	GetLatestParticipantsByGame(ctx context.Context, tournamentID uuid.UUID, gameType string) ([]*domain.TournamentParticipant, error)
	AddParticipant(ctx context.Context, participant *domain.TournamentParticipant) error
	GetLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*domain.LeaderboardEntry, error)
	GetCrossGameLeaderboard(ctx context.Context, tournamentID uuid.UUID) ([]*domain.CrossGameLeaderboardEntry, error)
}

// MatchRepository интерфейс для работы с матчами
type MatchRepository interface {
	Create(ctx context.Context, match *domain.Match) error
	CreateBatch(ctx context.Context, matches []*domain.Match) error
	GetByTournamentID(ctx context.Context, tournamentID uuid.UUID, limit, offset int) ([]*domain.Match, error)
	GetPendingByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]*domain.Match, error)
	GetPendingByTournamentAndGame(ctx context.Context, tournamentID uuid.UUID, gameType string) ([]*domain.Match, error)
	ResetFailedMatches(ctx context.Context, tournamentID uuid.UUID) (int64, error)
	GetNextRoundNumber(ctx context.Context, tournamentID uuid.UUID) (int, error)
	GetNextRoundNumberByGame(ctx context.Context, tournamentID uuid.UUID, gameType string) (int, error)
	GetMatchesByRounds(ctx context.Context, tournamentID uuid.UUID) ([]*domain.MatchRound, error)
	GetPlayedProgramPairs(ctx context.Context, tournamentID uuid.UUID, gameType string) (map[string]struct{}, error)
}

// QueueManager интерфейс для работы с очередями
type QueueManager interface {
	Enqueue(ctx context.Context, match *domain.Match) error
	EnqueueBatch(ctx context.Context, matches []*domain.Match) error
}

// DistributedLock интерфейс для распределённых блокировок
type DistributedLock interface {
	WithLock(ctx context.Context, key string, ttl time.Duration, fn func(ctx context.Context) error) error
}

// GameRepository интерфейс для работы с играми в турнире
type GameRepository interface {
	GetTournamentGames(ctx context.Context, tournamentID uuid.UUID) ([]*domain.TournamentGame, error)
	SetActiveGame(ctx context.Context, tournamentID, gameID uuid.UUID) error
	ResetGameByType(ctx context.Context, tournamentID uuid.UUID, gameType string) error
	// Auto-round
	GetAutoRoundEnabledGames(ctx context.Context) ([]*domain.AutoRoundGameInfo, error)
	UpdateAutoRoundLastRun(ctx context.Context, tournamentID, gameID uuid.UUID) error
	HasNewProgramsSince(ctx context.Context, tournamentID uuid.UUID, gameType string, since time.Time) (bool, error)
	HasActiveMatchesForGame(ctx context.Context, tournamentID uuid.UUID, gameType string) (bool, error)
}

// Service - сервис управления турнирами
type Service struct {
	tournamentRepo   TournamentRepository
	matchRepo        MatchRepository
	queueManager     QueueManager
	gameRepo         GameRepository
	tournamentCache  TournamentCacher
	leaderboardCache LeaderboardCacher
	eventBus         events.Bus
	distributedLock  DistributedLock
	log              *logger.Logger
	leaderboardSF    singleflight.Group
}

// NewService создаёт новый сервис турниров
func NewService(
	tournamentRepo TournamentRepository,
	matchRepo MatchRepository,
	queueManager QueueManager,
	gameRepo GameRepository,
	tournamentCache TournamentCacher,
	leaderboardCache LeaderboardCacher,
	eventBus events.Bus,
	distributedLock DistributedLock,
	log *logger.Logger,
) *Service {
	return &Service{
		tournamentRepo:   tournamentRepo,
		matchRepo:        matchRepo,
		queueManager:     queueManager,
		gameRepo:         gameRepo,
		tournamentCache:  tournamentCache,
		leaderboardCache: leaderboardCache,
		eventBus:         eventBus,
		distributedLock:  distributedLock,
		log:              log,
	}
}

// CreateRequest - запрос на создание турнира
type CreateRequest struct {
	Name            string                 `json:"name"`
	Description     string                 `json:"description,omitempty"`
	GameType        string                 `json:"game_type"`
	MaxParticipants *int                   `json:"max_participants,omitempty"`
	MaxTeamSize     int                    `json:"max_team_size,omitempty"`
	IsPermanent     bool                   `json:"is_permanent,omitempty"`
	StartTime       *time.Time             `json:"start_time,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	CreatorID       *uuid.UUID             `json:"-"` // Устанавливается из контекста, не из JSON
}

// generateCode генерирует уникальный код турнира (6 символов)
// Использует crypto/rand для равномерного распределения символов
func generateCode() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // без похожих символов I,O,0,1
	code := make([]byte, 6)
	max := big.NewInt(int64(len(charset)))
	for i := range code {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			// crypto/rand failure indicates broken OS entropy — panic rather than silently degrade
			panic("crypto/rand.Int failed: " + err.Error())
		}
		code[i] = charset[n.Int64()]
	}
	return string(code)
}

// Create создаёт новый турнир
func (s *Service) Create(ctx context.Context, req *CreateRequest) (*domain.Tournament, error) {
	// Устанавливаем значения по умолчанию
	maxTeamSize := req.MaxTeamSize
	if maxTeamSize <= 0 {
		maxTeamSize = 1
	}

	tournament := &domain.Tournament{
		ID:              uuid.New(),
		Code:            generateCode(),
		Name:            req.Name,
		Description:     req.Description,
		GameType:        req.GameType,
		Status:          domain.TournamentPending,
		MaxParticipants: req.MaxParticipants,
		MaxTeamSize:     maxTeamSize,
		IsPermanent:     req.IsPermanent,
		StartTime:       req.StartTime,
		Metadata:        req.Metadata,
		CreatorID:       req.CreatorID,
	}

	// Валидация
	if err := tournament.Validate(); err != nil {
		return nil, errors.ErrValidation.WithError(err)
	}

	// Сохраняем в БД
	if err := s.tournamentRepo.Create(ctx, tournament); err != nil {
		return nil, fmt.Errorf("failed to create tournament: %w", err)
	}

	s.log.Info("Tournament created",
		zap.String("tournament_id", tournament.ID.String()),
		zap.String("name", tournament.Name),
		zap.String("game_type", tournament.GameType),
	)

	// Publish event (cache side-effects handled by event handlers)
	s.eventBus.Publish(ctx, events.TournamentCreated{Version: 1, Tournament: tournament})

	return tournament, nil
}

// GetByID получает турнир по ID
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error) {
	// Проверяем кэш
	cached, err := s.tournamentCache.Get(ctx, id)
	if err == nil && cached != nil {
		return cached, nil
	}

	// Получаем из БД
	tournament, err := s.tournamentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Кэшируем
	if err := s.tournamentCache.Set(ctx, tournament); err != nil {
		s.log.Error("Failed to cache tournament", zap.Error(err))
	}

	return tournament, nil
}

// List получает список турниров с фильтрацией
func (s *Service) List(ctx context.Context, filter domain.TournamentFilter) ([]*domain.Tournament, error) {
	// Устанавливаем лимит по умолчанию
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	// Получаем из БД
	tournaments, err := s.tournamentRepo.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	return tournaments, nil
}

// JoinRequest - запрос на участие в турнире
type JoinRequest struct {
	TournamentID uuid.UUID `json:"tournament_id"`
	ProgramID    uuid.UUID `json:"program_id"`
}

// Join добавляет участника в турнир
func (s *Service) Join(ctx context.Context, req *JoinRequest) error {
	// Используем distributed lock для предотвращения race condition
	// при проверке лимита участников
	lockKey := fmt.Sprintf("tournament:join:%s", req.TournamentID.String())

	return s.distributedLock.WithLock(ctx, lockKey, 5*time.Second, func(ctx context.Context) error {
		// Получаем турнир
		tournament, err := s.GetByID(ctx, req.TournamentID)
		if err != nil {
			return err
		}

		// Проверяем статус турнира
		if tournament.Status != domain.TournamentPending {
			return errors.ErrTournamentStarted
		}

		// Проверяем лимит участников
		if tournament.MaxParticipants != nil {
			count, err := s.tournamentRepo.GetParticipantsCount(ctx, req.TournamentID)
			if err != nil {
				return fmt.Errorf("failed to get participants count: %w", err)
			}

			if count >= *tournament.MaxParticipants {
				return errors.ErrTournamentFull
			}
		}

		// Добавляем участника
		participant := &domain.TournamentParticipant{
			ID:           uuid.New(),
			TournamentID: req.TournamentID,
			ProgramID:    req.ProgramID,
			Rating:       1500, // Начальный рейтинг ELO
		}

		if err := s.tournamentRepo.AddParticipant(ctx, participant); err != nil {
			return fmt.Errorf("failed to add participant: %w", err)
		}

		s.log.Info("Participant joined tournament",
			zap.String("tournament_id", req.TournamentID.String()),
			zap.String("program_id", req.ProgramID.String()),
		)

		// Publish event (cache invalidation + leaderboard update handled by event handlers)
		s.eventBus.Publish(ctx, events.ParticipantJoined{
			Version:       1,
			TournamentID:  req.TournamentID,
			ProgramID:     req.ProgramID,
			InitialRating: 1500,
		})

		return nil
	})
}

// Start запускает турнир (меняет статус на active и активирует первую игру)
// Матчи НЕ генерируются автоматически - запускаются вручную администратором
func (s *Service) Start(ctx context.Context, tournamentID uuid.UUID) error {
	// Используем distributed lock для предотвращения одновременного старта
	lockKey := fmt.Sprintf("tournament:start:%s", tournamentID.String())

	lockErr := s.distributedLock.WithLock(ctx, lockKey, 60*time.Second, func(ctx context.Context) error {
		// Получаем турнир напрямую из БД (минуя кэш) для избежания проблем с версией
		// при оптимистичной блокировке
		tournament, err := s.tournamentRepo.GetByID(ctx, tournamentID)
		if err != nil {
			return err
		}

		// Проверяем статус
		if tournament.Status != domain.TournamentPending {
			return errors.ErrConflict.WithMessage("tournament already started or completed")
		}

		// Проверяем что есть минимум 2 команды
		teamsCount, err := s.tournamentRepo.GetTeamsCount(ctx, tournamentID)
		if err != nil {
			return fmt.Errorf("failed to get teams count: %w", err)
		}
		if teamsCount < 2 {
			return errors.ErrValidation.WithMessage("для старта турнира нужно минимум 2 команды")
		}

		// Обновляем статус турнира
		now := time.Now()
		tournament.Status = domain.TournamentActive
		tournament.StartTime = &now

		if err := s.tournamentRepo.Update(ctx, tournament); err != nil {
			s.log.Error("Failed to update tournament", zap.Error(err))
			return errors.ErrInternal.WithMessage("failed to update tournament status")
		}

		s.log.Info("Tournament started",
			zap.String("tournament_id", tournamentID.String()),
		)

		// Активируем первую игру (если есть)
		if s.gameRepo != nil {
			games, err := s.gameRepo.GetTournamentGames(ctx, tournamentID)
			if err != nil {
				s.log.Warn("Failed to get tournament games", zap.Error(err))
			} else if len(games) > 0 {
				// Активируем первую игру
				if err := s.gameRepo.SetActiveGame(ctx, tournamentID, games[0].GameID); err != nil {
					s.log.Warn("Failed to set first game as active", zap.Error(err))
				} else {
					s.log.Info("First game set as active",
						zap.String("tournament_id", tournamentID.String()),
						zap.String("game_id", games[0].GameID.String()),
					)
				}
			}
		}

		// Publish event (cache invalidation + broadcast handled by event handlers)
		s.eventBus.Publish(ctx, events.TournamentStarted{
			Version:      1,
			TournamentID: tournamentID,
			Status:       tournament.Status,
			StartTime:    tournament.StartTime,
		})

		return nil
	})

	// Обрабатываем ошибку блокировки
	if lockErr != nil {
		if errors.IsAppError(lockErr) {
			return lockErr
		}
		s.log.Error("Lock error during tournament start", zap.Error(lockErr))
		return errors.ErrConflict.WithMessage("could not start tournament, try again later")
	}
	return nil
}

// Complete завершает турнир
func (s *Service) Complete(ctx context.Context, tournamentID uuid.UUID) error {
	// Используем distributed lock для предотвращения одновременного завершения
	lockKey := fmt.Sprintf("tournament:complete:%s", tournamentID.String())

	lockErr := s.distributedLock.WithLock(ctx, lockKey, 60*time.Second, func(ctx context.Context) error {
		// Получаем турнир напрямую из БД (минуя кэш) для избежания stale данных
		tournament, err := s.tournamentRepo.GetByID(ctx, tournamentID)
		if err != nil {
			return err
		}

		if tournament.Status != domain.TournamentActive {
			return errors.ErrConflict.WithMessage("tournament is not active")
		}

		now := time.Now()
		tournament.Status = domain.TournamentCompleted
		tournament.EndTime = &now

		if err := s.tournamentRepo.Update(ctx, tournament); err != nil {
			return fmt.Errorf("failed to complete tournament: %w", err)
		}

		s.log.Info("Tournament completed",
			zap.String("tournament_id", tournamentID.String()),
		)

		// Publish event (cache invalidation + broadcast handled by event handlers)
		s.eventBus.Publish(ctx, events.TournamentCompleted{
			Version:      1,
			TournamentID: tournamentID,
			Status:       tournament.Status,
			EndTime:      tournament.EndTime,
		})

		return nil
	})

	// Обрабатываем ошибку блокировки
	if lockErr != nil {
		if errors.IsAppError(lockErr) {
			return lockErr
		}
		s.log.Error("Lock error during tournament completion", zap.Error(lockErr))
		return errors.ErrConflict.WithMessage("could not complete tournament, try again later")
	}
	return nil
}

// Delete удаляет турнир
func (s *Service) Delete(ctx context.Context, tournamentID uuid.UUID) error {
	// Получаем турнир для проверки
	tournament, err := s.GetByID(ctx, tournamentID)
	if err != nil {
		return err
	}

	// Нельзя удалить активный турнир
	if tournament.Status == domain.TournamentActive {
		return errors.ErrConflict.WithMessage("cannot delete active tournament")
	}

	// Удаляем из БД
	if err := s.tournamentRepo.Delete(ctx, tournamentID); err != nil {
		return fmt.Errorf("failed to delete tournament: %w", err)
	}

	s.log.Info("Tournament deleted",
		zap.String("tournament_id", tournamentID.String()),
	)

	// Publish event (cache cleanup handled by event handlers)
	s.eventBus.Publish(ctx, events.TournamentDeleted{Version: 1, TournamentID: tournamentID})

	return nil
}

// GetLeaderboard получает таблицу лидеров турнира
func (s *Service) GetLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*domain.LeaderboardEntry, error) {
	// Try full JSON cache first (short TTL, complete data)
	cached, err := s.leaderboardCache.GetFullLeaderboard(ctx, tournamentID, limit)
	if err != nil {
		s.log.Error("Failed to get full leaderboard cache", zap.Error(err))
	}
	if cached != nil {
		return cached, nil
	}

	// Cache miss — use singleflight to prevent thundering herd
	sfKey := fmt.Sprintf("leaderboard:%s:%d", tournamentID, limit)
	val, err, _ := s.leaderboardSF.Do(sfKey, func() (interface{}, error) {
		leaderboard, err := s.tournamentRepo.GetLeaderboard(ctx, tournamentID, limit)
		if err != nil {
			return nil, err
		}

		// Populate full JSON cache
		if err := s.leaderboardCache.SetFullLeaderboard(ctx, tournamentID, limit, leaderboard); err != nil {
			s.log.Error("Failed to set full leaderboard cache", zap.Error(err))
		}

		// Update sorted set cache for rating lookups
		for _, entry := range leaderboard {
			if err := s.leaderboardCache.UpdateRating(ctx, tournamentID, entry.ProgramID, entry.Rating); err != nil {
				s.log.Error("Failed to update leaderboard cache", zap.Error(err))
			}
		}

		return leaderboard, nil
	})
	if err != nil {
		return nil, err
	}
	entries, _ := val.([]*domain.LeaderboardEntry)
	return entries, nil
}

// CreateMatch создаёт матч и добавляет в очередь
func (s *Service) CreateMatch(ctx context.Context, tournamentID, program1ID, program2ID uuid.UUID, priority domain.MatchPriority) (*domain.Match, error) {
	// Получаем турнир для game_type
	tournament, err := s.GetByID(ctx, tournamentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tournament: %w", err)
	}

	match := &domain.Match{
		ID:           uuid.New(),
		TournamentID: tournamentID,
		Program1ID:   program1ID,
		Program2ID:   program2ID,
		GameType:     tournament.GameType,
		Status:       domain.MatchPending,
		Priority:     priority,
		CreatedAt:    time.Now(),
	}

	// Валидация
	if err := match.Validate(); err != nil {
		return nil, errors.ErrValidation.WithError(err)
	}

	// Сохраняем в БД
	if err := s.matchRepo.Create(ctx, match); err != nil {
		return nil, fmt.Errorf("failed to create match: %w", err)
	}

	// Добавляем в очередь
	if err := s.queueManager.Enqueue(ctx, match); err != nil {
		s.log.Error("Failed to enqueue match",
			zap.Error(err),
			zap.String("match_id", match.ID.String()),
		)
		// Не возвращаем ошибку, матч всё равно создан
	}

	s.log.Info("Match created",
		zap.String("match_id", match.ID.String()),
		zap.String("tournament_id", tournamentID.String()),
		zap.String("game_type", tournament.GameType),
		zap.String("priority", string(priority)),
	)

	return match, nil
}

// GetMatches получает матчи турнира
func (s *Service) GetMatches(ctx context.Context, tournamentID uuid.UUID, limit, offset int) ([]*domain.Match, error) {
	return s.matchRepo.GetByTournamentID(ctx, tournamentID, limit, offset)
}

// GetMatchesByRounds получает матчи турнира сгруппированные по раундам
func (s *Service) GetMatchesByRounds(ctx context.Context, tournamentID uuid.UUID) ([]*domain.MatchRound, error) {
	return s.matchRepo.GetMatchesByRounds(ctx, tournamentID)
}

// GetCrossGameLeaderboard возвращает кросс-игровой рейтинг турнира
// (команда — рейтинг игры 1 — … — рейтинг игры N — позиция в турнире)
func (s *Service) GetCrossGameLeaderboard(ctx context.Context, tournamentID uuid.UUID) ([]*domain.CrossGameLeaderboardEntry, error) {
	// Try full JSON cache first
	cached, err := s.leaderboardCache.GetFullCrossGameLeaderboard(ctx, tournamentID)
	if err != nil {
		s.log.Error("Failed to get cross-game leaderboard cache", zap.Error(err))
	}
	if cached != nil {
		return cached, nil
	}

	// Cache miss — use singleflight to prevent thundering herd
	sfKey := fmt.Sprintf("crossgame:%s", tournamentID)
	val, err, _ := s.leaderboardSF.Do(sfKey, func() (interface{}, error) {
		entries, err := s.tournamentRepo.GetCrossGameLeaderboard(ctx, tournamentID)
		if err != nil {
			return nil, fmt.Errorf("failed to get cross-game leaderboard: %w", err)
		}

		// Populate cache
		if err := s.leaderboardCache.SetFullCrossGameLeaderboard(ctx, tournamentID, entries); err != nil {
			s.log.Error("Failed to set cross-game leaderboard cache", zap.Error(err))
		}

		return entries, nil
	})
	if err != nil {
		return nil, err
	}
	entries, _ := val.([]*domain.CrossGameLeaderboardEntry)
	return entries, nil
}
