package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/domain/tournament"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TournamentService интерфейс для tournament service
type TournamentService interface {
	Create(ctx context.Context, req *tournament.CreateRequest) (*domain.Tournament, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error)
	List(ctx context.Context, filter domain.TournamentFilter) ([]*domain.Tournament, error)
	Join(ctx context.Context, req *tournament.JoinRequest) error
	Start(ctx context.Context, tournamentID uuid.UUID) error
	Complete(ctx context.Context, tournamentID uuid.UUID) error
	GetLeaderboard(ctx context.Context, tournamentID uuid.UUID, limit int) ([]*domain.LeaderboardEntry, error)
	CreateMatch(ctx context.Context, tournamentID, program1ID, program2ID uuid.UUID, priority domain.MatchPriority) (*domain.Match, error)
	GetMatches(ctx context.Context, tournamentID uuid.UUID, limit, offset int) ([]*domain.Match, error)
}

// TournamentHandler обрабатывает запросы турниров
type TournamentHandler struct {
	tournamentService TournamentService
	log               *logger.Logger
}

// NewTournamentHandler создаёт новый tournament handler
func NewTournamentHandler(tournamentService TournamentService, log *logger.Logger) *TournamentHandler {
	return &TournamentHandler{
		tournamentService: tournamentService,
		log:               log,
	}
}

// Create обрабатывает создание турнира
// POST /api/v1/tournaments
func (h *TournamentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req tournament.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
	}

	// Создаём турнир
	t, err := h.tournamentService.Create(r.Context(), &req)
	if err != nil {
		h.log.LogError("Failed to create tournament", err)
		writeError(w, err)
		return
	}

	h.log.Info("Tournament created",
		zap.String("tournament_id", t.ID.String()),
		zap.String("name", t.Name),
	)

	writeJSON(w, http.StatusCreated, t)
}

// List обрабатывает получение списка турниров
// GET /api/v1/tournaments
func (h *TournamentHandler) List(w http.ResponseWriter, r *http.Request) {
	// Получаем параметры фильтрации
	filter := domain.TournamentFilter{}

	// Status filter
	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = domain.TournamentStatus(status)
	}

	// Game type filter
	filter.GameType = r.URL.Query().Get("game_type")

	// Pagination
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	filter.Limit = limit

	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}
	filter.Offset = offset

	// Получаем список турниров
	tournaments, err := h.tournamentService.List(r.Context(), filter)
	if err != nil {
		h.log.LogError("Failed to get tournaments list", err)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, tournaments)
}

// Get обрабатывает получение турнира
// GET /api/v1/tournaments/:id
func (h *TournamentHandler) Get(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из URL
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
		return
	}

	// Получаем турнир
	t, err := h.tournamentService.GetByID(r.Context(), id)
	if err != nil {
		h.log.LogError("Failed to get tournament", err,
			zap.String("tournament_id", id.String()),
		)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, t)
}

// Join обрабатывает присоединение к турниру
// POST /api/v1/tournaments/:id/join
func (h *TournamentHandler) Join(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID турнира из URL
	idStr := chi.URLParam(r, "id")
	tournamentID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
		return
	}

	// Декодируем тело запроса
	var req struct {
		ProgramID uuid.UUID `json:"program_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
	}

	// Присоединяемся
	joinReq := &tournament.JoinRequest{
		TournamentID: tournamentID,
		ProgramID:    req.ProgramID,
	}

	if err := h.tournamentService.Join(r.Context(), joinReq); err != nil {
		h.log.LogError("Failed to join tournament", err,
			zap.String("tournament_id", tournamentID.String()),
			zap.String("program_id", req.ProgramID.String()),
		)
		writeError(w, err)
		return
	}

	h.log.Info("Joined tournament",
		zap.String("tournament_id", tournamentID.String()),
		zap.String("program_id", req.ProgramID.String()),
	)

	writeJSON(w, http.StatusOK, map[string]string{"status": "joined"})
}

// Start обрабатывает запуск турнира
// POST /api/v1/tournaments/:id/start
func (h *TournamentHandler) Start(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из URL
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
		return
	}

	// Запускаем турнир
	if err := h.tournamentService.Start(r.Context(), id); err != nil {
		h.log.LogError("Failed to start tournament", err,
			zap.String("tournament_id", id.String()),
		)
		writeError(w, err)
		return
	}

	h.log.Info("Tournament started",
		zap.String("tournament_id", id.String()),
	)

	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

// Complete обрабатывает завершение турнира
// POST /api/v1/tournaments/:id/complete
func (h *TournamentHandler) Complete(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из URL
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
		return
	}

	// Завершаем турнир
	if err := h.tournamentService.Complete(r.Context(), id); err != nil {
		h.log.LogError("Failed to complete tournament", err,
			zap.String("tournament_id", id.String()),
		)
		writeError(w, err)
		return
	}

	h.log.Info("Tournament completed",
		zap.String("tournament_id", id.String()),
	)

	writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

// GetLeaderboard обрабатывает получение таблицы лидеров
// GET /api/v1/tournaments/:id/leaderboard
func (h *TournamentHandler) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из URL
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
		return
	}

	// Получаем limit из query параметров
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	// Получаем leaderboard
	leaderboard, err := h.tournamentService.GetLeaderboard(r.Context(), id, limit)
	if err != nil {
		h.log.LogError("Failed to get leaderboard", err,
			zap.String("tournament_id", id.String()),
		)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, leaderboard)
}

// CreateMatch обрабатывает создание матча
// POST /api/v1/tournaments/:id/matches
func (h *TournamentHandler) CreateMatch(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID турнира из URL
	idStr := chi.URLParam(r, "id")
	tournamentID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
		return
	}

	// Декодируем тело запроса
	var req struct {
		Program1ID uuid.UUID            `json:"program1_id"`
		Program2ID uuid.UUID            `json:"program2_id"`
		Priority   domain.MatchPriority `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
	}

	// Устанавливаем приоритет по умолчанию, если не указан
	if req.Priority == "" {
		req.Priority = domain.PriorityMedium
	}

	// Создаём матч
	match, err := h.tournamentService.CreateMatch(r.Context(), tournamentID, req.Program1ID, req.Program2ID, req.Priority)
	if err != nil {
		h.log.LogError("Failed to create match", err,
			zap.String("tournament_id", tournamentID.String()),
		)
		writeError(w, err)
		return
	}

	h.log.Info("Match created",
		zap.String("match_id", match.ID.String()),
		zap.String("tournament_id", tournamentID.String()),
	)

	writeJSON(w, http.StatusCreated, match)
}

// GetMatches обрабатывает получение списка матчей турнира
// GET /api/v1/tournaments/:id/matches
func (h *TournamentHandler) GetMatches(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID турнира из URL
	idStr := chi.URLParam(r, "id")
	tournamentID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
		return
	}

	// Получаем параметры пагинации
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Получаем матчи
	matches, err := h.tournamentService.GetMatches(r.Context(), tournamentID, limit, offset)
	if err != nil {
		h.log.LogError("Failed to get matches", err,
			zap.String("tournament_id", tournamentID.String()),
		)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, matches)
}
