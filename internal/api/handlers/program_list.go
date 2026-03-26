package handlers

import (
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
	"go.uber.org/zap"
)

// List обрабатывает получение списка программ текущего пользователя
// @Summary Мои программы
// @Description Возвращает список программ текущего пользователя
// @Tags programs
// @Produce json
// @Security BearerAuth
// @Success 200 {array} domain.Program
// @Failure 401 {object} object{error=string}
// @Router /programs [get]
func (h *ProgramHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	programs, err := h.programRepo.GetByUserID(r.Context(), userID)
	if err != nil {
		h.log.LogError("Failed to get programs", err,
			zap.String("user_id", userID.String()),
		)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, programs)
}

// Get обрабатывает получение программы
// @Summary Получить программу
// @Description Возвращает программу по ID (владелец или админ)
// @Tags programs
// @Produce json
// @Param id path string true "Program ID" format(uuid)
// @Security BearerAuth
// @Success 200 {object} domain.Program
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /programs/{id} [get]
func (h *ProgramHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	id, ok := httputil.ParseUUIDParam(w, r, "id", "program")
	if !ok {
		return
	}

	program, err := h.programRepo.GetByID(r.Context(), id)
	if err != nil {
		h.log.LogError("Failed to get program", err,
			zap.String("program_id", id.String()),
		)
		writeError(w, err)
		return
	}

	// Admins can view any program; others must own it
	userRole, _ := r.Context().Value(middleware.RoleKey).(domain.Role)
	if userRole != domain.RoleAdmin && program.UserID != userID {
		writeError(w, errors.ErrForbidden.WithMessage("you don't own this program"))
		return
	}

	writeJSON(w, http.StatusOK, program)
}

// Download скачивает файл программы
// @Summary Скачать программу
// @Description Скачивает файл программы (владелец или админ)
// @Tags programs
// @Produce application/octet-stream
// @Param id path string true "Program ID" format(uuid)
// @Security BearerAuth
// @Success 200 {file} binary "Файл программы"
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /programs/{id}/download [get]
func (h *ProgramHandler) Download(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	id, ok := httputil.ParseUUIDParam(w, r, "id", "program")
	if !ok {
		return
	}

	// Admins can download any program
	userRole, _ := r.Context().Value(middleware.RoleKey).(domain.Role)
	if userRole != domain.RoleAdmin {
		// Проверяем владение программой
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
	}

	program, err := h.programRepo.GetByID(r.Context(), id)
	if err != nil {
		h.log.LogError("Failed to get program", err)
		writeError(w, err)
		return
	}

	// Проверяем наличие файла
	if program.FilePath == nil || *program.FilePath == "" {
		writeError(w, errors.ErrNotFound.WithMessage("program file not found"))
		return
	}

	filePath := *program.FilePath

	// Validate file path is within upload directory
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		h.log.Error("Failed to resolve absolute file path", zap.Error(err), zap.String("path", filePath))
		writeError(w, errors.ErrInternal.WithMessage("invalid file path"))
		return
	}
	absUploadDir, err := filepath.Abs(h.uploadDir)
	if err != nil {
		h.log.Error("Failed to resolve absolute upload dir", zap.Error(err), zap.String("upload_dir", h.uploadDir))
		writeError(w, errors.ErrInternal.WithMessage("invalid upload directory"))
		return
	}
	if !strings.HasPrefix(absFilePath, absUploadDir+string(os.PathSeparator)) {
		h.log.Error("File path outside upload directory", zap.String("path", filePath), zap.String("upload_dir", h.uploadDir))
		writeError(w, errors.ErrForbidden.WithMessage("access denied"))
		return
	}

	// Проверяем существование файла (используем absFilePath для defense-in-depth)
	if _, err := os.Stat(absFilePath); os.IsNotExist(err) {
		h.log.Error("Program file does not exist", zap.String("path", absFilePath))
		writeError(w, errors.ErrNotFound.WithMessage("program file not found on disk"))
		return
	}

	// Открываем файл
	file, err := os.Open(absFilePath)
	if err != nil {
		h.log.Error("Failed to open file", zap.Error(err))
		writeError(w, errors.ErrInternal.WithMessage("failed to read file"))
		return
	}
	defer file.Close()

	// Определяем имя файла для скачивания
	filename := filepath.Base(filePath)
	if program.Name != "" {
		ext := filepath.Ext(filePath)
		if filepath.Ext(program.Name) == ext {
			filename = program.Name
		} else {
			filename = program.Name + ext
		}
	}

	// Санитизируем имя файла для безопасного использования в заголовке
	safeName := strings.Map(func(r rune) rune {
		if r == '"' || r == '\\' || r == '\r' || r == '\n' {
			return '_'
		}
		return r
	}, filename)

	// Устанавливаем заголовки для скачивания
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", safeName))
	w.Header().Set("Content-Type", "application/octet-stream")

	// Копируем файл в response
	if _, err := io.Copy(w, file); err != nil {
		h.log.Error("Failed to send file", zap.Error(err))
		// Уже начали отправлять, не можем вернуть ошибку
		return
	}

	h.log.Info("Program downloaded",
		zap.String("program_id", id.String()),
		zap.String("user_id", userID.String()),
	)
}

// GetVersions получает все версии программ для команды и игры
// @Summary Версии программ
// @Description Возвращает все версии программ для команды и игры
// @Tags programs
// @Produce json
// @Param team_id query string true "Team ID" format(uuid)
// @Param game_id query string true "Game ID" format(uuid)
// @Security BearerAuth
// @Success 200 {array} domain.Program
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /programs/versions [get]
func (h *ProgramHandler) GetVersions(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	teamID, ok := httputil.ParseQueryUUID(w, r, "team_id")
	if !ok {
		return
	}

	gameID, ok := httputil.ParseQueryUUID(w, r, "game_id")
	if !ok {
		return
	}

	// Получаем все версии программ
	programs, err := h.programRepo.GetAllVersionsByTeamAndGame(r.Context(), teamID, gameID)
	if err != nil {
		h.log.LogError("Failed to get program versions", err)
		writeError(w, err)
		return
	}

	// Проверяем что хотя бы одна программа принадлежит текущему пользователю
	hasAccess := false
	for _, p := range programs {
		if p.UserID == userID {
			hasAccess = true
			break
		}
	}

	if !hasAccess && len(programs) > 0 {
		writeError(w, errors.ErrForbidden.WithMessage("you don't have access to these programs"))
		return
	}

	h.log.Info("Program versions fetched",
		zap.String("team_id", teamID.String()),
		zap.String("game_id", gameID.String()),
		zap.Int("count", len(programs)),
	)

	writeJSON(w, http.StatusOK, programs)
}
