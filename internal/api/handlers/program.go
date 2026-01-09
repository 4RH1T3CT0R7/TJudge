package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	GetLatestVersion(ctx context.Context, teamID, gameID uuid.UUID) (int, error)
	GetAllVersionsByTeamAndGame(ctx context.Context, teamID, gameID uuid.UUID) ([]*domain.Program, error)
}

// TournamentParticipantAdder интерфейс для добавления участников в турнир
type TournamentParticipantAdder interface {
	AddParticipant(ctx context.Context, participant *domain.TournamentParticipant) error
}

// MatchScheduler интерфейс для создания матчей
type MatchScheduler interface {
	ScheduleNewProgramMatches(ctx context.Context, tournamentID, gameID, newProgramID, teamID uuid.UUID) error
}

// GameLookup интерфейс для получения информации об игре
type GameLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Game, error)
}

// MatchExistenceChecker интерфейс для проверки существования матчей
type MatchExistenceChecker interface {
	HasStartedMatches(ctx context.Context, tournamentID uuid.UUID, gameType string) (bool, error)
	HasAnyRunningMatches(ctx context.Context, tournamentID uuid.UUID) (bool, error)
	GetActiveGameType(ctx context.Context, tournamentID uuid.UUID) (string, error)
}

// RoundCompletionChecker интерфейс для проверки завершения раунда игры
type RoundCompletionChecker interface {
	IsRoundCompleted(ctx context.Context, tournamentID, gameID uuid.UUID) (bool, error)
}

// ProgramHandler обрабатывает запросы программ
type ProgramHandler struct {
	programRepo    ProgramRepository
	tournamentRepo TournamentParticipantAdder
	matchScheduler MatchScheduler
	gameLookup     GameLookup
	matchChecker   MatchExistenceChecker
	roundChecker   RoundCompletionChecker
	uploadDir      string
	maxFileSize    int64
	log            *logger.Logger
}

// NewProgramHandler создаёт новый program handler
func NewProgramHandler(programRepo ProgramRepository, tournamentRepo TournamentParticipantAdder, matchScheduler MatchScheduler, log *logger.Logger) *ProgramHandler {
	// Создаём директорию для загрузок (используем PROGRAMS_PATH для согласованности с worker)
	uploadDir := os.Getenv("PROGRAMS_PATH")
	if uploadDir == "" {
		uploadDir = "/data/programs"
	}

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Error("Failed to create upload directory", zap.Error(err))
	}

	return &ProgramHandler{
		programRepo:    programRepo,
		tournamentRepo: tournamentRepo,
		matchScheduler: matchScheduler,
		uploadDir:      uploadDir,
		maxFileSize:    10 * 1024 * 1024, // 10MB
		log:            log,
	}
}

// SetGameLookup устанавливает GameLookup для проверки игр
func (h *ProgramHandler) SetGameLookup(gameLookup GameLookup) {
	h.gameLookup = gameLookup
}

// SetMatchChecker устанавливает MatchExistenceChecker для проверки наличия матчей
func (h *ProgramHandler) SetMatchChecker(matchChecker MatchExistenceChecker) {
	h.matchChecker = matchChecker
}

// SetRoundChecker устанавливает RoundCompletionChecker для проверки завершения раунда
func (h *ProgramHandler) SetRoundChecker(roundChecker RoundCompletionChecker) {
	h.roundChecker = roundChecker
}

// detectLanguage определяет язык программирования по расширению файла
func detectLanguage(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".py":
		return "python"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".c":
		return "c"
	case ".go":
		return "go"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".js":
		return "javascript"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".lua":
		return "lua"
	default:
		return "unknown"
	}
}

// getShebang возвращает shebang для интерпретируемых языков
func getShebang(language string) string {
	switch language {
	case "python":
		return "#!/usr/bin/env python3\n"
	case "javascript":
		return "#!/usr/bin/env node\n"
	case "ruby":
		return "#!/usr/bin/env ruby\n"
	case "php":
		return "#!/usr/bin/env php\n"
	case "lua":
		return "#!/usr/bin/env lua\n"
	default:
		return ""
	}
}

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

