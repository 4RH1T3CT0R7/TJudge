package handlers

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmstu-itstech/tjudge/internal/api/httputil"
	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"go.uber.org/zap"
)

// MarkGameRoundCompleted marks a game round as completed.
// @Summary Завершить раунд игры
// @Description Отмечает текущий раунд игры как завершённый (только для админов)
// @Tags games
// @Param id path string true "Tournament ID" format(uuid)
// @Param gameId path string true "Game ID" format(uuid)
// @Security BearerAuth
// @Success 204 "Раунд завершён"
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/games/{gameId}/complete-round [post]
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

// ResetGameRoundResponse is the response for a round reset.
type ResetGameRoundResponse struct {
	MatchesDeleted     int64 `json:"matches_deleted"`
	ParticipantsReset  int64 `json:"participants_reset"`
	RatingHistoryReset int64 `json:"rating_history_reset"`
}

// ResetGameRound fully resets a game round: deletes matches, resets ratings and stats.
// @Summary Сбросить раунд игры
// @Description Полностью сбрасывает раунд: удаляет матчи, обнуляет рейтинги и статистику (только для админов)
// @Tags games
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Param gameId path string true "Game ID" format(uuid)
// @Security BearerAuth
// @Success 200 {object} ResetGameRoundResponse
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/games/{gameId}/reset-round [post]
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
		Version:      1,
		TournamentID: tournamentID,
		GameID:       gameID,
	})

	writeJSON(w, http.StatusOK, ResetGameRoundResponse{
		MatchesDeleted:     matchesDeleted,
		ParticipantsReset:  participantsReset,
		RatingHistoryReset: ratingHistoryDeleted,
	})
}

// SetAutoRound enables or disables auto-round for a game in a tournament.
// @Summary Настроить авто-раунд
// @Description Включает или выключает автоматический запуск раундов для игры (только для админов)
// @Tags games
// @Accept json
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Param gameId path string true "Game ID" format(uuid)
// @Param request body object{enabled=bool,interval_seconds=int} true "Настройки авто-раунда"
// @Security BearerAuth
// @Success 200 {object} object{enabled=bool,interval_seconds=int}
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /tournaments/{id}/games/{gameId}/auto-round [post]
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

// GetAutoRound returns the auto-round status for a game in a tournament.
// @Summary Статус авто-раунда
// @Description Возвращает текущие настройки авто-раунда для игры (только для админов)
// @Tags games
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Param gameId path string true "Game ID" format(uuid)
// @Security BearerAuth
// @Success 200 {object} object{enabled=bool,interval_seconds=int,last_run_at=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/games/{gameId}/auto-round [get]
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
// @Summary Скачать все программы
// @Description Скачивает ZIP-архив со всеми программами турнира (только для админов)
// @Tags games
// @Produce application/zip
// @Param id path string true "Tournament ID" format(uuid)
// @Security BearerAuth
// @Success 200 {file} binary "ZIP-архив программ"
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/programs/download-zip [get]
func (h *GameRoundHandler) DownloadAllPrograms(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
	if !ok {
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

	// Resolve upload directory for path validation (EvalSymlinks resolves symlinks too)
	absUploadDir, err := filepath.EvalSymlinks(h.uploadDir)
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

			// Validate path is within upload directory (EvalSymlinks resolves symlinks too)
			absFilePath, err := filepath.EvalSymlinks(filePath)
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

// DeactivateAllGames deactivates all games in a tournament.
// @Summary Деактивировать все игры
// @Description Деактивирует все игры в турнире (только для админов)
// @Tags games
// @Param id path string true "Tournament ID" format(uuid)
// @Security BearerAuth
// @Success 204 "Все игры деактивированы"
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/games/deactivate-all [post]
func (h *GameRoundHandler) DeactivateAllGames(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
	if !ok {
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

// sanitizeZipPath cleans a name for use in ZIP entry paths.
func sanitizeZipPath(name string) string {
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' || r == '\x00' {
			return '_'
		}
		return r
	}, name)
	// Remove ".." traversal sequences
	name = strings.ReplaceAll(name, "..", "_")
	name = strings.TrimSpace(name)
	if name == "" {
		name = LangUnknown
	}
	return name
}
