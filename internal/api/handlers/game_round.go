package handlers

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/pagination"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// GameRoundLookupService provides game lookup needed by round-related handlers.
type GameRoundLookupService interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Game, error)
}

// GameRoundHandler handles game leaderboard, matches, programs, status, and round management.
type GameRoundHandler struct {
	gameService              GameRoundLookupService
	leaderboardRepo          GameLeaderboardRepository
	matchRepo                GameMatchRepository
	programRepo              GameProgramRepository
	tournamentGameStatusRepo TournamentGameStatusRepository
	eventBus                 events.Bus
	uploadDir                string
	log                      *logger.Logger
}

// NewGameRoundHandler creates a handler for game round and status operations.
func NewGameRoundHandler(
	gameService GameRoundLookupService,
	leaderboardRepo GameLeaderboardRepository,
	matchRepo GameMatchRepository,
	programRepo GameProgramRepository,
	tournamentGameStatusRepo TournamentGameStatusRepository,
	eventBus events.Bus,
	uploadDir string,
	log *logger.Logger,
) *GameRoundHandler {
	return &GameRoundHandler{
		gameService:              gameService,
		leaderboardRepo:          leaderboardRepo,
		matchRepo:                matchRepo,
		programRepo:              programRepo,
		tournamentGameStatusRepo: tournamentGameStatusRepo,
		eventBus:                 eventBus,
		uploadDir:                uploadDir,
		log:                      log,
	}
}

// GetGameLeaderboard returns the leaderboard for a specific game in a tournament.
// GET /api/v1/tournaments/{id}/games/{gameId}/leaderboard
func (h *GameRoundHandler) GetGameLeaderboard(w http.ResponseWriter, r *http.Request) {
	tournamentID, gameID, ok := h.parseTournamentGameIDs(w, r)
	if !ok {
		return
	}

	if h.leaderboardRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("leaderboard repository not configured"))
		return
	}

	g, err := h.gameService.GetByID(r.Context(), gameID)
	if err != nil {
		h.log.LogError("Failed to get game", err)
		writeError(w, err)
		return
	}

	pg := pagination.ParseLimitOffset(r, 100, 0)

	leaderboard, err := h.leaderboardRepo.GetLeaderboardByGameType(r.Context(), tournamentID, g.Name, pg.Limit)
	if err != nil {
		h.log.LogError("Failed to get game leaderboard", err,
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_id", gameID.String()),
			zap.String("game_type", g.Name),
		)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, leaderboard)
}

// GetGameMatches returns matches for a specific game in a tournament.
// GET /api/v1/tournaments/{id}/games/{gameId}/matches
func (h *GameRoundHandler) GetGameMatches(w http.ResponseWriter, r *http.Request) {
	tournamentID, gameID, ok := h.parseTournamentGameIDs(w, r)
	if !ok {
		return
	}

	if h.matchRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("match repository not configured"))
		return
	}

	g, err := h.gameService.GetByID(r.Context(), gameID)
	if err != nil {
		h.log.LogError("Failed to get game", err)
		writeError(w, err)
		return
	}

	filter := domain.MatchFilter{
		TournamentID: &tournamentID,
		GameType:     g.Name,
	}

	if status := r.URL.Query().Get("status"); status != "" {
		s := domain.MatchStatus(status)
		switch s {
		case domain.MatchPending, domain.MatchRunning, domain.MatchCompleted, domain.MatchFailed:
			filter.Status = s
		default:
			writeError(w, errors.ErrInvalidInput.WithMessage("invalid status filter, must be one of: pending, running, completed, failed"))
			return
		}
	}

	pg := pagination.ParseLimitOffset(r, 50, 0)
	filter.Limit = pg.Limit
	filter.Offset = pg.Offset

	matches, err := h.matchRepo.List(r.Context(), filter)
	if err != nil {
		h.log.LogError("Failed to get game matches", err,
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_id", gameID.String()),
			zap.String("game_type", g.Name),
		)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, matches)
}

