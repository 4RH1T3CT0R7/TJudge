package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/bmstu-itstech/tjudge/internal/api/middleware"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ProgramRepository интерфейс для работы с программами
type ProgramRepository interface {
	Create(ctx context.Context, program *domain.Program) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Program, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Program, error)
	Update(ctx context.Context, program *domain.Program) error
	Delete(ctx context.Context, id uuid.UUID) error
	CheckOwnership(ctx context.Context, programID, userID uuid.UUID) (bool, error)
}

// ProgramHandler обрабатывает запросы программ
type ProgramHandler struct {
	programRepo ProgramRepository
	log         *logger.Logger
}

// NewProgramHandler создаёт новый program handler
func NewProgramHandler(programRepo ProgramRepository, log *logger.Logger) *ProgramHandler {
	return &ProgramHandler{
		programRepo: programRepo,
		log:         log,
	}
}

// Create обрабатывает создание программы
// POST /api/v1/programs
func (h *ProgramHandler) Create(w http.ResponseWriter, r *http.Request) {
	// Получаем user ID из контекста
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	// Декодируем тело запроса
	var req struct {
		Name     string `json:"name"`
		GameType string `json:"game_type"`
		CodePath string `json:"code_path"`
		Language string `json:"language"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
	}

	// Создаём программу
	program := &domain.Program{
		ID:       uuid.New(),
		UserID:   userID,
		Name:     req.Name,
		GameType: req.GameType,
		CodePath: req.CodePath,
		Language: req.Language,
	}

	// Валидация
	if err := program.Validate(); err != nil {
		writeError(w, errors.ErrValidation.WithError(err))
		return
	}

	// Сохраняем в БД
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

// List обрабатывает получение списка программ текущего пользователя
// GET /api/v1/programs
func (h *ProgramHandler) List(w http.ResponseWriter, r *http.Request) {
	// Получаем user ID из контекста
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	// Получаем программы пользователя
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
// GET /api/v1/programs/:id
func (h *ProgramHandler) Get(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из URL
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid program ID"))
		return
	}

	// Получаем программу
	program, err := h.programRepo.GetByID(r.Context(), id)
	if err != nil {
		h.log.LogError("Failed to get program", err,
			zap.String("program_id", id.String()),
		)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, program)
}

// Update обрабатывает обновление программы
// PUT /api/v1/programs/:id
func (h *ProgramHandler) Update(w http.ResponseWriter, r *http.Request) {
	// Получаем user ID из контекста
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	// Извлекаем ID из URL
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid program ID"))
		return
	}

	// Проверяем ownership
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

	// Декодируем тело запроса
	var req struct {
		Name     string `json:"name"`
		CodePath string `json:"code_path"`
		Language string `json:"language"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
	}

	// Получаем текущую программу
	program, err := h.programRepo.GetByID(r.Context(), id)
	if err != nil {
		h.log.LogError("Failed to get program", err)
		writeError(w, err)
		return
	}

	// Обновляем поля
	program.Name = req.Name
	program.CodePath = req.CodePath
	program.Language = req.Language

	// Валидация
	if err := program.Validate(); err != nil {
		writeError(w, errors.ErrValidation.WithError(err))
		return
	}

	// Сохраняем
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
	// Получаем user ID из контекста
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	// Извлекаем ID из URL
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid program ID"))
		return
	}

	// Проверяем ownership
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

	// Удаляем
	if err := h.programRepo.Delete(r.Context(), id); err != nil {
		h.log.LogError("Failed to delete program", err)
		writeError(w, err)
		return
	}

	h.log.Info("Program deleted",
		zap.String("program_id", id.String()),
		zap.String("user_id", userID.String()),
	)

	w.WriteHeader(http.StatusNoContent)
}
