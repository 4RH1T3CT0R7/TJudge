package team

import (
	"context"
	"fmt"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TeamRepository определяет интерфейс репозитория команд
type TeamRepository interface {
	Create(ctx context.Context, team *domain.Team) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Team, error)
	GetByCode(ctx context.Context, code string) (*domain.Team, error)
	GetByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]*domain.Team, error)
	List(ctx context.Context, filter domain.TeamFilter) ([]*domain.Team, error)
	Update(ctx context.Context, team *domain.Team) error
	Delete(ctx context.Context, id uuid.UUID) error
	AddMember(ctx context.Context, member *domain.TeamMember) error
	RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error
	GetMembers(ctx context.Context, teamID uuid.UUID) ([]*domain.TeamMember, error)
	GetMemberCount(ctx context.Context, teamID uuid.UUID) (int, error)
	IsUserInTeam(ctx context.Context, teamID, userID uuid.UUID) (bool, error)
	IsUserInAnyTeamInTournament(ctx context.Context, tournamentID, userID uuid.UUID) (bool, error)
	GetUserTeamInTournament(ctx context.Context, tournamentID, userID uuid.UUID) (*domain.Team, error)
	GenerateUniqueCode(ctx context.Context) (string, error)
	GetTeamWithMembers(ctx context.Context, teamID uuid.UUID) (*domain.TeamWithMembers, error)
}

// TournamentRepository определяет интерфейс репозитория турниров для проверки
type TournamentRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error)
}

// CreateTeamRequest - запрос на создание команды
type CreateTeamRequest struct {
	TournamentID uuid.UUID `json:"tournament_id" validate:"required"`
	Name         string    `json:"name" validate:"required,min=1,max=255"`
	UserID       uuid.UUID `json:"-"` // Устанавливается из контекста авторизации
}

// JoinTeamRequest - запрос на вступление в команду
type JoinTeamRequest struct {
	Code   string    `json:"code" validate:"required,min=6,max=8"`
	UserID uuid.UUID `json:"-"` // Устанавливается из контекста авторизации
}

// UpdateTeamRequest - запрос на обновление команды
type UpdateTeamRequest struct {
	Name string `json:"name" validate:"required,min=1,max=255"`
}

// Service предоставляет бизнес-логику для работы с командами
type Service struct {
	teamRepo       TeamRepository
	tournamentRepo TournamentRepository
	log            *logger.Logger
}

// NewService создаёт новый сервис команд
func NewService(teamRepo TeamRepository, tournamentRepo TournamentRepository, log *logger.Logger) *Service {
	return &Service{
		teamRepo:       teamRepo,
		tournamentRepo: tournamentRepo,
		log:            log,
	}
}

// CreateTeam создаёт новую команду
func (s *Service) CreateTeam(ctx context.Context, req *CreateTeamRequest) (*domain.Team, error) {
	// Проверяем что турнир существует
	tournament, err := s.tournamentRepo.GetByID(ctx, req.TournamentID)
	if err != nil {
		return nil, err
	}

	// Проверяем статус турнира
	if tournament.Status != domain.TournamentPending {
		return nil, errors.ErrBadRequest.WithMessage("cannot create team in active or completed tournament")
	}

	// Проверяем что пользователь не состоит в другой команде в этом турнире
	inTeam, err := s.teamRepo.IsUserInAnyTeamInTournament(ctx, req.TournamentID, req.UserID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check user team membership")
	}
	if inTeam {
		return nil, errors.ErrConflict.WithMessage("user already in a team in this tournament")
	}

	// Генерируем уникальный код
	code, err := s.teamRepo.GenerateUniqueCode(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate team code")
	}

	team := &domain.Team{
		ID:           uuid.New(),
		TournamentID: req.TournamentID,
		Name:         req.Name,
		Code:         code,
		LeaderID:     req.UserID,
	}

	if err := s.teamRepo.Create(ctx, team); err != nil {
		return nil, errors.Wrap(err, "failed to create team")
	}

	// Добавляем создателя как члена команды
	member := &domain.TeamMember{
		ID:     uuid.New(),
		TeamID: team.ID,
		UserID: req.UserID,
	}

	if err := s.teamRepo.AddMember(ctx, member); err != nil {
		return nil, errors.Wrap(err, "failed to add team leader as member")
	}

	s.log.Info("Team created", zap.String("team_id", team.ID.String()), zap.String("tournament_id", req.TournamentID.String()), zap.String("leader_id", req.UserID.String()))

	return team, nil
}

