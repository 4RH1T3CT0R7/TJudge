package game

import (
	"context"
	"regexp"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// GameRepository определяет интерфейс репозитория игр
type GameRepository interface {
	Create(ctx context.Context, game *domain.Game) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Game, error)
	GetByName(ctx context.Context, name string) (*domain.Game, error)
	List(ctx context.Context, filter domain.GameFilter) ([]*domain.Game, error)
	Update(ctx context.Context, game *domain.Game) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]*domain.Game, error)
	AddToTournament(ctx context.Context, tournamentID, gameID uuid.UUID) error
	RemoveFromTournament(ctx context.Context, tournamentID, gameID uuid.UUID) error
	Exists(ctx context.Context, name string) (bool, error)
}

// CreateRequest - запрос на создание игры
type CreateRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=50"`
	DisplayName string `json:"display_name" validate:"required,min=1,max=255"`
	Rules       string `json:"rules"`
}

// UpdateRequest - запрос на обновление игры
type UpdateRequest struct {
	DisplayName string `json:"display_name" validate:"required,min=1,max=255"`
	Rules       string `json:"rules"`
}

// Service предоставляет бизнес-логику для работы с играми
type Service struct {
	gameRepo GameRepository
	log      *logger.Logger
}

// NewService создаёт новый сервис игр
func NewService(gameRepo GameRepository, log *logger.Logger) *Service {
	return &Service{
		gameRepo: gameRepo,
		log:      log,
	}
}

// nameRegex - регулярное выражение для проверки имени игры
var nameRegex = regexp.MustCompile(`^[a-z0-9_]+$`)

// Create создаёт новую игру
func (s *Service) Create(ctx context.Context, req *CreateRequest) (*domain.Game, error) {
	// Валидация имени
	if !nameRegex.MatchString(req.Name) {
		return nil, errors.ErrValidation.WithMessage("game name must contain only lowercase letters, digits and underscores")
	}

	// Проверяем уникальность имени
	exists, err := s.gameRepo.Exists(ctx, req.Name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check game existence")
	}
	if exists {
		return nil, errors.ErrConflict.WithMessage("game with this name already exists")
	}

	game := &domain.Game{
		ID:          uuid.New(),
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Rules:       req.Rules,
	}

	if err := s.gameRepo.Create(ctx, game); err != nil {
		return nil, errors.Wrap(err, "failed to create game")
	}

	s.log.Info("Game created", zap.String("game_id", game.ID.String()), zap.String("name", game.Name))

	return game, nil
}

// GetByID получает игру по ID
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*domain.Game, error) {
	game, err := s.gameRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return game, nil
}

// GetByName получает игру по имени
func (s *Service) GetByName(ctx context.Context, name string) (*domain.Game, error) {
	game, err := s.gameRepo.GetByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return game, nil
}

// List получает список игр
func (s *Service) List(ctx context.Context, filter domain.GameFilter) ([]*domain.Game, error) {
	// Применяем дефолтные значения
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 50
	}

	games, err := s.gameRepo.List(ctx, filter)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list games")
	}

	return games, nil
}

// Update обновляет игру
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *UpdateRequest) (*domain.Game, error) {
	game, err := s.gameRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	game.DisplayName = req.DisplayName
	game.Rules = req.Rules

	if err := s.gameRepo.Update(ctx, game); err != nil {
		return nil, errors.Wrap(err, "failed to update game")
	}

	s.log.Info("Game updated", zap.String("game_id", game.ID.String()), zap.String("name", game.Name))

	return game, nil
}

// Delete удаляет игру
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.gameRepo.Delete(ctx, id); err != nil {
		return err
	}

	s.log.Info("Game deleted", zap.String("game_id", id.String()))

	return nil
}

// GetByTournamentID получает игры турнира
func (s *Service) GetByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]*domain.Game, error) {
	games, err := s.gameRepo.GetByTournamentID(ctx, tournamentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tournament games")
	}
	return games, nil
}

// AddToTournament добавляет игру к турниру
func (s *Service) AddToTournament(ctx context.Context, tournamentID, gameID uuid.UUID) error {
	// Проверяем что игра существует
	_, err := s.gameRepo.GetByID(ctx, gameID)
	if err != nil {
		return err
	}

	if err := s.gameRepo.AddToTournament(ctx, tournamentID, gameID); err != nil {
		return errors.Wrap(err, "failed to add game to tournament")
	}

	s.log.Info("Game added to tournament", zap.String("tournament_id", tournamentID.String()), zap.String("game_id", gameID.String()))

	return nil
}

// RemoveFromTournament удаляет игру из турнира
func (s *Service) RemoveFromTournament(ctx context.Context, tournamentID, gameID uuid.UUID) error {
	if err := s.gameRepo.RemoveFromTournament(ctx, tournamentID, gameID); err != nil {
		return err
	}

	s.log.Info("Game removed from tournament", zap.String("tournament_id", tournamentID.String()), zap.String("game_id", gameID.String()))

	return nil
}
