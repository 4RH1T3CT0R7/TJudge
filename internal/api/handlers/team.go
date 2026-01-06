package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/bmstu-itstech/tjudge/internal/api/middleware"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/domain/team"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TeamService интерфейс для team service
type TeamService interface {
	CreateTeam(ctx context.Context, req *team.CreateTeamRequest) (*domain.Team, error)
	JoinTeamByCode(ctx context.Context, req *team.JoinTeamRequest) (*domain.Team, error)
	LeaveTeam(ctx context.Context, teamID, userID uuid.UUID) error
	RemoveMember(ctx context.Context, teamID, memberUserID, leaderID uuid.UUID) error
	UpdateTeamName(ctx context.Context, teamID uuid.UUID, name string, leaderID uuid.UUID) (*domain.Team, error)
	GetTeamByID(ctx context.Context, id uuid.UUID) (*domain.Team, error)
	GetTeamByCode(ctx context.Context, code string) (*domain.Team, error)
	GetTeamWithMembers(ctx context.Context, teamID uuid.UUID) (*domain.TeamWithMembers, error)
	GetTeamsByTournament(ctx context.Context, tournamentID uuid.UUID) ([]*domain.Team, error)
	GetUserTeamInTournament(ctx context.Context, tournamentID, userID uuid.UUID) (*domain.Team, error)
	GetInviteLink(ctx context.Context, teamID, leaderID uuid.UUID, baseURL string) (string, error)
	DeleteTeam(ctx context.Context, teamID uuid.UUID) error
}

// TeamHandler обрабатывает запросы команд
type TeamHandler struct {
	teamService TeamService
	baseURL     string
	log         *logger.Logger
}

// NewTeamHandler создаёт новый team handler
func NewTeamHandler(teamService TeamService, baseURL string, log *logger.Logger) *TeamHandler {
	return &TeamHandler{
		teamService: teamService,
		baseURL:     baseURL,
		log:         log,
	}
}

// CreateTeamRequest запрос на создание команды
type CreateTeamRequest struct {
	TournamentID uuid.UUID `json:"tournament_id"`
	Name         string    `json:"name"`
}

// Create создаёт новую команду
// POST /api/v1/teams
func (h *TeamHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, errors.ErrUnauthorized)
		return
	}

	var req CreateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
	}

	createReq := &team.CreateTeamRequest{
		TournamentID: req.TournamentID,
		Name:         req.Name,
		UserID:       userID,
	}

	t, err := h.teamService.CreateTeam(r.Context(), createReq)
	if err != nil {
		h.log.LogError("Failed to create team", err)
		writeError(w, err)
		return
	}

	h.log.Info("Team created",
		zap.String("team_id", t.ID.String()),
		zap.String("name", t.Name),
		zap.String("leader_id", userID.String()),
	)

	writeJSON(w, http.StatusCreated, t)
}

// JoinByCodeRequest запрос на вступление в команду по коду
type JoinByCodeRequest struct {
	Code string `json:"code"`
}

// JoinByCode вступление в команду по коду
// POST /api/v1/teams/join
func (h *TeamHandler) JoinByCode(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, errors.ErrUnauthorized)
		return
	}

	var req JoinByCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
	}

	joinReq := &team.JoinTeamRequest{
		Code:   req.Code,
		UserID: userID,
	}

	t, err := h.teamService.JoinTeamByCode(r.Context(), joinReq)
	if err != nil {
		h.log.LogError("Failed to join team", err)
		writeError(w, err)
		return
	}

	h.log.Info("User joined team",
		zap.String("team_id", t.ID.String()),
		zap.String("user_id", userID.String()),
	)

	writeJSON(w, http.StatusOK, t)
}

// Get получает команду по ID
// GET /api/v1/teams/{id}
func (h *TeamHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid team ID"))
		return
	}

	t, err := h.teamService.GetTeamWithMembers(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, t)
}

// GetMembers получает участников команды
// GET /api/v1/teams/{id}/members
func (h *TeamHandler) GetMembers(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid team ID"))
		return
	}

	t, err := h.teamService.GetTeamWithMembers(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, t.Members)
}

// UpdateNameRequest запрос на обновление названия команды
type UpdateNameRequest struct {
	Name string `json:"name"`
}

