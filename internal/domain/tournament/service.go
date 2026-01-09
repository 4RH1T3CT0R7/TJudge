package tournament

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/cache"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TournamentRepository интерфейс для работы с турнирами
type TournamentRepository interface {
	Create(ctx context.Context, tournament *domain.Tournament) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error)
	List(ctx context.Context, filter domain.TournamentFilter) ([]*domain.Tournament, error)
	Update(ctx context.Context, tournament *domain.Tournament) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.TournamentStatus) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetParticipantsCount(ctx context.Context, tournamentID uuid.UUID) (int, error)
	GetParticipants(ctx context.Context, tournamentID uuid.UUID) ([]*domain.TournamentParticipant, error)
	GetLatestParticipants(ctx context.Context, tournamentID uuid.UUID) ([]*domain.TournamentParticipant, error)
	GetLatestParticipantsGroupedByGame(ctx context.Context, tournamentID uuid.UUID) (map[string][]*domain.TournamentParticipant, error)
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
}

// Broadcaster интерфейс для broadcast обновлений
type Broadcaster interface {
	Broadcast(tournamentID uuid.UUID, messageType string, payload interface{})
}

// DistributedLock интерфейс для распределённых блокировок
type DistributedLock interface {
	WithLock(ctx context.Context, key string, ttl time.Duration, fn func(ctx context.Context) error) error
}

// Service - сервис управления турнирами
type Service struct {
	tournamentRepo   TournamentRepository
	matchRepo        MatchRepository
	queueManager     QueueManager
	tournamentCache  *cache.TournamentCache
	leaderboardCache *cache.LeaderboardCache
	broadcaster      Broadcaster
	distributedLock  DistributedLock
	log              *logger.Logger
}

// NewService создаёт новый сервис турниров
func NewService(
	tournamentRepo TournamentRepository,
	matchRepo MatchRepository,
	queueManager QueueManager,
	tournamentCache *cache.TournamentCache,
	leaderboardCache *cache.LeaderboardCache,
	broadcaster Broadcaster,
	distributedLock DistributedLock,
	log *logger.Logger,
) *Service {
	return &Service{
		tournamentRepo:   tournamentRepo,
		matchRepo:        matchRepo,
		queueManager:     queueManager,
		tournamentCache:  tournamentCache,
		leaderboardCache: leaderboardCache,
		broadcaster:      broadcaster,
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

// generateCode генерирует уникальный код турнира (6-8 символов)
func generateCode() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // без похожих символов I,O,0,1
	code := make([]byte, 6)
	for i := range code {
		code[i] = charset[uuid.New()[i]%byte(len(charset))]
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

	// Кэшируем
	if err := s.tournamentCache.Set(ctx, tournament); err != nil {
		s.log.Error("Failed to cache tournament", zap.Error(err))
	}

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

		// Инвалидируем кэш
		_ = s.tournamentCache.Invalidate(ctx, req.TournamentID)

		// Добавляем в leaderboard кэш
		if err := s.leaderboardCache.UpdateRating(ctx, req.TournamentID, req.ProgramID, 1500); err != nil {
			s.log.Error("Failed to update leaderboard cache", zap.Error(err))
		}

		return nil
	})
}