// handleFileUpload обрабатывает загрузку файла
func (h *ProgramHandler) handleFileUpload(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	// Ограничиваем размер файла
	r.Body = http.MaxBytesReader(w, r.Body, h.maxFileSize)

	// Парсим multipart form
	if err := r.ParseMultipartForm(h.maxFileSize); err != nil {
		h.log.Info("Failed to parse multipart form", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithMessage("file too large or invalid form"))
		return
	}

	// Получаем файл
	file, header, err := r.FormFile("file")
	if err != nil {
		h.log.Info("Failed to get file from form", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithMessage("file is required"))
		return
	}
	defer file.Close()

	// Получаем остальные поля
	teamIDStr := r.FormValue("team_id")
	tournamentIDStr := r.FormValue("tournament_id")
	gameIDStr := r.FormValue("game_id")
	name := r.FormValue("name")

	// Валидация обязательных полей
	if teamIDStr == "" || tournamentIDStr == "" || gameIDStr == "" {
		writeError(w, errors.ErrInvalidInput.WithMessage("team_id, tournament_id and game_id are required"))
		return
	}

	teamID, err := uuid.Parse(teamIDStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid team_id"))
		return
	}

	tournamentID, err := uuid.Parse(tournamentIDStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament_id"))
		return
	}

	gameID, err := uuid.Parse(gameIDStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid game_id"))
		return
	}

	// Проверяем, завершён ли раунд для этой игры (блокировка загрузки после завершения раунда)
	if h.roundChecker != nil {
		roundCompleted, err := h.roundChecker.IsRoundCompleted(r.Context(), tournamentID, gameID)
		if err != nil {
			h.log.LogError("Failed to check round completion", err,
				zap.String("tournament_id", tournamentID.String()),
				zap.String("game_id", gameID.String()),
			)
			// Продолжаем, если не можем проверить статус раунда
		} else if roundCompleted {
			h.log.Info("Upload blocked: round already completed",
				zap.String("tournament_id", tournamentID.String()),
				zap.String("game_id", gameID.String()),
			)
			writeError(w, errors.ErrForbidden.WithMessage("загрузка программ запрещена: раунд уже завершён для этой игры"))
			return
		}
	}

	// Проверяем, не выполняются ли матчи для ЛЮБОЙ игры в турнире
	// Пока идёт раунд одной игры, загрузка программ для всех игр заблокирована
	if h.matchChecker != nil {
		hasRunning, err := h.matchChecker.HasAnyRunningMatches(r.Context(), tournamentID)
		if err != nil {
			h.log.LogError("Failed to check running matches", err)
			writeError(w, errors.ErrInternal.WithMessage("failed to verify match status"))
			return
		}

		if hasRunning {
			// Получаем название активной игры для информативного сообщения
			activeGame, _ := h.matchChecker.GetActiveGameType(r.Context(), tournamentID)
			h.log.Info("Upload blocked: matches running for another game",
				zap.String("tournament_id", tournamentID.String()),
				zap.String("game_id", gameID.String()),
				zap.String("active_game", activeGame),
			)
			if activeGame != "" {
				writeError(w, errors.ErrForbidden.WithMessage(fmt.Sprintf("загрузка программ запрещена: выполняется раунд игры '%s'", activeGame)))
			} else {
				writeError(w, errors.ErrForbidden.WithMessage("загрузка программ запрещена: выполняется раунд"))
			}
			return
		}
	}

	// Если имя не указано, используем имя файла
	if name == "" {
		name = header.Filename
	}

	// Определяем язык по расширению
	language := detectLanguage(header.Filename)

	// Получаем последнюю версию программы для этой команды и игры
	version := 1
	if latestVersion, err := h.programRepo.GetLatestVersion(r.Context(), teamID, gameID); err == nil {
		version = latestVersion + 1
	}

	// Создаём уникальный путь для файла
	programID := uuid.New()
	ext := filepath.Ext(header.Filename)
	fileName := fmt.Sprintf("%s_%s_%s_v%d%s", teamID.String()[:8], gameID.String()[:8], programID.String()[:8], version, ext)
	filePath := filepath.Join(h.uploadDir, fileName)

	// Сохраняем файл
	dst, err := os.Create(filePath)
	if err != nil {
		h.log.Error("Failed to create file", zap.Error(err), zap.String("path", filePath))
		writeError(w, errors.ErrInternal.WithMessage("failed to save file"))
		return
	}
	defer dst.Close()

	// Добавляем shebang для интерпретируемых языков (если его нет)
	shebang := getShebang(language)
	if shebang != "" {
		// Читаем первые байты чтобы проверить наличие shebang
		firstBytes := make([]byte, 2)
		n, _ := file.Read(firstBytes)
		_, _ = file.Seek(0, 0) // Возвращаемся в начало

		// Если файл не начинается с #!, добавляем shebang
		if n < 2 || string(firstBytes) != "#!" {
			if _, err := dst.WriteString(shebang); err != nil {
				h.log.Error("Failed to write shebang", zap.Error(err))
				os.Remove(filePath)
				writeError(w, errors.ErrInternal.WithMessage("failed to save file"))
				return
			}
		}
	}

	if _, err := io.Copy(dst, file); err != nil {
		h.log.Error("Failed to write file", zap.Error(err))
		// Удаляем частично записанный файл
		os.Remove(filePath)
		writeError(w, errors.ErrInternal.WithMessage("failed to save file"))
		return
	}

	// Делаем файл исполняемым
	if err := os.Chmod(filePath, 0755); err != nil {
		h.log.Warn("Failed to make file executable", zap.Error(err), zap.String("path", filePath))
	}

	// Проверяем синтаксис для Python-файлов
	var syntaxError *string
	if language == "python" {
		if errMsg := validatePythonSyntax(filePath); errMsg != "" {
			syntaxError = &errMsg
			h.log.Info("Python syntax error detected",
				zap.String("file", filePath),
				zap.String("error", errMsg),
			)
		}
	}

	// Создаём запись в БД
	program := &domain.Program{
		ID:           programID,
		UserID:       userID,
		TeamID:       &teamID,
		TournamentID: &tournamentID,
		GameID:       &gameID,
		Name:         name,
		GameType:     "", // Заполнится из game
		CodePath:     filePath,
		FilePath:     &filePath,
		Language:     language,
		ErrorMessage: syntaxError,
		Version:      version,
	}

	if err := h.programRepo.Create(r.Context(), program); err != nil {
		h.log.LogError("Failed to create program", err)
		// Удаляем загруженный файл при ошибке
		os.Remove(filePath)
		writeError(w, err)
		return
	}

	// Автоматически регистрируем программу как участника турнира
	if h.tournamentRepo != nil {
		participant := &domain.TournamentParticipant{
			ID:           uuid.New(),
			TournamentID: tournamentID,
			ProgramID:    programID,
			Rating:       1500, // Начальный рейтинг ELO
		}

		if err := h.tournamentRepo.AddParticipant(r.Context(), participant); err != nil {
			h.log.Warn("Failed to add program as tournament participant (may already exist)",
				zap.Error(err),
				zap.String("program_id", programID.String()),
				zap.String("tournament_id", tournamentID.String()),
			)
			// Не возвращаем ошибку - программа уже создана, участие опционально
		} else {
			h.log.Info("Program registered as tournament participant",
				zap.String("program_id", programID.String()),
				zap.String("tournament_id", tournamentID.String()),
			)
		}
	}

	// ВАЖНО: Матчи НЕ создаются автоматически при загрузке программы!
	// Администратор должен вручную запустить матчи через кнопку "Run All Matches"
	// POST /api/v1/tournaments/{id}/run-matches

	h.log.Info("Program uploaded",
		zap.String("program_id", program.ID.String()),
		zap.String("user_id", userID.String()),
		zap.String("team_id", teamID.String()),
		zap.String("file", header.Filename),
		zap.Int("version", version),
	)

	writeJSON(w, http.StatusCreated, program)
}

