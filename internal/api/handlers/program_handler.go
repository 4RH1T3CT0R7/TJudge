package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmstu-itstech/tjudge/internal/api/httputil"
	"github.com/bmstu-itstech/tjudge/internal/api/middleware"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Create обрабатывает создание программы (с загрузкой файла)
// POST /api/v1/programs
func (h *ProgramHandler) Create(w http.ResponseWriter, r *http.Request) {
	// Получаем user ID из контекста
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	// Проверяем Content-Type
	contentType := r.Header.Get("Content-Type")

	// Если multipart/form-data - загрузка файла
	if strings.HasPrefix(contentType, "multipart/form-data") {
		h.handleFileUpload(w, r, userID)
		return
	}

	// Иначе - JSON запрос (для обратной совместимости)
	h.handleJSONCreate(w, r, userID)
}

// handleJSONCreate обрабатывает JSON запрос (обратная совместимость)
func (h *ProgramHandler) handleJSONCreate(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	var req struct {
		Name     string `json:"name"`
		GameType string `json:"game_type"`
		CodePath string `json:"code_path"`
		Language string `json:"language"`
	}

	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
	}

	// Reject path traversal and enforce upload directory boundary for absolute paths.
	if req.CodePath != "" {
		cleaned := filepath.Clean(req.CodePath)
		if strings.Contains(cleaned, "..") {
			writeError(w, errors.ErrForbidden.WithMessage("invalid code path"))
			return
		}
		// Absolute paths must be within the upload directory
		if filepath.IsAbs(cleaned) {
			uploadDir := filepath.Clean(h.uploadDir)
			if !strings.HasPrefix(cleaned, uploadDir+string(filepath.Separator)) {
				writeError(w, errors.ErrForbidden.WithMessage("code path must be within the programs directory"))
				return
			}
		}
		req.CodePath = cleaned
	}

	program := &domain.Program{
		ID:       uuid.New(),
		UserID:   userID,
		Name:     req.Name,
		GameType: req.GameType,
		CodePath: req.CodePath,
		Language: req.Language,
		Version:  1,
	}

	if err := program.Validate(); err != nil {
		writeError(w, errors.ErrValidation.WithError(err))
		return
	}

	if err := h.programRepo.Create(r.Context(), program); err != nil {
		h.log.LogError("Failed to create program", err)
		writeError(w, err)
		return
	}

	h.log.Info("Program created",
		zap.String("program_id", program.ID.String()),
		zap.String("user_id", userID.String()),
		zap.String("name", program.Name),
	)

	writeJSON(w, http.StatusCreated, program)
}

// Update обрабатывает обновление программы
// PUT /api/v1/programs/:id
func (h *ProgramHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	id, ok := httputil.ParseUUIDParam(w, r, "id", "program")
	if !ok {
		return
	}

	isOwner, err := h.programRepo.CheckOwnership(r.Context(), id, userID)
	if err != nil {
		h.log.LogError("Failed to check ownership", err)
		writeError(w, err)
		return
	}
	if !isOwner {
		writeError(w, errors.ErrForbidden.WithMessage("you don't own this program"))
		return
	}

	var req struct {
		Name     string `json:"name"`
		CodePath string `json:"code_path"`
		Language string `json:"language"`
	}

	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
	}

	// Reject path traversal and enforce upload directory boundary for absolute paths.
	if req.CodePath != "" {
		cleaned := filepath.Clean(req.CodePath)
		if strings.Contains(cleaned, "..") {
			writeError(w, errors.ErrForbidden.WithMessage("invalid code path"))
			return
		}
		// Absolute paths must be within the upload directory
		if filepath.IsAbs(cleaned) {
			uploadDir := filepath.Clean(h.uploadDir)
			if !strings.HasPrefix(cleaned, uploadDir+string(filepath.Separator)) {
				writeError(w, errors.ErrForbidden.WithMessage("code path must be within the programs directory"))
				return
			}
		}
		req.CodePath = cleaned
	}

	program, err := h.programRepo.GetByID(r.Context(), id)
	if err != nil {
		h.log.LogError("Failed to get program", err)
		writeError(w, err)
		return
	}

	program.Name = req.Name
	program.CodePath = req.CodePath
	program.Language = req.Language

	if err := program.Validate(); err != nil {
		writeError(w, errors.ErrValidation.WithError(err))
		return
	}

	if err := h.programRepo.Update(r.Context(), program); err != nil {
		h.log.LogError("Failed to update program", err)
		writeError(w, err)
		return
	}

	h.log.Info("Program updated",
		zap.String("program_id", id.String()),
		zap.String("user_id", userID.String()),
	)

	writeJSON(w, http.StatusOK, program)
}

// Delete обрабатывает удаление программы
// DELETE /api/v1/programs/:id
func (h *ProgramHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	id, ok := httputil.ParseUUIDParam(w, r, "id", "program")
	if !ok {
		return
	}

	isOwner, err := h.programRepo.CheckOwnership(r.Context(), id, userID)
	if err != nil {
		h.log.LogError("Failed to check ownership", err)
		writeError(w, err)
		return
	}
	if !isOwner {
		writeError(w, errors.ErrForbidden.WithMessage("you don't own this program"))
		return
	}

	// Получаем программу чтобы удалить файл
	program, err := h.programRepo.GetByID(r.Context(), id)
	if err != nil {
		h.log.LogError("Failed to get program", err)
		writeError(w, err)
		return
	}

	// Удаляем из БД
	if err := h.programRepo.Delete(r.Context(), id); err != nil {
		h.log.LogError("Failed to delete program", err)
		writeError(w, err)
		return
	}

	// Удаляем файл (если есть)
	if program.FilePath != nil && *program.FilePath != "" {
		if err := os.Remove(*program.FilePath); err != nil {
			h.log.Warn("Failed to delete program file", zap.Error(err), zap.String("path", *program.FilePath))
		}
	}

	h.log.Info("Program deleted",
		zap.String("program_id", id.String()),
		zap.String("user_id", userID.String()),
	)

	w.WriteHeader(http.StatusNoContent)
}

// ClearProgramErrors очищает сообщения об ошибках для всех программ турнира (только для админов)
// POST /api/v1/tournaments/:id/programs/clear-errors
func (h *ProgramHandler) ClearProgramErrors(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

	// Очищаем ошибки
	cleared, err := h.programRepo.ClearErrorMessages(r.Context(), tournamentID)
	if err != nil {
		h.log.LogError("Failed to clear program errors", err,
			zap.String("tournament_id", tournamentID.String()),
		)
		writeError(w, err)
		return
	}

	h.log.Info("Program errors cleared",
		zap.String("tournament_id", tournamentID.String()),
		zap.Int64("cleared_count", cleared),
	)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"cleared": cleared,
		"message": fmt.Sprintf("Очищено %d ошибок", cleared),
	})
}