// Start запускает турнир и генерирует матчи
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

		// Получаем участников сгруппированных по играм
		participantsByGame, err := s.tournamentRepo.GetLatestParticipantsGroupedByGame(ctx, tournamentID)
		if err != nil {
			s.log.Error("Failed to get participants grouped by game", zap.Error(err))
			return errors.ErrInternal.WithMessage("failed to get participants")
		}

		// Считаем общее количество участников
		totalParticipants := 0
		for _, participants := range participantsByGame {
			totalParticipants += len(participants)
		}

		// Проверяем минимальное количество участников
		if totalParticipants < 2 {
			return errors.ErrValidation.WithMessage(fmt.Sprintf("tournament needs at least 2 participants to start, got %d", totalParticipants))
		}

		s.log.Info("Starting tournament with participants",
			zap.String("tournament_id", tournamentID.String()),
			zap.Int("total_participants", totalParticipants),
			zap.Int("games_count", len(participantsByGame)),
		)

		// Сортируем игры для детерминированного порядка выполнения
		// Первая игра (по алфавиту) получает HIGH приоритет, вторая - MEDIUM, остальные - LOW
		gameTypes := make([]string, 0, len(participantsByGame))
		for gameType := range participantsByGame {
			gameTypes = append(gameTypes, gameType)
		}
		sort.Strings(gameTypes)

		// Генерируем матчи для каждой игры отдельно
		var allMatches []*domain.Match
		for gameIndex, gameType := range gameTypes {
			participants := participantsByGame[gameType]
			if len(participants) < 2 {
				s.log.Info("Skipping game with less than 2 participants",
					zap.String("game_type", gameType),
					zap.Int("participants", len(participants)),
				)
				continue
			}

			// Определяем приоритет на основе порядка игры
			// Первая игра = HIGH (выполняется первой), вторая = MEDIUM, остальные = LOW
			var priority domain.MatchPriority
			switch gameIndex {
			case 0:
				priority = domain.PriorityHigh
			case 1:
				priority = domain.PriorityMedium
			default:
				priority = domain.PriorityLow
			}

			// Получаем следующий номер раунда для этой игры
			roundNumber, err := s.matchRepo.GetNextRoundNumberByGame(ctx, tournamentID, gameType)
			if err != nil {
				s.log.Warn("Failed to get next round number for game, using 1",
					zap.Error(err),
					zap.String("game_type", gameType),
				)
				roundNumber = 1
			}

			// Генерируем round-robin матчи для этой игры
			matches, err := s.generateRoundRobinMatchesForGame(tournament, participants, gameType, roundNumber, priority)
			if err != nil {
				s.log.Error("Failed to generate matches for game",
					zap.Error(err),
					zap.String("game_type", gameType),
				)
				return errors.ErrInternal.WithMessage(fmt.Sprintf("failed to generate matches for game %s", gameType))
			}

			s.log.Info("Generated matches for game",
				zap.String("game_type", gameType),
				zap.Int("participants", len(participants)),
				zap.Int("matches", len(matches)),
				zap.String("priority", string(priority)),
			)

			allMatches = append(allMatches, matches...)
		}

		if len(allMatches) == 0 {
			return errors.ErrValidation.WithMessage("no matches could be generated - need at least 2 participants per game")
		}

		// Создаём матчи в БД
		if err := s.matchRepo.CreateBatch(ctx, allMatches); err != nil {
			s.log.Error("Failed to create matches", zap.Error(err))
			return errors.ErrInternal.WithMessage("failed to create matches")
		}

		// Добавляем матчи в очередь
		for _, match := range allMatches {
			if err := s.queueManager.Enqueue(ctx, match); err != nil {
				s.log.Error("Failed to enqueue match",
					zap.Error(err),
					zap.String("match_id", match.ID.String()),
				)
				// Продолжаем, даже если какой-то матч не удалось добавить в очередь
			}
		}

		// Обновляем статус турнира
		now := time.Now()
		tournament.Status = domain.TournamentActive
		tournament.StartTime = &now

		if err := s.tournamentRepo.Update(ctx, tournament); err != nil {
			s.log.Error("Failed to update tournament", zap.Error(err))
			return errors.ErrInternal.WithMessage("failed to update tournament status")
		}

		s.log.Info("Tournament started with matches",
			zap.String("tournament_id", tournamentID.String()),
			zap.Int("matches_created", len(allMatches)),
			zap.Int("games", len(participantsByGame)),
		)

		// Инвалидируем кэш
		_ = s.tournamentCache.Invalidate(ctx, tournamentID)

		// Отправляем broadcast обновление
		s.broadcaster.Broadcast(tournamentID, "tournament_update", map[string]interface{}{
			"status":        tournament.Status,
			"matches_count": len(allMatches),
			"start_time":    tournament.StartTime,
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

// generateRoundRobinMatches генерирует матчи по системе round-robin (каждый с каждым)
// Каждая пара играет 2 матча (AB и BA), итерации выполняются внутри tjudge-cli через параметр -i
// Рейтинг = сумма очков из всех матчей
func (s *Service) generateRoundRobinMatches(tournament *domain.Tournament, participants []*domain.TournamentParticipant, roundNumber int) ([]*domain.Match, error) {
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
				GameType:     tournament.GameType,
				Status:       domain.MatchPending,
				Priority:     domain.PriorityMedium,
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

// Complete завершает турнир
func (s *Service) Complete(ctx context.Context, tournamentID uuid.UUID) error {
	tournament, err := s.GetByID(ctx, tournamentID)
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

	_ = s.tournamentCache.Invalidate(ctx, tournamentID)

	// Отправляем broadcast обновление
	s.broadcaster.Broadcast(tournamentID, "tournament_update", map[string]interface{}{
		"status":   tournament.Status,
		"end_time": tournament.EndTime,
	})

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

	// Инвалидируем кэш
	_ = s.tournamentCache.Invalidate(ctx, tournamentID)

	return nil
}

// GetLeaderboard получает таблицу лидеров турнира
func (s *Service) GetLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*domain.LeaderboardEntry, error) {
	// Проверяем кэш
	cached, err := s.leaderboardCache.GetTop(ctx, tournamentID, limit)
	if err == nil && cached != nil && len(cached) > 0 {
		return cached, nil
	}

	// Получаем из БД
	leaderboard, err := s.tournamentRepo.GetLeaderboard(ctx, tournamentID, limit)
	if err != nil {
		return nil, err
	}

	// Обновляем кэш
	for _, entry := range leaderboard {
		if err := s.leaderboardCache.UpdateRating(ctx, tournamentID, entry.ProgramID, entry.Rating); err != nil {
			s.log.Error("Failed to update leaderboard cache", zap.Error(err))
		}
	}

	return leaderboard, nil
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

			match := &domain.Match{
				ID:           uuid.New(),
				TournamentID: req.TournamentID,
				Program1ID:   req.NewProgramID,
				Program2ID:   prog.ID,
				GameType:     tournament.GameType,
				Status:       domain.MatchPending,
				Priority:     domain.PriorityHigh, // Новые матчи с высоким приоритетом
				CreatedAt:    now,
			}

			if err := match.Validate(); err != nil {
				s.log.Error("Invalid match generated",
					zap.Error(err),
					zap.String("program1_id", req.NewProgramID.String()),
					zap.String("program2_id", prog.ID.String()),
				)
				continue
			}

			matches = append(matches, match)
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

		// Добавляем матчи в очередь
		for _, match := range matches {
			if err := s.queueManager.Enqueue(ctx, match); err != nil {
				s.log.Error("Failed to enqueue match",
					zap.Error(err),
					zap.String("match_id", match.ID.String()),
				)
			}
		}

		s.log.Info("New program matches scheduled",
			zap.String("tournament_id", req.TournamentID.String()),
			zap.String("program_id", req.NewProgramID.String()),
			zap.Int("matches_created", len(matches)),
		)

		// Отправляем broadcast обновление
		s.broadcaster.Broadcast(req.TournamentID, "matches_created", map[string]interface{}{
			"program_id":    req.NewProgramID.String(),
			"matches_count": len(matches),
		})

		return nil
	})
}

