package game

import (
	"context"
	"regexp"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// GameRepository — игры в бд
type GameRepository interface {
	Create(ctx context.Context, game *models.Game) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Game, error)
	GetByName(ctx context.Context, name string) (*models.Game, error)
	List(ctx context.Context, filter models.GameFilter) ([]*models.Game, error)
	Update(ctx context.Context, game *models.Game) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]*models.Game, error)
	AddToTournament(ctx context.Context, tournamentID, gameID uuid.UUID) error
	RemoveFromTournament(ctx context.Context, tournamentID, gameID uuid.UUID) error
	Exists(ctx context.Context, name string) (bool, error)
}

// CreateRequest — тело запроса на создание игры
type CreateRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=50"`
	DisplayName string `json:"display_name" validate:"required,min=1,max=255"`
	Rules       string `json:"rules"`
}

// UpdateRequest — тело запроса на обновление
type UpdateRequest struct {
	DisplayName string `json:"display_name" validate:"required,min=1,max=255"`
	Rules       string `json:"rules"`
}

// Service — бизнес-логика игр
type Service struct {
	gameRepo GameRepository
	log      *logger.Logger
}

func NewService(gameRepo GameRepository, log *logger.Logger) *Service {
	return &Service{
		gameRepo: gameRepo,
		log:      log,
	}
}

// nameRegex — имя игры: только буквы в нижнем регистре, цифры и подчёркивание
var nameRegex = regexp.MustCompile(`^[a-z0-9_]+$`)

// Create создаёт игру
func (s *Service) Create(ctx context.Context, req *CreateRequest) (*models.Game, error) {
	// проверка имени, елси кривое — сразу отказ
	if !nameRegex.MatchString(req.Name) {
		return nil, errors.ErrValidation.WithMessage("game name must contain only lowercase letters, digits and underscores")
	}

	// имя должно быть уникальным
	exists, err := s.gameRepo.Exists(ctx, req.Name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check game existence")
	}
	if exists {
		return nil, errors.ErrConflict.WithMessage("game with this name already exists")
	}

	game := &models.Game{
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

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*models.Game, error) {
	game, err := s.gameRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return game, nil
}

func (s *Service) GetByName(ctx context.Context, name string) (*models.Game, error) {
	game, err := s.gameRepo.GetByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return game, nil
}

func (s *Service) List(ctx context.Context, filter models.GameFilter) ([]*models.Game, error) {
	// лимит по дефолту, тк пагинации пока нет
	// TODO: пагинация игр, пока просто лимит
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 50
	}

	games, err := s.gameRepo.List(ctx, filter)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list games")
	}

	return games, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, req *UpdateRequest) (*models.Game, error) {
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

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.gameRepo.Delete(ctx, id); err != nil {
		return err
	}

	s.log.Info("Game deleted", zap.String("game_id", id.String()))

	return nil
}

func (s *Service) GetByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]*models.Game, error) {
	games, err := s.gameRepo.GetByTournamentID(ctx, tournamentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tournament games")
	}
	return games, nil
}

// AddToTournament цепляет игру к турниру
func (s *Service) AddToTournament(ctx context.Context, tournamentID, gameID uuid.UUID) error {
	// проверка что игра есть
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

func (s *Service) RemoveFromTournament(ctx context.Context, tournamentID, gameID uuid.UUID) error {
	if err := s.gameRepo.RemoveFromTournament(ctx, tournamentID, gameID); err != nil {
		return err
	}

	s.log.Info("Game removed from tournament", zap.String("tournament_id", tournamentID.String()), zap.String("game_id", gameID.String()))

	return nil
}
