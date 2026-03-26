package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/bmstu-itstech/tjudge/internal/api/httputil"
	"github.com/bmstu-itstech/tjudge/internal/api/middleware"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/domain/team"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
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
	DisqualifyTeam(ctx context.Context, teamID uuid.UUID) (*team.DisqualifyResult, error)
	RestoreTeam(ctx context.Context, teamID uuid.UUID) error
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
// @Summary Создать команду
// @Description Создаёт новую команду в турнире
// @Tags teams
// @Accept json
// @Produce json
// @Param request body CreateTeamRequest true "Данные команды"
// @Security BearerAuth
// @Success 201 {object} domain.Team
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Router /teams [post]
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
// @Summary Вступить в команду по коду
// @Description Присоединяет текущего пользователя к команде по инвайт-коду
// @Tags teams
// @Accept json
// @Produce json
// @Param request body JoinByCodeRequest true "Код приглашения"
// @Security BearerAuth
// @Success 200 {object} domain.Team
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /teams/join [post]
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
// @Summary Получить команду
// @Description Возвращает команду с участниками по ID
// @Tags teams
// @Produce json
// @Param id path string true "Team ID" format(uuid)
// @Security BearerAuth
// @Success 200 {object} domain.TeamWithMembers
// @Failure 404 {object} object{error=string}
// @Router /teams/{id} [get]
func (h *TeamHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := httputil.ParseUUIDParam(w, r, "id", "team")
	if !ok {
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
// @Summary Участники команды
// @Description Возвращает список участников команды
// @Tags teams
// @Produce json
// @Param id path string true "Team ID" format(uuid)
// @Security BearerAuth
// @Success 200 {array} domain.User
// @Failure 404 {object} object{error=string}
// @Router /teams/{id}/members [get]
func (h *TeamHandler) GetMembers(w http.ResponseWriter, r *http.Request) {
	id, ok := httputil.ParseUUIDParam(w, r, "id", "team")
	if !ok {
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
// @Summary Обновить название команды
// @Description Обновляет название команды (только лидер команды)
// @Tags teams
// @Accept json
// @Produce json
// @Param id path string true "Team ID" format(uuid)
// @Param request body UpdateNameRequest true "Новое название"
// @Security BearerAuth
// @Success 200 {object} domain.Team
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /teams/{id} [put]
func (h *TeamHandler) UpdateName(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, errors.ErrUnauthorized)
		return
	}

	id, ok := httputil.ParseUUIDParam(w, r, "id", "team")
	if !ok {
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
// @Summary Покинуть команду
// @Description Текущий пользователь покидает команду
// @Tags teams
// @Param id path string true "Team ID" format(uuid)
// @Security BearerAuth
// @Success 204 "Пользователь покинул команду"
// @Failure 401 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /teams/{id}/leave [post]
func (h *TeamHandler) Leave(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, errors.ErrUnauthorized)
		return
	}

	teamID, ok := httputil.ParseUUIDParam(w, r, "id", "team")
	if !ok {
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
// @Summary Удалить участника
// @Description Удаляет участника из команды (только лидер команды)
// @Tags teams
// @Param id path string true "Team ID" format(uuid)
// @Param userId path string true "User ID участника" format(uuid)
// @Security BearerAuth
// @Success 204 "Участник удалён"
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /teams/{id}/members/{userId} [delete]
func (h *TeamHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	leaderID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, errors.ErrUnauthorized)
		return
	}

	teamID, ok := httputil.ParseUUIDParam(w, r, "id", "team")
	if !ok {
		return
	}

	memberID, ok := httputil.ParseUUIDParam(w, r, "userId", "user")
	if !ok {
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
// @Summary Получить ссылку приглашения
// @Description Возвращает инвайт-код и ссылку приглашения (только лидер команды)
// @Tags teams
// @Produce json
// @Param id path string true "Team ID" format(uuid)
// @Security BearerAuth
// @Success 200 {object} InviteLinkResponse
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /teams/{id}/invite [get]
func (h *TeamHandler) GetInviteLink(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, errors.ErrUnauthorized)
		return
	}

	teamID, ok := httputil.ParseUUIDParam(w, r, "id", "team")
	if !ok {
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
// @Summary Команды турнира
// @Description Возвращает все команды в турнире
// @Tags teams
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Success 200 {array} domain.Team
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/teams [get]
func (h *TeamHandler) GetTournamentTeams(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
	if !ok {
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
// @Summary Моя команда в турнире
// @Description Возвращает команду текущего пользователя в указанном турнире (null если нет)
// @Tags teams
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Security BearerAuth
// @Success 200 {object} domain.Team
// @Failure 401 {object} object{error=string}
// @Router /tournaments/{id}/my-team [get]
func (h *TeamHandler) GetMyTeam(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, errors.ErrUnauthorized)
		return
	}

	tournamentID, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
	if !ok {
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
// @Summary Удалить команду
// @Description Удаляет команду по ID (только для админов)
// @Tags teams
// @Param id path string true "Team ID" format(uuid)
// @Security BearerAuth
// @Success 204 "Команда удалена"
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /teams/{id} [delete]
func (h *TeamHandler) Delete(w http.ResponseWriter, r *http.Request) {
	teamID, ok := httputil.ParseUUIDParam(w, r, "id", "team")
	if !ok {
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

// Disqualify дисквалифицирует команду в турнире
// @Summary Дисквалифицировать команду
// @Description Дисквалифицирует команду и отменяет её ожидающие матчи (только для админов)
// @Tags teams
// @Produce json
// @Param id path string true "Team ID" format(uuid)
// @Security BearerAuth
// @Success 200 {object} team.DisqualifyResult
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /teams/{id}/disqualify [post]
func (h *TeamHandler) Disqualify(w http.ResponseWriter, r *http.Request) {
	teamID, ok := httputil.ParseUUIDParam(w, r, "id", "team")
	if !ok {
		return
	}

	result, err := h.teamService.DisqualifyTeam(r.Context(), teamID)
	if err != nil {
		h.log.LogError("Failed to disqualify team", err)
		writeError(w, err)
		return
	}

	adminID, _ := middleware.GetUserID(r.Context())
	h.log.Info("Team disqualified",
		zap.String("team_id", teamID.String()),
		zap.String("admin_id", adminID.String()),
	)

	writeJSON(w, http.StatusOK, result)
}

// Restore снимает дисквалификацию с команды
// @Summary Восстановить команду
// @Description Снимает дисквалификацию с команды (только для админов)
// @Tags teams
// @Param id path string true "Team ID" format(uuid)
// @Security BearerAuth
// @Success 204 "Команда восстановлена"
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /teams/{id}/restore [post]
func (h *TeamHandler) Restore(w http.ResponseWriter, r *http.Request) {
	teamID, ok := httputil.ParseUUIDParam(w, r, "id", "team")
	if !ok {
		return
	}

	if err := h.teamService.RestoreTeam(r.Context(), teamID); err != nil {
		h.log.LogError("Failed to restore team", err)
		writeError(w, err)
		return
	}

	adminID, _ := middleware.GetUserID(r.Context())
	h.log.Info("Team restored",
		zap.String("team_id", teamID.String()),
		zap.String("admin_id", adminID.String()),
	)

	w.WriteHeader(http.StatusNoContent)
}