// GetCrossGameLeaderboard возвращает кросс-игровой рейтинг турнира
// (команда — рейтинг игры 1 — … — рейтинг игры N — позиция в турнире)
func (s *Service) GetCrossGameLeaderboard(ctx context.Context, tournamentID uuid.UUID) ([]*domain.CrossGameLeaderboardEntry, error) {
	// Получаем все программы с их статистикой по играм
	entries, err := s.tournamentRepo.GetCrossGameLeaderboard(ctx, tournamentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cross-game leaderboard: %w", err)
	}

	return entries, nil
}

// RunAllMatches запускает все pending матчи турнира (для админа)
// Если нет pending матчей, создаёт новый раунд round-robin матчей
func (s *Service) RunAllMatches(ctx context.Context, tournamentID uuid.UUID) (int, error) {
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

		// Получаем участников (только последние версии программ каждой команды)
		participants, err := s.tournamentRepo.GetLatestParticipants(ctx, tournamentID)
		if err != nil {
			return 0, fmt.Errorf("failed to get participants: %w", err)
		}

		if len(participants) < 2 {
			return 0, errors.ErrValidation.WithMessage("need at least 2 participants to run matches")
		}

		// Получаем следующий номер раунда
		roundNumber, err := s.matchRepo.GetNextRoundNumber(ctx, tournamentID)
		if err != nil {
			s.log.Warn("Failed to get next round number, using 1",
				zap.Error(err),
			)
			roundNumber = 1
		}

		// Генерируем новый раунд матчей
		matches, err = s.generateRoundRobinMatches(tournament, participants, roundNumber)
		if err != nil {
			return 0, fmt.Errorf("failed to generate matches: %w", err)
		}

		// Сохраняем матчи в БД
		if err := s.matchRepo.CreateBatch(ctx, matches); err != nil {
			return 0, fmt.Errorf("failed to create matches: %w", err)
		}

		s.log.Info("Generated new round of matches",
			zap.String("tournament_id", tournamentID.String()),
			zap.Int("round_number", roundNumber),
			zap.Int("matches_count", len(matches)),
		)
	}

	// Добавляем все матчи в очередь
	enqueued := 0
	for _, match := range matches {
		if err := s.queueManager.Enqueue(ctx, match); err != nil {
			s.log.Error("Failed to enqueue match",
				zap.Error(err),
				zap.String("match_id", match.ID.String()),
			)
			continue
		}
		enqueued++
	}

	s.log.Info("Admin triggered all matches",
		zap.String("tournament_id", tournamentID.String()),
		zap.Int("total_pending", len(matches)),
		zap.Int("enqueued", enqueued),
	)

	return enqueued, nil
}

