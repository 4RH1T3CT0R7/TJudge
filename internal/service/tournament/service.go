package tournament

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

// TournamentCacher — кэш турниров
type TournamentCacher interface {
	Set(ctx context.Context, tournament *models.Tournament) error
	Get(ctx context.Context, tournamentID uuid.UUID) (*models.Tournament, error)
	Invalidate(ctx context.Context, tournamentID uuid.UUID) error
}

// LeaderboardCacher — кэш лидерборда
// чтение cache-aside в GetLeaderboard/GetCrossGameLeaderboard
type LeaderboardCacher interface {
	// полный json лидерборда: короткий ttl, готовый ответ для api
	GetFullLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*models.LeaderboardEntry, error)
	SetFullLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int, entries []*models.LeaderboardEntry) error
	GetFullCrossGameLeaderboard(ctx context.Context, tournamentID uuid.UUID) ([]*models.CrossGameLeaderboardEntry, error)
	SetFullCrossGameLeaderboard(ctx context.Context, tournamentID uuid.UUID, entries []*models.CrossGameLeaderboardEntry) error
	InvalidateFullLeaderboard(ctx context.Context, tournamentID uuid.UUID) error
}

// TournamentRepository — турниры в бд
type TournamentRepository interface {
	Create(ctx context.Context, tournament *models.Tournament) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Tournament, error)
	List(ctx context.Context, filter models.TournamentFilter) ([]*models.Tournament, error)
	Update(ctx context.Context, tournament *models.Tournament) error
	Complete(ctx context.Context, tournament *models.Tournament) (int64, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.TournamentStatus) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetParticipantsCount(ctx context.Context, tournamentID uuid.UUID) (int, error)
	GetTeamsCount(ctx context.Context, tournamentID uuid.UUID) (int, error)
	GetLatestParticipantsGroupedByGame(ctx context.Context, tournamentID uuid.UUID) (map[string][]*models.TournamentParticipant, error)
	GetLatestParticipantsByGame(ctx context.Context, tournamentID uuid.UUID, gameType string) ([]*models.TournamentParticipant, error)
	AddParticipant(ctx context.Context, participant *models.TournamentParticipant) error
	GetLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*models.LeaderboardEntry, error)
	GetCrossGameLeaderboard(ctx context.Context, tournamentID uuid.UUID) ([]*models.CrossGameLeaderboardEntry, error)
}

// MatchRepository — матчи в бд
type MatchRepository interface {
	Create(ctx context.Context, match *models.Match) error
	DeleteBatch(ctx context.Context, ids []uuid.UUID) error
	GetByTournamentID(ctx context.Context, tournamentID uuid.UUID, limit, offset int) ([]*models.Match, error)
	GetPendingByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]*models.Match, error)
	GetPendingByTournamentAndGame(ctx context.Context, tournamentID uuid.UUID, gameType string) ([]*models.Match, error)
	ResetFailedMatches(ctx context.Context, tournamentID uuid.UUID) (int64, error)
	GetMatchesByRounds(ctx context.Context, tournamentID uuid.UUID, page *models.RoundPage) ([]*models.MatchRound, error)
}

// QueueManager кладёт матчи в очередь
type QueueManager interface {
	Enqueue(ctx context.Context, match *models.Match) error
	EnqueueBatch(ctx context.Context, matches []*models.Match) error
}

// DistributedLock — распределённый лок (редис)
type DistributedLock interface {
	WithLock(ctx context.Context, key string, ttl time.Duration, fn func(ctx context.Context) error) error
}

// GameRepository — игры внутри турнира
type GameRepository interface {
	GetTournamentGames(ctx context.Context, tournamentID uuid.UUID) ([]*models.TournamentGame, error)
	SetActiveGame(ctx context.Context, tournamentID, gameID uuid.UUID) error
	StartNewRound(ctx context.Context, tournamentID uuid.UUID, gameTypes []string, matches []*models.Match) error
	ResetGameRoundFull(ctx context.Context, tournamentID uuid.UUID, gameType string) (matchesDeleted, participantsReset, ratingHistoryDeleted int64, err error)
	// авто-раунд
	GetAutoRoundEnabledGames(ctx context.Context) ([]*models.AutoRoundGameInfo, error)
	UpdateAutoRoundLastRun(ctx context.Context, tournamentID, gameID uuid.UUID) error
	HasNewProgramsSince(ctx context.Context, tournamentID uuid.UUID, gameType string, since time.Time) (bool, error)
	HasActiveMatchesForGame(ctx context.Context, tournamentID uuid.UUID, gameType string) (bool, error)
}