// GetGamePrograms returns programs for a specific game in a tournament.
// GET /api/v1/tournaments/:id/games/:gameId/programs
func (h *GameRoundHandler) GetGamePrograms(w http.ResponseWriter, r *http.Request) {
	tournamentID, gameID, ok := h.parseTournamentGameIDs(w, r)
	if !ok {
		return
	}

	if h.programRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("program repository not configured"))
		return
	}

	programs, err := h.programRepo.GetByTournamentAndGame(r.Context(), tournamentID, gameID)
	if err != nil {
		h.log.LogError("Failed to get game programs", err,
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_id", gameID.String()),
		)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, programs)
}

// TournamentGameWithDetails contains tournament-game link info with game details.
type TournamentGameWithDetails struct {
	TournamentID          uuid.UUID `json:"tournament_id"`
	GameID                uuid.UUID `json:"game_id"`
	GameName              string    `json:"game_name"`
	GameDisplayName       string    `json:"game_display_name"`
	IsActive              bool      `json:"is_active"`
	RoundCompleted        bool      `json:"round_completed"`
	RoundCompletedAt      *string   `json:"round_completed_at,omitempty"`
	CurrentRound          int       `json:"current_round"`
	AutoRoundEnabled      bool      `json:"auto_round_enabled"`
	AutoRoundIntervalSecs int       `json:"auto_round_interval_seconds"`
	AutoRoundLastRunAt    *string   `json:"auto_round_last_run_at,omitempty"`
}

// GetTournamentGamesWithStatus returns games with their round status.
// GET /api/v1/tournaments/{id}/games/status
func (h *GameRoundHandler) GetTournamentGamesWithStatus(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	tournamentID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
		return
	}

	if h.tournamentGameStatusRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("tournament game status repository not configured"))
		return
	}

	details, err := h.tournamentGameStatusRepo.GetTournamentGamesWithDetails(r.Context(), tournamentID)
	if err != nil {
		h.log.LogError("Failed to get tournament games status", err,
			zap.String("tournament_id", tournamentID.String()),
		)
		writeError(w, err)
		return
	}

	result := make([]TournamentGameWithDetails, 0, len(details))
	for _, d := range details {
		item := TournamentGameWithDetails{
			TournamentID:          d.TournamentID,
			GameID:                d.GameID,
			GameName:              d.GameName,
			GameDisplayName:       d.GameDisplayName,
			IsActive:              d.IsActive,
			RoundCompleted:        d.RoundCompleted,
			CurrentRound:          d.CurrentRound,
			AutoRoundEnabled:      d.AutoRoundEnabled,
			AutoRoundIntervalSecs: d.AutoRoundIntervalSecs,
		}
		if d.RoundCompletedAt != nil {
			formatted := d.RoundCompletedAt.Format("2006-01-02T15:04:05Z07:00")
			item.RoundCompletedAt = &formatted
		}
		if d.AutoRoundLastRunAt != nil {
			formatted := d.AutoRoundLastRunAt.Format("2006-01-02T15:04:05Z07:00")
			item.AutoRoundLastRunAt = &formatted
		}
		result = append(result, item)
	}

	writeJSON(w, http.StatusOK, result)
}

// MarkGameRoundCompleted marks a game round as completed.
// POST /api/v1/tournaments/{id}/games/{gameId}/complete-round
func (h *GameRoundHandler) MarkGameRoundCompleted(w http.ResponseWriter, r *http.Request) {
	tournamentID, gameID, ok := h.parseTournamentGameIDs(w, r)
	if !ok {
		return
	}

	if h.tournamentGameStatusRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("tournament game status repository not configured"))
		return
	}

	if err := h.tournamentGameStatusRepo.MarkRoundCompleted(r.Context(), tournamentID, gameID); err != nil {
		h.log.LogError("Failed to mark round completed", err,
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_id", gameID.String()),
		)
		writeError(w, err)
		return
	}

	h.log.Info("Game round marked as completed",
		zap.String("tournament_id", tournamentID.String()),
		zap.String("game_id", gameID.String()),
	)

	w.WriteHeader(http.StatusNoContent)
}

// SetActiveGameRequest is the request for setting the active game.
type SetActiveGameRequest struct {
	GameID uuid.UUID `json:"game_id"`
}

