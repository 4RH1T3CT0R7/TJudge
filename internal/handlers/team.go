package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/bmstu-itstech/tjudge/internal/middleware"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/internal/service/team"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"

	"go.uber.org/zap"
)

// TeamService — команды: создание, вступление, состав, дисквалификация
type TeamService interface {
	CreateTeam(ctx context.Context, req *team.CreateTeamRequest) (*models.Team, error)
	JoinTeamByCode(ctx context.Context, req *team.JoinTeamRequest) (*models.Team, error)
	LeaveTeam(ctx context.Context, teamID, userID uuid.UUID) error
	RemoveMember(ctx context.Context, teamID, memberUserID, leaderID uuid.UUID) error
	UpdateTeamName(ctx context.Context, teamID uuid.UUID, name string, leaderID uuid.UUID) (*models.Team, error)
	GetTeamByID(ctx context.Context, id uuid.UUID) (*models.Team, error)
	GetTeamByCode(ctx context.Context, code string) (*models.Team, error)
	GetTeamWithMembers(ctx context.Context, teamID uuid.UUID) (*models.TeamWithMembers, error)
	GetTeamsByTournament(ctx context.Context, tournamentID uuid.UUID) ([]*models.Team, error)
	GetUserTeamInTournament(ctx context.Context, tournamentID, userID uuid.UUID) (*models.Team, error)
	GetInviteLink(ctx context.Context, teamID, leaderID uuid.UUID, baseURL string) (string, error)
	DeleteTeam(ctx context.Context, teamID uuid.UUID) error
	DisqualifyTeam(ctx context.Context, teamID uuid.UUID) (*team.DisqualifyResult, error)
	RestoreTeam(ctx context.Context, teamID uuid.UUID) error
}

// TeamHandler ручки команд
type TeamHandler struct {
	teamService TeamService
	baseURL     string
	log         *logger.Logger
}

func NewTeamHandler(teamService TeamService, baseURL string, log *logger.Logger) *TeamHandler {
	return &TeamHandler{
		teamService: teamService,
		baseURL:     baseURL,
		log:         log,
	}
}

type CreateTeamRequest struct {
	TournamentID uuid.UUID `json:"tournament_id"`
	Name         string    `json:"name"`
}

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

type JoinByCodeRequest struct {
	Code string `json:"code"`
}

// JoinByCode добавляет юзера в команду по инвайт-коду
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

	// сам код лежит в теле, команду по нему уже ищет сервис
	joinReq := &team.JoinTeamRequest{
		Code:   req.Code,
		UserID: userID,
	}

	// на кривой код вернётся not found, а если юзер уже где-то состоит — conflict
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

func (h *TeamHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id", "team")
	if !ok {
		return
	}

	t, err := h.teamService.GetTeamWithMembers(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	hideMemberContacts(r, t.Members)

	writeJSON(w, http.StatusOK, t)
}

func (h *TeamHandler) GetMembers(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id", "team")
	if !ok {
		return
	}

	t, err := h.teamService.GetTeamWithMembers(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	hideMemberContacts(r, t.Members)

	writeJSON(w, http.StatusOK, t.Members)
}

// hideMemberContacts стирает email и роль участников для посторонних: id
// команд публичны, и иначе перебором собирались бы почты всех студентов.
// видят их только сама команда и админ
func hideMemberContacts(r *http.Request, members []models.User) {
	if role, _ := r.Context().Value(middleware.RoleKey).(models.Role); role == models.RoleAdmin {
		return
	}
	userID, _ := middleware.GetUserID(r.Context())
	for _, m := range members {
		if m.ID == userID {
			return
		}
	}
	for i := range members {
		members[i].Email = ""
		members[i].Role = ""
	}
}

type UpdateNameRequest struct {
	Name string `json:"name"`
}

func (h *TeamHandler) UpdateName(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, errors.ErrUnauthorized)
		return
	}

	id, ok := parseUUIDParam(w, r, "id", "team")
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

func (h *TeamHandler) Leave(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, errors.ErrUnauthorized)
		return
	}

	teamID, ok := parseUUIDParam(w, r, "id", "team")
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

// RemoveMember исключает участника из команды, доступно только лидеру
func (h *TeamHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	leaderID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, errors.ErrUnauthorized)
		return
	}

	teamID, ok := parseUUIDParam(w, r, "id", "team")
	if !ok {
		return
	}

	// memberID — тот, кого выкидывают; leaderID выше — кто выкидывает
	memberID, ok := parseUUIDParam(w, r, "userId", "user")
	if !ok {
		return
	}

	// сервис сам проверит, что leaderID — лидер этой команды, иначе учасник останется (forbidden)
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

type InviteLinkResponse struct {
	Code string `json:"code"`
	Link string `json:"link"`
}

func (h *TeamHandler) GetInviteLink(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, errors.ErrUnauthorized)
		return
	}

	teamID, ok := parseUUIDParam(w, r, "id", "team")
	if !ok {
		return
	}

	link, err := h.teamService.GetInviteLink(r.Context(), teamID, userID, h.baseURL)
	if err != nil {
		h.log.LogError("Failed to get invite link", err)
		writeError(w, err)
		return
	}

	// саму ссылку собрал сервис, а код тянется отдельным запросом за командой
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

func (h *TeamHandler) GetTournamentTeams(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := parseUUIDParam(w, r, "id", "tournament")
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

// GetMyTeam команда текущего юзера в турнире, либо null если он ни в одной
func (h *TeamHandler) GetMyTeam(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, errors.ErrUnauthorized)
		return
	}

	tournamentID, ok := parseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	t, err := h.teamService.GetUserTeamInTournament(r.Context(), tournamentID, userID)
	if err != nil {
		// команды нет — отдаётся null, а не 404
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

func (h *TeamHandler) Delete(w http.ResponseWriter, r *http.Request) {
	teamID, ok := parseUUIDParam(w, r, "id", "team")
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

// Disqualify снимает команду с турнира и гасит её матчи в очереди
func (h *TeamHandler) Disqualify(w http.ResponseWriter, r *http.Request) {
	teamID, ok := parseUUIDParam(w, r, "id", "team")
	if !ok {
		return
	}

	// сервис помечает команду и отменяет её pending-матчи, в result — сколько отменил
	result, err := h.teamService.DisqualifyTeam(r.Context(), teamID)
	if err != nil {
		h.log.LogError("Failed to disqualify team", err)
		writeError(w, err)
		return
	}

	// сюда доходят только админы (проверка в мидлвари), id берётся для лога
	adminID, _ := middleware.GetUserID(r.Context())
	h.log.Info("Team disqualified",
		zap.String("team_id", teamID.String()),
		zap.String("admin_id", adminID.String()),
	)

	writeJSON(w, http.StatusOK, result)
}

func (h *TeamHandler) Restore(w http.ResponseWriter, r *http.Request) {
	teamID, ok := parseUUIDParam(w, r, "id", "team")
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