// handleJSONCreate обрабатывает JSON запрос (обратная совместимость)
func (h *ProgramHandler) handleJSONCreate(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
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

// List обрабатывает получение списка программ текущего пользователя
// GET /api/v1/programs
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
// GET /api/v1/programs/:id
func (h *ProgramHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid program ID"))
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

	writeJSON(w, http.StatusOK, program)
}

// Update обрабатывает обновление программы
// PUT /api/v1/programs/:id
func (h *ProgramHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid program ID"))
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

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Info("Invalid request body", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithError(err))
		return
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

// Download скачивает файл программы
// GET /api/v1/programs/:id/download
func (h *ProgramHandler) Download(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid program ID"))
		return
	}

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

	// Проверяем существование файла
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		h.log.Error("Program file does not exist", zap.String("path", filePath))
		writeError(w, errors.ErrNotFound.WithMessage("program file not found on disk"))
		return
	}

	// Открываем файл
	file, err := os.Open(filePath)
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
		filename = program.Name + ext
	}

	// Устанавливаем заголовки для скачивания
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
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

// Delete обрабатывает удаление программы
// DELETE /api/v1/programs/:id
func (h *ProgramHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid program ID"))
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

// GetVersions получает все версии программ для команды и игры
// GET /api/v1/programs/versions?team_id=xxx&game_id=xxx
func (h *ProgramHandler) GetVersions(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	teamIDStr := r.URL.Query().Get("team_id")
	gameIDStr := r.URL.Query().Get("game_id")

	if teamIDStr == "" || gameIDStr == "" {
		writeError(w, errors.ErrInvalidInput.WithMessage("team_id and game_id are required"))
		return
	}

	teamID, err := uuid.Parse(teamIDStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid team_id"))
		return
	}

	gameID, err := uuid.Parse(gameIDStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid game_id"))
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

// validatePythonSyntax проверяет синтаксис Python файла с помощью py_compile
// Возвращает сообщение об ошибке или пустую строку, если синтаксис корректен
func validatePythonSyntax(filePath string) string {
	// Используем py_compile для проверки синтаксиса
	cmd := exec.Command("python3", "-m", "py_compile", filePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Парсим вывод ошибки
		errorMsg := strings.TrimSpace(string(output))
		if errorMsg == "" {
			errorMsg = "Синтаксическая ошибка в Python коде"
		}
		// Ограничиваем длину сообщения
		if len(errorMsg) > 500 {
			errorMsg = errorMsg[:500] + "..."
		}
		return errorMsg
	}
	return ""
}