// SetActiveGame sets the active game for a tournament.
// POST /api/v1/tournaments/{id}/active-game
func (h *GameRoundHandler) SetActiveGame(w http.ResponseWriter, r *http.Request) {
	tournamentIDStr := chi.URLParam(r, "id")
	tournamentID, err := uuid.Parse(tournamentIDStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
		return
	}

	if h.tournamentGameStatusRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("tournament game status repository not configured"))
		return
	}

	var req SetActiveGameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithMessage("game_id is required"))
		return
	}

	if err := h.tournamentGameStatusRepo.SetActiveGame(r.Context(), tournamentID, req.GameID); err != nil {
		h.log.LogError("Failed to set active game", err,
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_id", req.GameID.String()),
		)
		writeError(w, err)
		return
	}

	h.log.Info("Active game set",
		zap.String("tournament_id", tournamentID.String()),
		zap.String("game_id", req.GameID.String()),
	)

	w.WriteHeader(http.StatusNoContent)
}

// DeactivateAllGames deactivates all games in a tournament.
// POST /api/v1/tournaments/{id}/games/deactivate-all
func (h *GameRoundHandler) DeactivateAllGames(w http.ResponseWriter, r *http.Request) {
	tournamentIDStr := chi.URLParam(r, "id")
	tournamentID, err := uuid.Parse(tournamentIDStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
		return
	}

	if h.tournamentGameStatusRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("tournament game status repository not configured"))
		return
	}

	if err := h.tournamentGameStatusRepo.DeactivateAllGames(r.Context(), tournamentID); err != nil {
		h.log.LogError("Failed to deactivate all games", err,
			zap.String("tournament_id", tournamentID.String()),
		)
		writeError(w, err)
		return
	}

	h.log.Info("All games deactivated",
		zap.String("tournament_id", tournamentID.String()),
	)

	w.WriteHeader(http.StatusNoContent)
}

// GetActiveGame returns the currently active game for a tournament.
// GET /api/v1/tournaments/{id}/active-game
func (h *GameRoundHandler) GetActiveGame(w http.ResponseWriter, r *http.Request) {
	tournamentIDStr := chi.URLParam(r, "id")
	tournamentID, err := uuid.Parse(tournamentIDStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
		return
	}

	if h.tournamentGameStatusRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("tournament game status repository not configured"))
		return
	}

	activeGame, err := h.tournamentGameStatusRepo.GetActiveGame(r.Context(), tournamentID)
	if err != nil {
		if errors.IsNotFound(err) {
			writeJSON(w, http.StatusOK, nil)
			return
		}
		h.log.LogError("Failed to get active game", err,
			zap.String("tournament_id", tournamentID.String()),
		)
		writeError(w, err)
		return
	}

	g, err := h.gameService.GetByID(r.Context(), activeGame.GameID)
	if err != nil {
		h.log.LogError("Failed to get game details", err,
			zap.String("game_id", activeGame.GameID.String()),
		)
		writeError(w, err)
		return
	}

	result := TournamentGameWithDetails{
		TournamentID:          activeGame.TournamentID,
		GameID:                activeGame.GameID,
		GameName:              g.Name,
		GameDisplayName:       g.DisplayName,
		IsActive:              activeGame.IsActive,
		RoundCompleted:        activeGame.RoundCompleted,
		CurrentRound:          activeGame.CurrentRound,
		AutoRoundEnabled:      activeGame.AutoRoundEnabled,
		AutoRoundIntervalSecs: activeGame.AutoRoundIntervalSecs,
	}
	if activeGame.RoundCompletedAt != nil {
		formatted := activeGame.RoundCompletedAt.Format("2006-01-02T15:04:05Z07:00")
		result.RoundCompletedAt = &formatted
	}
	if activeGame.AutoRoundLastRunAt != nil {
		formatted := activeGame.AutoRoundLastRunAt.Format("2006-01-02T15:04:05Z07:00")
		result.AutoRoundLastRunAt = &formatted
	}

	writeJSON(w, http.StatusOK, result)
}

// ResetGameRoundResponse is the response for a round reset.
type ResetGameRoundResponse struct {
	MatchesDeleted     int64 `json:"matches_deleted"`
	ParticipantsReset  int64 `json:"participants_reset"`
	RatingHistoryReset int64 `json:"rating_history_reset"`
}

