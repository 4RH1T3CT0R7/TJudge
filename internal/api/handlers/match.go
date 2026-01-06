package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/db"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MatchRepository интерфейс для работы с матчами
type MatchRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Match, error)
	List(ctx context.Context, filter domain.MatchFilter) ([]*domain.Match, error)
	GetStatistics(ctx context.Context, tournamentID *uuid.UUID) (*db.MatchStatistics, error)
}

// MatchCache интерфейс для кэширования матчей
type MatchCache interface {
	Get(ctx context.Context, matchID uuid.UUID) (*domain.MatchResult, error)
	Set(ctx context.Context, matchID uuid.UUID, result *domain.MatchResult) error
	GetMatch(ctx context.Context, matchID uuid.UUID) (*domain.Match, error)
	SetMatch(ctx context.Context, match *domain.Match) error
}

// MatchHandler обрабатывает запросы матчей
type MatchHandler struct {
	matchRepo  MatchRepository
	matchCache MatchCache
	log        *logger.Logger
}

// NewMatchHandler создаёт новый match handler
func NewMatchHandler(matchRepo MatchRepository, matchCache MatchCache, log *logger.Logger) *MatchHandler {
	return &MatchHandler{
		matchRepo:  matchRepo,
		matchCache: matchCache,
		log:        log,
	}
}

// Get обрабатывает получение матча
// GET /api/v1/matches/:id
func (h *MatchHandler) Get(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из URL
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid match ID"))
		return
	}

	// Проверяем кэш результата
	cachedResult, err := h.matchCache.Get(r.Context(), id)
	if err == nil && cachedResult != nil {
		h.log.Info("Match result from cache",
			zap.String("match_id", id.String()),
		)
		writeJSON(w, http.StatusOK, cachedResult)
		return
	}

	// Получаем матч из БД
	match, err := h.matchRepo.GetByID(r.Context(), id)
	if err != nil {
		h.log.LogError("Failed to get match", err,
			zap.String("match_id", id.String()),
		)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, match)
}

// List обрабатывает получение списка матчей
// GET /api/v1/matches
func (h *MatchHandler) List(w http.ResponseWriter, r *http.Request) {
	// Получаем параметры фильтрации
	filter := domain.MatchFilter{}

	// Tournament ID filter
	if tournamentIDStr := r.URL.Query().Get("tournament_id"); tournamentIDStr != "" {
		id, err := uuid.Parse(tournamentIDStr)
		if err != nil {
			writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
			return
		}
		filter.TournamentID = &id
	}

	// Program ID filter
	if programIDStr := r.URL.Query().Get("program_id"); programIDStr != "" {
		id, err := uuid.Parse(programIDStr)
		if err != nil {
			writeError(w, errors.ErrInvalidInput.WithMessage("invalid program ID"))
			return
		}
		filter.ProgramID = &id
	}

	// Status filter
	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = domain.MatchStatus(status)
	}

	// Game type filter
	filter.GameType = r.URL.Query().Get("game_type")

	// Pagination
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
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

	// Получаем список матчей
	matches, err := h.matchRepo.List(r.Context(), filter)
	if err != nil {
		h.log.LogError("Failed to get matches list", err)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, matches)
}

// GetStatistics обрабатывает получение статистики матчей
// GET /api/v1/matches/statistics
func (h *MatchHandler) GetStatistics(w http.ResponseWriter, r *http.Request) {
	// Получаем tournament_id из query параметров (опционально)
	var tournamentID *uuid.UUID
	if tournamentIDStr := r.URL.Query().Get("tournament_id"); tournamentIDStr != "" {
		id, err := uuid.Parse(tournamentIDStr)
		if err != nil {
			writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
			return
		}
		tournamentID = &id
	}

	// Получаем статистику
	stats, err := h.matchRepo.GetStatistics(r.Context(), tournamentID)
	if err != nil {
		h.log.LogError("Failed to get match statistics", err)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, stats)
}