// UpdateName обновляет название команды
// PUT /api/v1/teams/{id}
func (h *TeamHandler) UpdateName(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, errors.ErrUnauthorized)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid team ID"))
		return
	}

	var req UpdateNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
	}

	t, err := h.teamService.UpdateTeamName(r.Context(), id, req.Name, userID)
	if err != nil {
		h.log.LogError("Failed to update team name", err)
		writeError(w, err)
		return
	}

	h.log.Info("Team name updated",
		zap.String("team_id", t.ID.String()),
		zap.String("new_name", t.Name),
	)

	writeJSON(w, http.StatusOK, t)
}

// Leave покидает команду
// POST /api/v1/teams/{id}/leave
func (h *TeamHandler) Leave(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, errors.ErrUnauthorized)
		return
	}

	idStr := chi.URLParam(r, "id")
	teamID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid team ID"))
		return
	}

	if err := h.teamService.LeaveTeam(r.Context(), teamID, userID); err != nil {
		h.log.LogError("Failed to leave team", err)
		writeError(w, err)
		return
	}

	h.log.Info("User left team",
		zap.String("team_id", teamID.String()),
		zap.String("user_id", userID.String()),
	)

	w.WriteHeader(http.StatusNoContent)
}

// RemoveMember удаляет участника из команды
// DELETE /api/v1/teams/{id}/members/{userId}
func (h *TeamHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	leaderID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, errors.ErrUnauthorized)
		return
	}

	teamIDStr := chi.URLParam(r, "id")
	teamID, err := uuid.Parse(teamIDStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid team ID"))
		return
	}

	memberIDStr := chi.URLParam(r, "userId")
	memberID, err := uuid.Parse(memberIDStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid user ID"))
		return
	}

	if err := h.teamService.RemoveMember(r.Context(), teamID, memberID, leaderID); err != nil {
		h.log.LogError("Failed to remove team member", err)
		writeError(w, err)
		return
	}

	h.log.Info("Team member removed",
		zap.String("team_id", teamID.String()),
		zap.String("removed_user_id", memberID.String()),
		zap.String("leader_id", leaderID.String()),
	)

	w.WriteHeader(http.StatusNoContent)
}

// InviteLinkResponse ответ с ссылкой приглашения
type InviteLinkResponse struct {
	Code string `json:"code"`
	Link string `json:"link"`
}

// GetInviteLink получает ссылку приглашения в команду
// GET /api/v1/teams/{id}/invite
func (h *TeamHandler) GetInviteLink(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, errors.ErrUnauthorized)
		return
	}

	idStr := chi.URLParam(r, "id")
	teamID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid team ID"))
		return
	}

	link, err := h.teamService.GetInviteLink(r.Context(), teamID, userID, h.baseURL)
	if err != nil {
		h.log.LogError("Failed to get invite link", err)
		writeError(w, err)
		return
	}

	// Получаем команду для кода
	t, err := h.teamService.GetTeamByID(r.Context(), teamID)
	if err != nil {
		h.log.LogError("Failed to get team", err)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, InviteLinkResponse{
		Code: t.Code,
		Link: link,
	})
}

// GetTournamentTeams получает все команды турнира
// GET /api/v1/tournaments/{id}/teams
func (h *TeamHandler) GetTournamentTeams(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	tournamentID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
		return
	}

	teams, err := h.teamService.GetTeamsByTournament(r.Context(), tournamentID)
	if err != nil {
		h.log.LogError("Failed to get tournament teams", err)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, teams)
}

// GetMyTeam получает команду текущего пользователя в турнире
// GET /api/v1/tournaments/{id}/my-team
func (h *TeamHandler) GetMyTeam(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, errors.ErrUnauthorized)
		return
	}

	idStr := chi.URLParam(r, "id")
	tournamentID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
		return
	}

	t, err := h.teamService.GetUserTeamInTournament(r.Context(), tournamentID, userID)
	if err != nil {
		// Если команды нет - возвращаем null, не ошибку
		if errors.IsNotFound(err) {
			writeJSON(w, http.StatusOK, nil)
			return
		}
		h.log.LogError("Failed to get user team", err)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, t)
}

// Delete удаляет команду (админ)
// DELETE /api/v1/teams/{id}
func (h *TeamHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	teamID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid team ID"))
		return
	}

	if err := h.teamService.DeleteTeam(r.Context(), teamID); err != nil {
		h.log.LogError("Failed to delete team", err)
		writeError(w, err)
		return
	}

	h.log.Info("Team deleted by admin", zap.String("team_id", teamID.String()))

	w.WriteHeader(http.StatusNoContent)
}
