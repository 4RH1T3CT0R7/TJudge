package tournament

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
	s.eventBus.Publish(ctx, events.TournamentCreated{Tournament: tournament})

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
	s.eventBus.Publish(ctx, events.TournamentDeleted{TournamentID: tournamentID})

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

// ProgramRepository интерфейс для работы с программами (для оптимизированного round-robin)
type ProgramRepository interface {
	GetByTournamentAndGame(ctx context.Context, tournamentID, gameID uuid.UUID) ([]*domain.Program, error)
}

// ScheduleNewProgramMatchesRequest запрос на создание матчей для новой программы
type ScheduleNewProgramMatchesRequest struct {
	TournamentID uuid.UUID
	GameID       uuid.UUID
	NewProgramID uuid.UUID
	TeamID       uuid.UUID
}

// ScheduleNewProgramMatches создаёт матчи для новой программы против всех существующих
// Это оптимизированный round-robin - вместо генерации всех матчей заново,
// создаются только матчи с новой программой
func (s *Service) ScheduleNewProgramMatches(ctx context.Context, req *ScheduleNewProgramMatchesRequest, programRepo ProgramRepository) error {
	// Используем distributed lock для предотвращения гонок при создании матчей
	lockKey := fmt.Sprintf("tournament:schedule:%s:%s", req.TournamentID.String(), req.GameID.String())

	return s.distributedLock.WithLock(ctx, lockKey, 10*time.Second, func(ctx context.Context) error {
		// Получаем турнир
		tournament, err := s.GetByID(ctx, req.TournamentID)
		if err != nil {
			return err
		}

		// Проверяем статус турнира
		if tournament.Status != domain.TournamentActive && tournament.Status != domain.TournamentPending {
			return errors.ErrConflict.WithMessage("cannot schedule matches for completed tournament")
		}

		// Получаем все программы в турнире для данной игры
		programs, err := programRepo.GetByTournamentAndGame(ctx, req.TournamentID, req.GameID)
		if err != nil {
			return fmt.Errorf("failed to get programs: %w", err)
		}

		// Создаём матчи только против других программ (не своей команды)
		var matches []*domain.Match
		now := time.Now()

		for _, prog := range programs {
			// Пропускаем свою программу и программы своей команды
			if prog.ID == req.NewProgramID {
				continue
			}
			if prog.TeamID != nil && *prog.TeamID == req.TeamID {
				continue
			}

			// Матч 1: новая программа как Program1, существующая как Program2
			match1 := &domain.Match{
				ID:           uuid.New(),
				TournamentID: req.TournamentID,
				Program1ID:   req.NewProgramID,
				Program2ID:   prog.ID,
				GameType:     tournament.GameType,
				Status:       domain.MatchPending,
				Priority:     domain.PriorityHigh, // Новые матчи с высоким приоритетом
				CreatedAt:    now,
			}

			if err := match1.Validate(); err != nil {
				s.log.Error("Invalid match generated",
					zap.Error(err),
					zap.String("program1_id", req.NewProgramID.String()),
					zap.String("program2_id", prog.ID.String()),
				)
				continue
			}

			// Матч 2: существующая программа как Program1, новая как Program2
			match2 := &domain.Match{
				ID:           uuid.New(),
				TournamentID: req.TournamentID,
				Program1ID:   prog.ID,
				Program2ID:   req.NewProgramID,
				GameType:     tournament.GameType,
				Status:       domain.MatchPending,
				Priority:     domain.PriorityHigh, // Новые матчи с высоким приоритетом
				CreatedAt:    now,
			}

			if err := match2.Validate(); err != nil {
				s.log.Error("Invalid reverse match generated",
					zap.Error(err),
					zap.String("program1_id", prog.ID.String()),
					zap.String("program2_id", req.NewProgramID.String()),
				)
				continue
			}

			matches = append(matches, match1, match2)
		}

		if len(matches) == 0 {
			s.log.Info("No new matches to schedule",
				zap.String("tournament_id", req.TournamentID.String()),
				zap.String("program_id", req.NewProgramID.String()),
			)
			return nil
		}

		// Создаём матчи в БД
		if err := s.matchRepo.CreateBatch(ctx, matches); err != nil {
			return fmt.Errorf("failed to create matches: %w", err)
		}

		// Добавляем матчи в очередь (batch — один Redis pipeline)
		if err := s.queueManager.EnqueueBatch(ctx, matches); err != nil {
			return fmt.Errorf("failed to enqueue matches: %w", err)
		}

		s.log.Info("New program matches scheduled",
			zap.String("tournament_id", req.TournamentID.String()),
			zap.String("program_id", req.NewProgramID.String()),
			zap.Int("matches_created", len(matches)),
		)

		// Publish event (broadcast handled by event handlers)
		s.eventBus.Publish(ctx, events.MatchesCreated{
			TournamentID: req.TournamentID,
			ProgramID:    req.NewProgramID,
			MatchCount:   len(matches),
		})

		return nil
	})
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

// RunAllMatches запускает все pending матчи турнира (для админа)
// Если нет pending матчей, создаёт новый раунд round-robin матчей
func (s *Service) RunAllMatches(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	// Используем distributed lock для предотвращения дублирования матчей
	lockKey := fmt.Sprintf("tournament:run_matches:%s", tournamentID.String())

	var enqueued int
	lockErr := s.distributedLock.WithLock(ctx, lockKey, 60*time.Second, func(ctx context.Context) error {
		var err error
		enqueued, err = s.runAllMatchesLocked(ctx, tournamentID)
		return err
	})

	return enqueued, lockErr
}

func (s *Service) runAllMatchesLocked(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	// Получаем все pending матчи
	matches, err := s.matchRepo.GetPendingByTournamentID(ctx, tournamentID)
	if err != nil {
		return 0, fmt.Errorf("failed to get pending matches: %w", err)
	}

	// Если нет pending матчей, создаём новый раунд
	if len(matches) == 0 {
		s.log.Info("No pending matches, generating new round",
			zap.String("tournament_id", tournamentID.String()),
		)

		// Получаем турнир
		tournament, err := s.GetByID(ctx, tournamentID)
		if err != nil {
			return 0, fmt.Errorf("failed to get tournament: %w", err)
		}

		// Проверяем что турнир активен
		if tournament.Status != domain.TournamentActive {
			return 0, errors.ErrConflict.WithMessage("tournament is not active")
		}

		// Получаем участников сгруппированных по играм (чтобы не сводить программы разных игр)
		participantsByGame, err := s.tournamentRepo.GetLatestParticipantsGroupedByGame(ctx, tournamentID)
		if err != nil {
			return 0, fmt.Errorf("failed to get participants: %w", err)
		}

		if len(participantsByGame) == 0 {
			return 0, errors.ErrValidation.WithMessage("need at least 2 participants to run matches")
		}

		// Генерируем матчи отдельно для каждой игры
		for gameType, participants := range participantsByGame {
			if len(participants) < 2 {
				s.log.Warn("Skipping game with fewer than 2 participants",
					zap.String("game_type", gameType),
					zap.Int("participants", len(participants)),
				)
				continue
			}

			roundNumber, err := s.matchRepo.GetNextRoundNumberByGame(ctx, tournamentID, gameType)
			if err != nil {
				s.log.Warn("Failed to get next round number for game, using 1",
					zap.String("game_type", gameType),
					zap.Error(err),
				)
				roundNumber = 1
			}

			gameMatches, err := s.generateRoundRobinMatchesForGame(tournament, participants, gameType, roundNumber, domain.PriorityMedium)
			if err != nil {
				return 0, fmt.Errorf("failed to generate matches for game %s: %w", gameType, err)
			}

			if err := s.matchRepo.CreateBatch(ctx, gameMatches); err != nil {
				return 0, fmt.Errorf("failed to create matches for game %s: %w", gameType, err)
			}

			matches = append(matches, gameMatches...)

			s.log.Info("Generated new round of matches for game",
				zap.String("tournament_id", tournamentID.String()),
				zap.String("game_type", gameType),
				zap.Int("round_number", roundNumber),
				zap.Int("matches_count", len(gameMatches)),
			)
		}
	}

	// Добавляем все матчи в очередь (batch — один Redis pipeline)
	if err := s.queueManager.EnqueueBatch(ctx, matches); err != nil {
		return 0, fmt.Errorf("failed to enqueue matches: %w", err)
	}

	s.log.Info("Admin triggered all matches",
		zap.String("tournament_id", tournamentID.String()),
		zap.Int("total_pending", len(matches)),
		zap.Int("enqueued", len(matches)),
	)

	return len(matches), nil
}

// RunGameMatches запускает матчи для конкретной игры в турнире
func (s *Service) RunGameMatches(ctx context.Context, tournamentID uuid.UUID, gameType string) (int, error) {
	// Используем distributed lock для предотвращения дублирования матчей
	lockKey := fmt.Sprintf("tournament:run_game_matches:%s:%s", tournamentID.String(), gameType)

	var enqueued int
	lockErr := s.distributedLock.WithLock(ctx, lockKey, 60*time.Second, func(ctx context.Context) error {
		var err error
		enqueued, err = s.runGameMatchesLocked(ctx, tournamentID, gameType)
		return err
	})

	return enqueued, lockErr
}

func (s *Service) runGameMatchesLocked(ctx context.Context, tournamentID uuid.UUID, gameType string) (int, error) {
	// Получаем pending матчи для конкретной игры
	matches, err := s.matchRepo.GetPendingByTournamentAndGame(ctx, tournamentID, gameType)
	if err != nil {
		return 0, fmt.Errorf("failed to get pending matches: %w", err)
	}

	// Если нет pending матчей, создаём новый раунд для этой игры
	if len(matches) == 0 {
		s.log.Info("No pending matches for game, generating new round",
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_type", gameType),
		)

		// Получаем турнир
		tournament, err := s.GetByID(ctx, tournamentID)
		if err != nil {
			return 0, fmt.Errorf("failed to get tournament: %w", err)
		}

		// Проверяем что турнир активен
		if tournament.Status != domain.TournamentActive {
			return 0, errors.ErrConflict.WithMessage("tournament is not active")
		}

		// Получаем участников (только последние версии программ каждой команды для этой игры)
		participants, err := s.getLatestParticipantsByGame(ctx, tournamentID, gameType)
		if err != nil {
			return 0, fmt.Errorf("failed to get participants: %w", err)
		}

		if len(participants) < 2 {
			return 0, errors.ErrValidation.WithMessage("need at least 2 participants with programs for this game")
		}

		// Получаем следующий номер раунда для этой игры
		roundNumber, err := s.matchRepo.GetNextRoundNumberByGame(ctx, tournamentID, gameType)
		if err != nil {
			s.log.Warn("Failed to get next round number for game, using 1",
				zap.Error(err),
			)
			roundNumber = 1
		}

		// Генерируем матчи для этой игры с высоким приоритетом (ручной запуск)
		matches, err = s.generateRoundRobinMatchesForGame(tournament, participants, gameType, roundNumber, domain.PriorityHigh)
		if err != nil {
			return 0, fmt.Errorf("failed to generate matches: %w", err)
		}

		// Сохраняем матчи в БД
		if err := s.matchRepo.CreateBatch(ctx, matches); err != nil {
			return 0, fmt.Errorf("failed to create matches: %w", err)
		}

		s.log.Info("Generated new round of matches for game",
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_type", gameType),
			zap.Int("round_number", roundNumber),
			zap.Int("matches_count", len(matches)),
		)
	}

	// Добавляем все матчи в очередь (batch — один Redis pipeline)
	if err := s.queueManager.EnqueueBatch(ctx, matches); err != nil {
		return 0, fmt.Errorf("failed to enqueue matches: %w", err)
	}

	s.log.Info("Admin triggered game matches",
		zap.String("tournament_id", tournamentID.String()),
		zap.String("game_type", gameType),
		zap.Int("total_pending", len(matches)),
		zap.Int("enqueued", len(matches)),
	)

	return len(matches), nil
}

// getLatestParticipantsByGame получает последние версии программ участников для конкретной игры
func (s *Service) getLatestParticipantsByGame(ctx context.Context, tournamentID uuid.UUID, gameType string) ([]*domain.TournamentParticipant, error) {
	return s.tournamentRepo.GetLatestParticipantsByGame(ctx, tournamentID, gameType)
}

// generateRoundRobinMatchesForGame генерирует матчи для конкретной игры
func (s *Service) generateRoundRobinMatchesForGame(tournament *domain.Tournament, participants []*domain.TournamentParticipant, gameType string, roundNumber int, priority domain.MatchPriority) ([]*domain.Match, error) {
	var matches []*domain.Match
	now := time.Now()

	// Каждый участник играет с каждым в обе стороны (AB и BA)
	for i := 0; i < len(participants); i++ {
		for j := 0; j < len(participants); j++ {
			// Пропускаем матч против себя
			if i == j {
				continue
			}

			match := &domain.Match{
				ID:           uuid.New(),
				TournamentID: tournament.ID,
				Program1ID:   participants[i].ProgramID,
				Program2ID:   participants[j].ProgramID,
				GameType:     gameType,
				Status:       domain.MatchPending,
				Priority:     priority,
				RoundNumber:  roundNumber,
				CreatedAt:    now,
			}

			if err := match.Validate(); err != nil {
				return nil, fmt.Errorf("invalid match generated: %w", err)
			}

			matches = append(matches, match)
		}
	}

	return matches, nil
}

// RetryFailedMatches сбрасывает failed матчи в pending и ставит их в очередь
func (s *Service) RetryFailedMatches(ctx context.Context, tournamentID uuid.UUID) (int, error) {
	// Сбрасываем все failed матчи в pending
	resetCount, err := s.matchRepo.ResetFailedMatches(ctx, tournamentID)
	if err != nil {
		return 0, fmt.Errorf("failed to reset failed matches: %w", err)
	}

	if resetCount == 0 {
		return 0, nil
	}

	// Получаем все pending матчи и ставим в очередь
	matches, err := s.matchRepo.GetPendingByTournamentID(ctx, tournamentID)
	if err != nil {
		return 0, fmt.Errorf("failed to get pending matches: %w", err)
	}

	// Добавляем все матчи в очередь (batch — один Redis pipeline)
	if err := s.queueManager.EnqueueBatch(ctx, matches); err != nil {
		return 0, fmt.Errorf("failed to enqueue matches: %w", err)
	}

	s.log.Info("Admin retried failed matches",
		zap.String("tournament_id", tournamentID.String()),
		zap.Int64("reset_count", resetCount),
		zap.Int("enqueued", len(matches)),
	)

	return len(matches), nil
}