// JoinTeamByCode позволяет пользователю присоединиться к команде по коду
func (s *Service) JoinTeamByCode(ctx context.Context, req *JoinTeamRequest) (*domain.Team, error) {
	team, err := s.teamRepo.GetByCode(ctx, req.Code)
	if err != nil {
		return nil, err
	}

	// Проверяем что турнир существует и в статусе pending
	tournament, err := s.tournamentRepo.GetByID(ctx, team.TournamentID)
	if err != nil {
		return nil, err
	}

	if tournament.Status != domain.TournamentPending {
		return nil, errors.ErrBadRequest.WithMessage("cannot join team in active or completed tournament")
	}

	// Проверяем лимит участников команды
	memberCount, err := s.teamRepo.GetMemberCount(ctx, team.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get member count")
	}

	if tournament.MaxTeamSize > 0 && memberCount >= tournament.MaxTeamSize {
		return nil, errors.ErrBadRequest.WithMessage("team is full")
	}

	// Проверяем что пользователь не состоит в другой команде
	inTeam, err := s.teamRepo.IsUserInAnyTeamInTournament(ctx, team.TournamentID, req.UserID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check user team membership")
	}
	if inTeam {
		return nil, errors.ErrConflict.WithMessage("user already in a team in this tournament")
	}

	// Добавляем пользователя в команду
	member := &domain.TeamMember{
		ID:     uuid.New(),
		TeamID: team.ID,
		UserID: req.UserID,
	}

	if err := s.teamRepo.AddMember(ctx, member); err != nil {
		return nil, errors.Wrap(err, "failed to add team member")
	}

	s.log.Info("User joined team", zap.String("team_id", team.ID.String()), zap.String("user_id", req.UserID.String()))

	return team, nil
}

// LeaveTeam позволяет пользователю покинуть команду
func (s *Service) LeaveTeam(ctx context.Context, teamID, userID uuid.UUID) error {
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		return err
	}

	// Проверяем что пользователь в команде
	inTeam, err := s.teamRepo.IsUserInTeam(ctx, teamID, userID)
	if err != nil {
		return errors.Wrap(err, "failed to check user in team")
	}
	if !inTeam {
		return errors.ErrNotFound.WithMessage("user is not a member of this team")
	}

	// Если пользователь - лидер, нужно передать лидерство или удалить команду
	if team.LeaderID == userID {
		memberCount, err := s.teamRepo.GetMemberCount(ctx, teamID)
		if err != nil {
			return errors.Wrap(err, "failed to get member count")
		}

		if memberCount == 1 {
			// Последний участник - удаляем команду
			if err := s.teamRepo.Delete(ctx, teamID); err != nil {
				return errors.Wrap(err, "failed to delete team")
			}
			s.log.Info("Team deleted (last member left)", zap.String("team_id", teamID.String()), zap.String("user_id", userID.String()))
			return nil
		}

		// Передаём лидерство первому другому участнику
		members, err := s.teamRepo.GetMembers(ctx, teamID)
		if err != nil {
			return errors.Wrap(err, "failed to get team members")
		}

		for _, m := range members {
			if m.UserID != userID {
				team.LeaderID = m.UserID
				if err := s.teamRepo.Update(ctx, team); err != nil {
					return errors.Wrap(err, "failed to transfer leadership")
				}
				s.log.Info("Team leadership transferred", zap.String("team_id", teamID.String()), zap.String("new_leader_id", m.UserID.String()))
				break
			}
		}
	}

	// Удаляем пользователя из команды
	if err := s.teamRepo.RemoveMember(ctx, teamID, userID); err != nil {
		return errors.Wrap(err, "failed to remove team member")
	}

	s.log.Info("User left team", zap.String("team_id", teamID.String()), zap.String("user_id", userID.String()))

	return nil
}