// ResetGameRound fully resets a game round: deletes matches, resets ratings and stats.
// POST /api/v1/tournaments/{id}/games/{gameId}/reset-round
func (h *GameRoundHandler) ResetGameRound(w http.ResponseWriter, r *http.Request) {
	tournamentID, gameID, ok := h.parseTournamentGameIDs(w, r)
	if !ok {
		return
	}

	if h.tournamentGameStatusRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("tournament game status repository not configured"))
		return
	}

	g, err := h.gameService.GetByID(r.Context(), gameID)
	if err != nil {
		h.log.LogError("Failed to get game details", err,
			zap.String("game_id", gameID.String()),
		)
		writeError(w, err)
		return
	}

	matchesDeleted, participantsReset, ratingHistoryDeleted, err := h.tournamentGameStatusRepo.ResetGameRoundFull(r.Context(), tournamentID, gameID, g.Name)
	if err != nil {
		h.log.LogError("Failed to reset game round", err,
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_id", gameID.String()),
		)
		writeError(w, err)
		return
	}

	h.log.Info("Game round reset completed",
		zap.String("tournament_id", tournamentID.String()),
		zap.String("game_id", gameID.String()),
		zap.Int64("matches_deleted", matchesDeleted),
		zap.Int64("participants_reset", participantsReset),
		zap.Int64("rating_history_reset", ratingHistoryDeleted),
	)

	h.eventBus.Publish(r.Context(), events.GameRoundReset{
		TournamentID: tournamentID,
		GameID:       gameID,
	})

	writeJSON(w, http.StatusOK, ResetGameRoundResponse{
		MatchesDeleted:     matchesDeleted,
		ParticipantsReset:  participantsReset,
		RatingHistoryReset: ratingHistoryDeleted,
	})
}

// parseTournamentGameIDs is a helper to parse tournament and game IDs from URL params.
func (h *GameRoundHandler) parseTournamentGameIDs(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	tournamentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
		return uuid.Nil, uuid.Nil, false
	}

	gameID, err := uuid.Parse(chi.URLParam(r, "gameId"))
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid game ID"))
		return uuid.Nil, uuid.Nil, false
	}

	return tournamentID, gameID, true
}