// Service — управление турнирами
type Service struct {
	tournamentRepo   TournamentRepository
	matchRepo        MatchRepository
	queueManager     QueueManager
	gameRepo         GameRepository
	tournamentCache  TournamentCacher
	leaderboardCache LeaderboardCacher
	notifier         events.Notifier
	distributedLock  DistributedLock
	log              *logger.Logger
	leaderboardSF    singleflight.Group
}

func NewService(
	tournamentRepo TournamentRepository,
	matchRepo MatchRepository,
	queueManager QueueManager,
	gameRepo GameRepository,
	tournamentCache TournamentCacher,
	leaderboardCache LeaderboardCacher,
	notifier events.Notifier,
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
		notifier:         notifier,
		distributedLock:  distributedLock,
		log:              log,
	}
}

// CreateRequest — тело запроса на создание турнира
type CreateRequest struct {
	Name            string         `json:"name"`
	Description     string         `json:"description,omitempty"`
	GameType        string         `json:"game_type"`
	MaxParticipants *int           `json:"max_participants,omitempty"`
	MaxTeamSize     int            `json:"max_team_size,omitempty"`
	IsPermanent     bool           `json:"is_permanent,omitempty"`
	StartTime       *time.Time     `json:"start_time,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	CreatorID       *uuid.UUID     `json:"-"` // ставится из контекста, а не из json
}

// generateCode — код турнира на 6 символов
// берётся crypto/rand чтобы символы падали равномерно
func generateCode() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // выкинул похожие символы I,O,0,1
	code := make([]byte, 6)
	max := big.NewInt(int64(len(charset)))
	for i := range code {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			// если crypto/rand отвалился — сломана энтропия ос, тут лучше паника чем тихий мусор
			panic("crypto/rand.Int failed: " + err.Error())
		}
		code[i] = charset[n.Int64()]
	}
	return string(code)
}

// Create создаёт турнир
func (s *Service) Create(ctx context.Context, req *CreateRequest) (*models.Tournament, error) {
	// дефолты
	maxTeamSize := req.MaxTeamSize
	if maxTeamSize <= 0 {
		maxTeamSize = 1
	}

	tournament := &models.Tournament{
		ID:              uuid.New(),
		Code:            generateCode(),
		Name:            req.Name,
		Description:     req.Description,
		GameType:        req.GameType,
		Status:          models.TournamentPending,
		MaxParticipants: req.MaxParticipants,
		MaxTeamSize:     maxTeamSize,
		IsPermanent:     req.IsPermanent,
		StartTime:       req.StartTime,
		Metadata:        req.Metadata,
		CreatorID:       req.CreatorID,
	}

	// валидация
	if err := tournament.Validate(); err != nil {
		return nil, errors.ErrValidation.WithError(err)
	}

	// сохранение
	if err := s.tournamentRepo.Create(ctx, tournament); err != nil {
		return nil, fmt.Errorf("failed to create tournament: %w", err)
	}

	s.log.Info("Tournament created",
		zap.String("tournament_id", tournament.ID.String()),
		zap.String("name", tournament.Name),
		zap.String("game_type", tournament.GameType),
	)

	// уходит событие, кэш чистится в обработчиках
	s.notifier.TournamentCreated(ctx, events.TournamentCreated{Version: 1, Tournament: tournament})

	return tournament, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*models.Tournament, error) {
	// сначала кэш
	cached, err := s.tournamentCache.Get(ctx, id)
	if err == nil && cached != nil {
		return cached, nil
	}

	// иначе из бд
	tournament, err := s.tournamentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// запись в кэш
	if err := s.tournamentCache.Set(ctx, tournament); err != nil {
		s.log.Error("Failed to cache tournament", zap.Error(err))
	}

	return tournament, nil
}

func (s *Service) List(ctx context.Context, filter models.TournamentFilter) ([]*models.Tournament, error) {
	// лимит по дефолту, чтобы не тащить всё
	// TODO: дефолт 50 и потолок 100 захардкожены, по-хорошему из конфига
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	tournaments, err := s.tournamentRepo.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	return tournaments, nil
}

// JoinRequest — тело запроса на join
type JoinRequest struct {
	TournamentID uuid.UUID `json:"tournament_id"`
	ProgramID    uuid.UUID `json:"program_id"`
}

// Join добавляет участника в турнир
func (s *Service) Join(ctx context.Context, req *JoinRequest) error {
	// лок, иначе гонка на проверке лимита участников
	lockKey := fmt.Sprintf("tournament:join:%s", req.TournamentID.String())

	return s.distributedLock.WithLock(ctx, lockKey, 5*time.Second, func(ctx context.Context) error {
		// берётся турнир
		tournament, err := s.GetByID(ctx, req.TournamentID)
		if err != nil {
			return err
		}

		// турнир должен быть ещё pending
		if tournament.Status != models.TournamentPending {
			return errors.ErrTournamentStarted
		}

		// проверка лимита
		if tournament.MaxParticipants != nil {
			count, err := s.tournamentRepo.GetParticipantsCount(ctx, req.TournamentID)
			if err != nil {
				return fmt.Errorf("failed to get participants count: %w", err)
			}

			if count >= *tournament.MaxParticipants {
				return errors.ErrTournamentFull
			}
		}

		// добавление участника
		participant := &models.TournamentParticipant{
			ID:           uuid.New(),
			TournamentID: req.TournamentID,
			ProgramID:    req.ProgramID,
			Rating:       1500, // стартовый эло
		}

		if err := s.tournamentRepo.AddParticipant(ctx, participant); err != nil {
			return fmt.Errorf("failed to add participant: %w", err)
		}

		s.log.Info("Participant joined tournament",
			zap.String("tournament_id", req.TournamentID.String()),
			zap.String("program_id", req.ProgramID.String()),
		)

		// событие: лидерборд и кэш обновятся в обработчиках
		s.notifier.ParticipantJoined(ctx, events.ParticipantJoined{
			Version:       1,
			TournamentID:  req.TournamentID,
			ProgramID:     req.ProgramID,
			InitialRating: 1500,
		})

		return nil
	})
}

// Start переводит турнир в active и включает первую игру
// матчи сами не создаются, их запускает админ руками
func (s *Service) Start(ctx context.Context, tournamentID uuid.UUID) error {
	// лок чтобы не стартануть турнир дважды
	lockKey := fmt.Sprintf("tournament:start:%s", tournamentID.String())

	lockErr := s.distributedLock.WithLock(ctx, lockKey, 60*time.Second, func(ctx context.Context) error {
		// чтение прямо из бд мимо кэша, иначе конфликт версий
		// на оптимистичной блокировке
		tournament, err := s.tournamentRepo.GetByID(ctx, tournamentID)
		if err != nil {
			return err
		}

		// статус
		if tournament.Status != models.TournamentPending {
			return errors.ErrConflict.WithMessage("tournament already started or completed")
		}

		// нужно минимум 2 команды
		teamsCount, err := s.tournamentRepo.GetTeamsCount(ctx, tournamentID)
		if err != nil {
			return fmt.Errorf("failed to get teams count: %w", err)
		}
		if teamsCount < 2 {
			return errors.ErrValidation.WithMessage("для старта турнира нужно минимум 2 команды")
		}

		// смена статуса
		now := time.Now()
		tournament.Status = models.TournamentActive
		tournament.StartTime = &now

		if err := s.tournamentRepo.Update(ctx, tournament); err != nil {
			s.log.Error("Failed to update tournament", zap.Error(err))
			return errors.ErrInternal.WithMessage("failed to update tournament status")
		}

		s.log.Info("Tournament started",
			zap.String("tournament_id", tournamentID.String()),
		)

		// активация первой игры елси она есть
		if s.gameRepo != nil {
			games, err := s.gameRepo.GetTournamentGames(ctx, tournamentID)
			if err != nil {
				s.log.Warn("Failed to get tournament games", zap.Error(err))
			} else if len(games) > 0 {
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

		// событие: кэш и broadcast в обработчиках
		s.notifier.TournamentStarted(ctx, events.TournamentStarted{
			Version:      1,
			TournamentID: tournamentID,
			Status:       tournament.Status,
			StartTime:    tournament.StartTime,
		})

		return nil
	})

	// разбор ошибки лока
	if lockErr != nil {
		if errors.IsAppError(lockErr) {
			return lockErr
		}
		s.log.Error("Lock error during tournament start", zap.Error(lockErr))
		return errors.ErrConflict.WithMessage("could not start tournament, try again later")
	}
	return nil
}

// Complete завершает турнир. недоигранные матчи отменяются, чтобы итоговые
// результаты больше не менялись
func (s *Service) Complete(ctx context.Context, tournamentID uuid.UUID) error {
	// лок планирования: и от двойного завершения, и от запуска раунда параллельно
	lockErr := s.distributedLock.WithLock(ctx, scheduleLockKey(tournamentID), 60*time.Second, func(ctx context.Context) error {
		// прямо из бд, кэш может быть протухшим
		tournament, err := s.tournamentRepo.GetByID(ctx, tournamentID)
		if err != nil {
			return err
		}

		if tournament.Status != models.TournamentActive {
			return errors.ErrConflict.WithMessage("tournament is not active")
		}

		now := time.Now()
		tournament.Status = models.TournamentCompleted
		tournament.EndTime = &now

		// смена статуса и отмена недоигранных матчей - одна транзакция
		cancelled, err := s.tournamentRepo.Complete(ctx, tournament)
		if err != nil {
			return fmt.Errorf("failed to complete tournament: %w", err)
		}

		s.log.Info("Tournament completed",
			zap.String("tournament_id", tournamentID.String()),
			zap.Int64("matches_cancelled", cancelled),
		)

		// событие: кэш и broadcast в обработчиках
		s.notifier.TournamentCompleted(ctx, events.TournamentCompleted{
			Version:      1,
			TournamentID: tournamentID,
			Status:       tournament.Status,
			EndTime:      tournament.EndTime,
		})

		return nil
	})

	// ошибка лока
	if lockErr != nil {
		if errors.IsAppError(lockErr) {
			return lockErr
		}
		s.log.Error("Lock error during tournament completion", zap.Error(lockErr))
		return errors.ErrConflict.WithMessage("could not complete tournament, try again later")
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, tournamentID uuid.UUID) error {
	// подтягивается турнир для проверки
	tournament, err := s.GetByID(ctx, tournamentID)
	if err != nil {
		return err
	}

	// активный турнир удалять нельзя
	if tournament.Status == models.TournamentActive {
		return errors.ErrConflict.WithMessage("cannot delete active tournament")
	}

	// удаление
	if err := s.tournamentRepo.Delete(ctx, tournamentID); err != nil {
		return fmt.Errorf("failed to delete tournament: %w", err)
	}

	s.log.Info("Tournament deleted",
		zap.String("tournament_id", tournamentID.String()),
	)

	// событие, кэш чистится в обработчиках
	s.notifier.TournamentDeleted(ctx, events.TournamentDeleted{Version: 1, TournamentID: tournamentID})

	return nil
}

// GetLeaderboard — таблица лидеров
func (s *Service) GetLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*models.LeaderboardEntry, error) {
	// сначала полный json-кэш (короткий ttl)
	// TODO: лидерборд без пагинации, отдаётся весь топ как есть
	cached, err := s.leaderboardCache.GetFullLeaderboard(ctx, tournamentID, limit)
	if err != nil {
		s.log.Error("Failed to get full leaderboard cache", zap.Error(err))
	}
	if cached != nil {
		return cached, nil
	}

	// промах кэша: singleflight чтобы не долбить бд толпой (thundering herd)
	// FIXME: формат sfKey руками повторяет ключ кэша, разъедутся - схлопывать перестанет
	sfKey := fmt.Sprintf("leaderboard:%s:%d", tournamentID, limit)
	val, err, _ := s.leaderboardSF.Do(sfKey, func() (any, error) {
		leaderboard, err := s.tournamentRepo.GetLeaderboard(ctx, tournamentID, limit)
		if err != nil {
			return nil, err
		}

		// полный json кладётся в кэш
		if err := s.leaderboardCache.SetFullLeaderboard(ctx, tournamentID, limit, leaderboard); err != nil {
			s.log.Error("Failed to set full leaderboard cache", zap.Error(err))
		}

		return leaderboard, nil
	})
	if err != nil {
		return nil, err
	}
	entries, _ := val.([]*models.LeaderboardEntry)
	return entries, nil
}

// CreateMatch создаёт матч и добавляет в очередь
func (s *Service) CreateMatch(ctx context.Context, tournamentID, program1ID, program2ID uuid.UUID, priority models.MatchPriority) (*models.Match, error) {
	// турнир нужен ради game_type
	tournament, err := s.GetByID(ctx, tournamentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tournament: %w", err)
	}

	match := &models.Match{
		ID:           uuid.New(),
		TournamentID: tournamentID,
		Program1ID:   program1ID,
		Program2ID:   program2ID,
		GameType:     tournament.GameType,
		Status:       models.MatchPending,
		Priority:     priority,
		CreatedAt:    time.Now(),
	}

	// валидация
	if err := match.Validate(); err != nil {
		return nil, errors.ErrValidation.WithError(err)
	}

	// сохранение в бд
	if err := s.matchRepo.Create(ctx, match); err != nil {
		return nil, fmt.Errorf("failed to create match: %w", err)
	}

	// в очередь
	if err := s.queueManager.Enqueue(ctx, match); err != nil {
		s.log.Error("Failed to enqueue match",
			zap.Error(err),
			zap.String("match_id", match.ID.String()),
		)
		// ошибка не возвращается, матч уже в бд тк создан выше
	}

	s.log.Info("Match created",
		zap.String("match_id", match.ID.String()),
		zap.String("tournament_id", tournamentID.String()),
		zap.String("game_type", tournament.GameType),
		zap.String("priority", string(priority)),
	)

	return match, nil
}

func (s *Service) GetMatches(ctx context.Context, tournamentID uuid.UUID, limit, offset int) ([]*models.Match, error) {
	return s.matchRepo.GetByTournamentID(ctx, tournamentID, limit, offset)
}

func (s *Service) GetMatchesByRounds(ctx context.Context, tournamentID uuid.UUID, page *models.RoundPage) ([]*models.MatchRound, error) {
	return s.matchRepo.GetMatchesByRounds(ctx, tournamentID, page)
}

// GetCrossGameLeaderboard — кросс-игровой рейтинг
// команда, рейтинги по каждой игре, итоговая позиция
func (s *Service) GetCrossGameLeaderboard(ctx context.Context, tournamentID uuid.UUID) ([]*models.CrossGameLeaderboardEntry, error) {
	// сначала кэш
	cached, err := s.leaderboardCache.GetFullCrossGameLeaderboard(ctx, tournamentID)
	if err != nil {
		s.log.Error("Failed to get cross-game leaderboard cache", zap.Error(err))
	}
	if cached != nil {
		return cached, nil
	}

	// промах, singleflight от thundering herd
	sfKey := fmt.Sprintf("crossgame:%s", tournamentID)
	val, err, _ := s.leaderboardSF.Do(sfKey, func() (any, error) {
		entries, err := s.tournamentRepo.GetCrossGameLeaderboard(ctx, tournamentID)
		if err != nil {
			return nil, fmt.Errorf("failed to get cross-game leaderboard: %w", err)
		}

		// в кэш
		if err := s.leaderboardCache.SetFullCrossGameLeaderboard(ctx, tournamentID, entries); err != nil {
			s.log.Error("Failed to set cross-game leaderboard cache", zap.Error(err))
		}

		return entries, nil
	})
	if err != nil {
		return nil, err
	}
	entries, _ := val.([]*models.CrossGameLeaderboardEntry)
	return entries, nil
}