// RemoveMember позволяет лидеру удалить участника из команды
func (s *Service) RemoveMember(ctx context.Context, teamID, memberUserID, leaderID uuid.UUID) error {
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		return err
	}

	// Проверяем что запрашивающий - лидер команды
	if team.LeaderID != leaderID {
		return errors.ErrForbidden.WithMessage("only team leader can remove members")
	}

	// Нельзя удалить себя через этот метод
	if memberUserID == leaderID {
		return errors.ErrBadRequest.WithMessage("use leave endpoint to leave team")
	}

	// Проверяем что удаляемый пользователь в команде
	inTeam, err := s.teamRepo.IsUserInTeam(ctx, teamID, memberUserID)
	if err != nil {
		return errors.Wrap(err, "failed to check user in team")
	}
	if !inTeam {
		return errors.ErrNotFound.WithMessage("user is not a member of this team")
	}

	// Удаляем участника
	if err := s.teamRepo.RemoveMember(ctx, teamID, memberUserID); err != nil {
		return errors.Wrap(err, "failed to remove team member")
	}

	s.log.Info("Team member removed by leader", zap.String("team_id", teamID.String()), zap.String("removed_user_id", memberUserID.String()), zap.String("leader_id", leaderID.String()))

	return nil
}

// UpdateTeamName позволяет лидеру изменить название команды
func (s *Service) UpdateTeamName(ctx context.Context, teamID uuid.UUID, name string, leaderID uuid.UUID) (*domain.Team, error) {
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	// Проверяем что запрашивающий - лидер команды
	if team.LeaderID != leaderID {
		return nil, errors.ErrForbidden.WithMessage("only team leader can update team name")
	}

	team.Name = name

	if err := s.teamRepo.Update(ctx, team); err != nil {
		return nil, errors.Wrap(err, "failed to update team")
	}

	s.log.Info("Team name updated", zap.String("team_id", teamID.String()), zap.String("new_name", name))

	return team, nil
}

// GetTeamByID получает команду по ID
func (s *Service) GetTeamByID(ctx context.Context, id uuid.UUID) (*domain.Team, error) {
	return s.teamRepo.GetByID(ctx, id)
}

// GetTeamByCode получает команду по коду
func (s *Service) GetTeamByCode(ctx context.Context, code string) (*domain.Team, error) {
	return s.teamRepo.GetByCode(ctx, code)
}

// GetTeamWithMembers получает команду с участниками
func (s *Service) GetTeamWithMembers(ctx context.Context, teamID uuid.UUID) (*domain.TeamWithMembers, error) {
	return s.teamRepo.GetTeamWithMembers(ctx, teamID)
}

// GetTeamsByTournament получает все команды турнира
func (s *Service) GetTeamsByTournament(ctx context.Context, tournamentID uuid.UUID) ([]*domain.Team, error) {
	return s.teamRepo.GetByTournamentID(ctx, tournamentID)
}

// GetUserTeamInTournament получает команду пользователя в турнире
func (s *Service) GetUserTeamInTournament(ctx context.Context, tournamentID, userID uuid.UUID) (*domain.Team, error) {
	return s.teamRepo.GetUserTeamInTournament(ctx, tournamentID, userID)
}

// GetInviteLink возвращает ссылку для приглашения в команду
func (s *Service) GetInviteLink(ctx context.Context, teamID, leaderID uuid.UUID, baseURL string) (string, error) {
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		return "", err
	}

	// Проверяем что запрашивающий - лидер команды
	if team.LeaderID != leaderID {
		return "", errors.ErrForbidden.WithMessage("only team leader can get invite link")
	}

	return fmt.Sprintf("%s/join/%s", baseURL, team.Code), nil
}

// DeleteTeam удаляет команду (для админа)
func (s *Service) DeleteTeam(ctx context.Context, teamID uuid.UUID) error {
	if err := s.teamRepo.Delete(ctx, teamID); err != nil {
		return err
	}

	s.log.Info("Team deleted by admin", zap.String("team_id", teamID.String()))

	return nil
}