// SetAutoRound включает или выключает авто-раунд для игры в турнире
// POST /api/v1/tournaments/{id}/games/{gameId}/auto-round
func (h *GameRoundHandler) SetAutoRound(w http.ResponseWriter, r *http.Request) {
	tournamentID, gameID, ok := h.parseTournamentGameIDs(w, r)
	if !ok {
		return
	}

	var req struct {
		Enabled         bool `json:"enabled"`
		IntervalSeconds int  `json:"interval_seconds"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid request body"))
		return
	}

	if req.Enabled && (req.IntervalSeconds < 10 || req.IntervalSeconds > 3600) {
		writeError(w, errors.ErrValidation.WithMessage("interval_seconds must be between 10 and 3600"))
		return
	}

	// Если выключаем, ставим дефолтный интервал
	if !req.Enabled && req.IntervalSeconds == 0 {
		req.IntervalSeconds = 60
	}

	if err := h.tournamentGameStatusRepo.SetAutoRound(r.Context(), tournamentID, gameID, req.Enabled, req.IntervalSeconds); err != nil {
		h.log.LogError("Failed to set auto-round", err,
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_id", gameID.String()),
		)
		writeError(w, err)
		return
	}

	status := "disabled"
	if req.Enabled {
		status = "enabled"
	}

	h.log.Info("Auto-round updated",
		zap.String("tournament_id", tournamentID.String()),
		zap.String("game_id", gameID.String()),
		zap.String("status", status),
		zap.Int("interval", req.IntervalSeconds),
	)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"enabled":          req.Enabled,
		"interval_seconds": req.IntervalSeconds,
	})
}

// GetAutoRound возвращает статус авто-раунда для игры в турнире
// GET /api/v1/tournaments/{id}/games/{gameId}/auto-round
func (h *GameRoundHandler) GetAutoRound(w http.ResponseWriter, r *http.Request) {
	tournamentID, gameID, ok := h.parseTournamentGameIDs(w, r)
	if !ok {
		return
	}

	tg, err := h.tournamentGameStatusRepo.GetTournamentGame(r.Context(), tournamentID, gameID)
	if err != nil {
		h.log.LogError("Failed to get auto-round status", err,
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_id", gameID.String()),
		)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"enabled":          tg.AutoRoundEnabled,
		"interval_seconds": tg.AutoRoundIntervalSecs,
		"last_run_at":      tg.AutoRoundLastRunAt,
	})
}

// DownloadAllPrograms streams a ZIP archive of all programs for a tournament.
// GET /api/v1/tournaments/{id}/programs/download-zip
func (h *GameRoundHandler) DownloadAllPrograms(w http.ResponseWriter, r *http.Request) {
	tournamentIDStr := chi.URLParam(r, "id")
	tournamentID, err := uuid.Parse(tournamentIDStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament ID"))
		return
	}

	if h.tournamentGameStatusRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("tournament game status repository not configured"))
		return
	}
	if h.programRepo == nil {
		writeError(w, errors.ErrInternal.WithMessage("program repository not configured"))
		return
	}

	// Get all games in this tournament
	games, err := h.tournamentGameStatusRepo.GetTournamentGames(r.Context(), tournamentID)
	if err != nil {
		h.log.LogError("Failed to get tournament games", err,
			zap.String("tournament_id", tournamentID.String()),
		)
		writeError(w, err)
		return
	}

	// Resolve upload directory for path validation
	absUploadDir, err := filepath.Abs(h.uploadDir)
	if err != nil {
		h.log.Error("Failed to resolve upload dir", zap.Error(err))
		writeError(w, errors.ErrInternal.WithMessage("invalid upload directory"))
		return
	}

	// Set response headers before writing body
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"programs_%s.zip\"", tournamentID.String()[:8]))

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	filesAdded := 0

	for _, tg := range games {
		// Get game details for display name
		game, err := h.gameService.GetByID(r.Context(), tg.GameID)
		if err != nil {
			h.log.Error("Failed to get game details, skipping",
				zap.String("game_id", tg.GameID.String()),
				zap.Error(err),
			)
			continue
		}

		gameDirName := sanitizeZipPath(game.DisplayName)
		if gameDirName == "" {
			gameDirName = game.Name
		}

		// Get latest programs for this game
		programs, err := h.programRepo.GetByTournamentAndGame(r.Context(), tournamentID, tg.GameID)
		if err != nil {
			h.log.Error("Failed to get programs for game, skipping",
				zap.String("game_id", tg.GameID.String()),
				zap.Error(err),
			)
			continue
		}

		for _, prog := range programs {
			if prog.FilePath == nil || *prog.FilePath == "" {
				continue
			}

			filePath := *prog.FilePath

			// Validate path is within upload directory
			absFilePath, err := filepath.Abs(filePath)
			if err != nil || !strings.HasPrefix(absFilePath, absUploadDir+string(os.PathSeparator)) {
				h.log.Error("Program file path outside upload dir, skipping",
					zap.String("program_id", prog.ID.String()),
					zap.String("path", filePath),
				)
				continue
			}

			// Check file exists
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				h.log.Error("Program file not found on disk, skipping",
					zap.String("program_id", prog.ID.String()),
					zap.String("path", filePath),
				)
				continue
			}

			// Build ZIP entry path: game_name/program_name_v{version}.ext
			ext := filepath.Ext(filePath)
			entryName := fmt.Sprintf("%s/%s_v%d%s", gameDirName, sanitizeZipPath(prog.Name), prog.Version, ext)

			f, err := os.Open(filePath)
			if err != nil {
				h.log.Error("Failed to open program file, skipping",
					zap.String("program_id", prog.ID.String()),
					zap.Error(err),
				)
				continue
			}

			zf, err := zipWriter.Create(entryName)
			if err != nil {
				f.Close()
				h.log.Error("Failed to create ZIP entry", zap.Error(err))
				continue
			}

			if _, err := io.Copy(zf, f); err != nil {
				f.Close()
				h.log.Error("Failed to write program to ZIP", zap.Error(err))
				continue
			}
			f.Close()
			filesAdded++
		}
	}

	h.log.Info("Programs archive created",
		zap.String("tournament_id", tournamentID.String()),
		zap.Int("files_added", filesAdded),
	)
}

// sanitizeZipPath cleans a name for use in ZIP entry paths.
func sanitizeZipPath(name string) string {
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' || r == '\x00' {
			return '_'
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "" {
		name = "unknown"
	}
	return name
}