// RunGameMatches запускает матчи для конкретной игры в турнире
func (s *Service) RunGameMatches(ctx context.Context, tournamentID uuid.UUID, gameType string) (int, error) {
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

	// Добавляем все матчи в очередь
	enqueued := 0
	for _, match := range matches {
		if err := s.queueManager.Enqueue(ctx, match); err != nil {
			s.log.Error("Failed to enqueue match",
				zap.Error(err),
				zap.String("match_id", match.ID.String()),
			)
			continue
		}
		enqueued++
	}

	s.log.Info("Admin triggered game matches",
		zap.String("tournament_id", tournamentID.String()),
		zap.String("game_type", gameType),
		zap.Int("total_pending", len(matches)),
		zap.Int("enqueued", enqueued),
	)

	return enqueued, nil
}

// getLatestParticipantsByGame получает последние версии программ участников для конкретной игры
func (s *Service) getLatestParticipantsByGame(ctx context.Context, tournamentID uuid.UUID, gameType string) ([]*domain.TournamentParticipant, error) {
	// Получаем всех участников
	allParticipants, err := s.tournamentRepo.GetLatestParticipants(ctx, tournamentID)
	if err != nil {
		return nil, err
	}

	// Фильтруем по игре - это требует проверки game_id программы
	// Для простоты пока возвращаем всех участников
	// TODO: добавить фильтрацию по game_id
	return allParticipants, nil
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

	enqueued := 0
	for _, match := range matches {
		if err := s.queueManager.Enqueue(ctx, match); err != nil {
			s.log.Error("Failed to enqueue match",
				zap.Error(err),
				zap.String("match_id", match.ID.String()),
			)
			continue
		}
		enqueued++
	}

	s.log.Info("Admin retried failed matches",
		zap.String("tournament_id", tournamentID.String()),
		zap.Int64("reset_count", resetCount),
		zap.Int("enqueued", enqueued),
	)

	return enqueued, nil
}
