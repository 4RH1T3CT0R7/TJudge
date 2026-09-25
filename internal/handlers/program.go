package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmstu-itstech/tjudge/internal/middleware"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/internal/service/codescan"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ProgramRepository интерфейс для работы с программами
type ProgramRepository interface {
	Create(ctx context.Context, program *models.Program) error
	CreateWithAtomicVersion(ctx context.Context, program *models.Program) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Program, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Program, error)
	Update(ctx context.Context, program *models.Program) error
	Delete(ctx context.Context, id uuid.UUID) error
	CheckOwnership(ctx context.Context, programID, userID uuid.UUID) (bool, error)
	GetAllVersionsByTeamAndGame(ctx context.Context, teamID, gameID uuid.UUID) ([]*models.Program, error)
	ClearErrorMessages(ctx context.Context, tournamentID uuid.UUID) (int64, error)
}

// TournamentRepo - добавление участника и чтение турнира. это один и тот же
// репозиторий, держать под него два интерфейса было незачем
type TournamentRepo interface {
	AddParticipant(ctx context.Context, participant *models.TournamentParticipant) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Tournament, error)
}

// GameLookup отдаёт информацию об игре
type GameLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Game, error)
}

// MatchExistenceChecker проверяет наличие и активность матчей турнира
type MatchExistenceChecker interface {
	HasAnyRunningMatches(ctx context.Context, tournamentID uuid.UUID) (bool, error)
	GetActiveGameType(ctx context.Context, tournamentID uuid.UUID) (string, error)
}

// RoundCompletionChecker - состояние раунда игры (закрыт ли, включён ли авто-раунд)
type RoundCompletionChecker interface {
	IsRoundCompleted(ctx context.Context, tournamentID, gameID uuid.UUID) (bool, error)
	IsAutoRoundEnabled(ctx context.Context, tournamentID, gameID uuid.UUID) (bool, error)
}

// TeamMembershipChecker проверяет членство в команде и её дисквалификацию
type TeamMembershipChecker interface {
	IsUserInTeam(ctx context.Context, teamID, userID uuid.UUID) (bool, error)
	IsTeamDisqualified(ctx context.Context, teamID uuid.UUID) (bool, error)
}

// CompileEnqueuer ставит загруженную программу в очередь асинхронной
// компиляции (выполняется worker'ом в Docker-песочнице)
type CompileEnqueuer interface {
	Enqueue(ctx context.Context, programID uuid.UUID) error
}

// FIXME: файл-бог на ~1200 строк, а у NewProgramHandler аж 8 зависимостей —
// когда дойдут руки, стоит растащить загрузку/CRUD/листинг по под-сервисам
// и собирать конструктор из более крупных бандлов, а не из десятка интерфейсов

// ProgramHandler обрабатывает запросы программ
type ProgramHandler struct {
	programRepo    ProgramRepository
	tournamentRepo TournamentRepo
	gameLookup     GameLookup
	matchChecker   MatchExistenceChecker
	roundChecker   RoundCompletionChecker
	teamChecker    TeamMembershipChecker
	compileQueue   CompileEnqueuer
	uploadDir      string
	maxFileSize    int64
	log            *logger.Logger
}

// NewProgramHandler создаёт program handler и гарантирует наличие upload-директории
func NewProgramHandler(
	programRepo ProgramRepository,
	tournamentRepo TournamentRepo,
	gameLookup GameLookup,
	matchChecker MatchExistenceChecker,
	roundChecker RoundCompletionChecker,
	teamChecker TeamMembershipChecker,
	compileQueue CompileEnqueuer,
	uploadDir string,
	log *logger.Logger,
) *ProgramHandler {
	if uploadDir == "" {
		uploadDir = "/data/programs"
	}

	if err := os.MkdirAll(uploadDir, 0o750); err != nil {
		log.Error("Failed to create upload directory", zap.Error(err))
	}

	return &ProgramHandler{
		programRepo:    programRepo,
		tournamentRepo: tournamentRepo,
		gameLookup:     gameLookup,
		matchChecker:   matchChecker,
		roundChecker:   roundChecker,
		teamChecker:    teamChecker,
		compileQueue:   compileQueue,
		uploadDir:      uploadDir,
		maxFileSize:    10 * 1024 * 1024, // 10MB
		log:            log,
	}
}

// @Summary Создать программу
// @Description Создаёт новую программу. Поддерживает загрузку файла (multipart/form-data) и JSON
// @Tags programs
// @Accept multipart/form-data,json
// @Produce json
// @Param file formData file false "Файл программы"
// @Param team_id formData string false "Team ID" format(uuid)
// @Param tournament_id formData string false "Tournament ID" format(uuid)
// @Param game_id formData string false "Game ID" format(uuid)
// @Param name formData string false "Название программы"
// @Security BearerAuth
// @Success 201 {object} models.Program
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /programs [post]
func (h *ProgramHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	contentType := r.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "multipart/form-data") {
		h.handleFileUpload(w, r, userID)
		return
	}

	h.handleJSONCreate(w, r, userID)
}

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

	// path-traversal отбрасывается, абсолютные пути должны быть внутри upload-директории
	if req.CodePath != "" {
		cleaned := filepath.Clean(req.CodePath)
		if strings.Contains(cleaned, "..") {
			writeError(w, errors.ErrForbidden.WithMessage("invalid code path"))
			return
		}
		// абсолютные пути должны лежать внутри upload-директории
		if filepath.IsAbs(cleaned) {
			uploadDir := filepath.Clean(h.uploadDir)
			if !strings.HasPrefix(cleaned, uploadDir+string(filepath.Separator)) {
				writeError(w, errors.ErrForbidden.WithMessage("code path must be within the programs directory"))
				return
			}
		}
		req.CodePath = cleaned
	}

	program := &models.Program{
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

// @Summary Обновить программу
// @Description Обновляет метаданные программы (название, путь, язык)
// @Tags programs
// @Accept json
// @Produce json
// @Param id path string true "Program ID" format(uuid)
// @Param request body object{name=string,code_path=string,language=string} true "Данные для обновления"
// @Security BearerAuth
// @Success 200 {object} models.Program
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /programs/{id} [put]
func (h *ProgramHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	id, ok := parseUUIDParam(w, r, "id", "program")
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

	if req.CodePath != "" {
		cleaned := filepath.Clean(req.CodePath)
		if strings.Contains(cleaned, "..") {
			writeError(w, errors.ErrForbidden.WithMessage("invalid code path"))
			return
		}
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

// @Summary Удалить программу
// @Description Удаляет программу и связанный файл
// @Tags programs
// @Param id path string true "Program ID" format(uuid)
// @Security BearerAuth
// @Success 204 "Программа удалена"
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /programs/{id} [delete]
func (h *ProgramHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	id, ok := parseUUIDParam(w, r, "id", "program")
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

	program, err := h.programRepo.GetByID(r.Context(), id)
	if err != nil {
		h.log.LogError("Failed to get program", err)
		writeError(w, err)
		return
	}

	if err := h.programRepo.Delete(r.Context(), id); err != nil {
		h.log.LogError("Failed to delete program", err)
		writeError(w, err)
		return
	}

	// удаление файла, если он есть
	// #nosec G703 -- program.FilePath установлен сервером при upload
	// (generateProgramPath из UUID + controlled uploadDir), не пользователем.
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

// @Summary Очистить ошибки программ
// @Description Очищает все сообщения об ошибках для программ в турнире (только для админов)
// @Tags programs
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Security BearerAuth
// @Success 200 {object} object{cleared=int,message=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tournaments/{id}/programs/clear-errors [post]
func (h *ProgramHandler) ClearProgramErrors(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := parseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}

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

	writeJSON(w, http.StatusOK, map[string]any{
		"cleared": cleared,
		"message": fmt.Sprintf("Очищено %d ошибок", cleared),
	})
}

// поддерживаемые языки программирования: расширение файла определяет тулчейн,
// которым worker будет собирать/запускать бота в песочнице
const (
	LangPython     = "python"
	LangCpp        = "cpp"
	LangC          = "c"
	LangGo         = "go"
	LangRust       = "rust"
	LangJava       = "java"
	LangJavaScript = "javascript"
	LangRuby       = "ruby"
	LangPHP        = "php"
	LangLua        = "lua"
	// LangUnknown — sentinel для нераспознанного расширения файла;
	// вынесен в const по требованию goconst (встречается в 3+ местах)
	LangUnknown = "unknown"
)

// detectLanguage определяет язык программирования по расширению файла
func detectLanguage(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".py":
		return LangPython
	case ".cpp", ".cc", ".cxx":
		return LangCpp
	case ".c":
		return LangC
	case ".go":
		return LangGo
	case ".rs":
		return LangRust
	case ".java":
		return LangJava
	case ".js":
		return LangJavaScript
	case ".rb":
		return LangRuby
	case ".php":
		return LangPHP
	case ".lua":
		return LangLua
	default:
		return LangUnknown
	}
}

// canonicalExtension возвращает безопасное (hardcoded) расширение для языка;
// используется при генерации имён файлов на диске вместо небезопасного
// filepath.Ext(form.filename), которое может тащить shell-метасимволы
func canonicalExtension(language string) string {
	switch language {
	case LangPython:
		return ".py"
	case LangCpp:
		return ".cpp"
	case LangC:
		return ".c"
	case LangGo:
		return ".go"
	case LangRust:
		return ".rs"
	case LangJava:
		return ".java"
	case LangJavaScript:
		return ".js"
	case LangRuby:
		return ".rb"
	case LangPHP:
		return ".php"
	case LangLua:
		return ".lua"
	default:
		return ""
	}
}

// getShebang возвращает shebang для интерпретируемых языков
func getShebang(language string) string {
	switch language {
	case LangPython:
		return "#!/usr/bin/env python3\n"
	case LangJavaScript:
		return "#!/usr/bin/env node\n"
	case LangRuby:
		return "#!/usr/bin/env ruby\n"
	case LangPHP:
		return "#!/usr/bin/env php\n"
	case LangLua:
		return "#!/usr/bin/env lua\n"
	default:
		return ""
	}
}

// uploadFormData — разобранные данные multipart-формы загрузки файла
type uploadFormData struct {
	fileContent  []byte
	filename     string
	name         string
	teamID       uuid.UUID
	tournamentID uuid.UUID
	gameID       uuid.UUID
}

// parseUploadForm разбирает multipart-форму, вытаскивает файл и поля,
// валидирует обязательные team_id/tournament_id/game_id и читает содержимое
// файла целиком в память (размер ограничен maxFileSize). Возвращает nil при
// ошибке — ответ клиенту к этому моменту уже записан
func (h *ProgramHandler) parseUploadForm(w http.ResponseWriter, r *http.Request) *uploadFormData {
	// парсинг multipart form
	// #nosec G120 -- h.maxFileSize ограничивает размер form, плюс routes.go
	// применяет middleware.MaxBodySize(10 << 20) на /programs роуте. Double-bound.
	if err := r.ParseMultipartForm(h.maxFileSize); err != nil {
		h.log.Info("Failed to parse multipart form", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithMessage("file too large or invalid form"))
		return nil
	}

	// сам файл-бот
	file, header, err := r.FormFile("file")
	if err != nil {
		h.log.Info("Failed to get file from form", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithMessage("file is required"))
		return nil
	}
	defer file.Close()

	// остальные поля формы
	teamIDStr := r.FormValue("team_id")
	tournamentIDStr := r.FormValue("tournament_id")
	gameIDStr := r.FormValue("game_id")
	name := r.FormValue("name")

	// без привязки к команде/турниру/игре программа бессмысленна
	if teamIDStr == "" || tournamentIDStr == "" || gameIDStr == "" {
		writeError(w, errors.ErrInvalidInput.WithMessage("team_id, tournament_id and game_id are required"))
		return nil
	}

	teamID, err := uuid.Parse(teamIDStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid team_id"))
		return nil
	}

	tournamentID, err := uuid.Parse(tournamentIDStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid tournament_id"))
		return nil
	}

	gameID, err := uuid.Parse(gameIDStr)
	if err != nil {
		writeError(w, errors.ErrInvalidInput.WithMessage("invalid game_id"))
		return nil
	}

	// чтение всего содержимого файла в память (ограничено maxFileSize)
	fileContent, err := io.ReadAll(file)
	if err != nil {
		h.log.Error("Failed to read uploaded file", zap.Error(err))
		writeError(w, errors.ErrInternal.WithMessage("не удалось прочитать файл"))
		return nil
	}

	// если имя не задано — берётся имя загруженного файла
	if name == "" {
		name = header.Filename
	}

	return &uploadFormData{
		fileContent:  fileContent,
		filename:     header.Filename,
		name:         name,
		teamID:       teamID,
		tournamentID: tournamentID,
		gameID:       gameID,
	}
}

// validateTeamAccess требует членства пользователя в команде и отсутствия
// дисквалификации; при nil-checker'е фейлится закрыто. false — ответ записан
func (h *ProgramHandler) validateTeamAccess(w http.ResponseWriter, r *http.Request, teamID, userID uuid.UUID) bool {
	if h.teamChecker == nil {
		h.log.Error("Team membership checker not configured")
		writeError(w, errors.ErrInternal.WithMessage("authorization service unavailable"))
		return false
	}

	isMember, err := h.teamChecker.IsUserInTeam(r.Context(), teamID, userID)
	if err != nil {
		h.log.LogError("Failed to check team membership", err)
		writeError(w, errors.ErrInternal.WithMessage("failed to verify team membership"))
		return false
	}
	if !isMember {
		writeError(w, errors.ErrForbidden.WithMessage("you are not a member of this team"))
		return false
	}

	disqualified, err := h.teamChecker.IsTeamDisqualified(r.Context(), teamID)
	if err != nil {
		h.log.LogError("Failed to check team disqualification", err)
		writeError(w, errors.ErrInternal.WithMessage("failed to verify team status"))
		return false
	}
	if disqualified {
		writeError(w, errors.ErrForbidden.WithMessage("команда дисквалифицирована"))
		return false
	}

	return true
}

// validateTournamentActive пускает загрузку только в активный турнир
func (h *ProgramHandler) validateTournamentActive(w http.ResponseWriter, r *http.Request, tournamentID uuid.UUID) bool {
	if h.tournamentRepo == nil {
		return true
	}

	t, err := h.tournamentRepo.GetByID(r.Context(), tournamentID)
	if err != nil {
		h.log.LogError("Failed to get tournament status", err)
		writeError(w, errors.ErrInternal.WithMessage("failed to verify tournament status"))
		return false
	}
	if t.Status != models.TournamentActive {
		writeError(w, errors.ErrForbidden.WithMessage("загрузка программ запрещена: турнир ещё не начался"))
		return false
	}

	return true
}

// validateUploadNotBlocked в ручном режиме блокирует загрузку завершённым
// раундом или идущими матчами; в авто-режиме загрузки не блокируются, т.к.
// новая программа будет подхвачена следующим раундом. false — ответ записан
func (h *ProgramHandler) validateUploadNotBlocked(w http.ResponseWriter, r *http.Request, tournamentID, gameID uuid.UUID) bool {
	// проверка, включён ли авто-раунд для этой игры
	autoRoundEnabled := false
	if h.roundChecker != nil {
		var autoRoundErr error
		autoRoundEnabled, autoRoundErr = h.roundChecker.IsAutoRoundEnabled(r.Context(), tournamentID, gameID)
		if autoRoundErr != nil {
			h.log.Warn("Failed to check auto-round status, defaulting to manual mode",
				zap.Error(autoRoundErr),
				zap.String("tournament_id", tournamentID.String()),
				zap.String("game_id", gameID.String()),
			)
		}
	}

	// в авто-режиме загрузка не блокируется матчами
	if autoRoundEnabled {
		return true
	}

	// в ручном режиме сохраняется оригинальная логика блокировки
	if !h.validateRoundNotCompleted(w, r, tournamentID, gameID) {
		return false
	}
	return h.validateNoRunningMatches(w, r, tournamentID, gameID)
}

// validateRoundNotCompleted блокирует загрузку, если раунд игры уже закрыт;
// при невозможности проверить статус — без помех (fail-open)
func (h *ProgramHandler) validateRoundNotCompleted(w http.ResponseWriter, r *http.Request, tournamentID, gameID uuid.UUID) bool {
	if h.roundChecker == nil {
		return true
	}

	roundCompleted, err := h.roundChecker.IsRoundCompleted(r.Context(), tournamentID, gameID)
	if err != nil {
		h.log.LogError("Failed to check round completion", err,
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_id", gameID.String()),
		)
		// проход дальше, если не смогли проверить статус раунда
		return true
	}

	if roundCompleted {
		h.log.Info("Upload blocked: round already completed",
			zap.String("tournament_id", tournamentID.String()),
			zap.String("game_id", gameID.String()),
		)
		writeError(w, errors.ErrForbidden.WithMessage("загрузка программ запрещена: раунд уже завершён для этой игры"))
		return false
	}

	return true
}

// validateNoRunningMatches блокирует загрузку, пока в турнире крутятся матчи
// другой игры — иначе рейтинги посчитались бы на несогласованном наборе ботов
func (h *ProgramHandler) validateNoRunningMatches(w http.ResponseWriter, r *http.Request, tournamentID, gameID uuid.UUID) bool {
	if h.matchChecker == nil {
		return true
	}

	hasRunning, err := h.matchChecker.HasAnyRunningMatches(r.Context(), tournamentID)
	if err != nil {
		h.log.LogError("Failed to check running matches", err)
		writeError(w, errors.ErrInternal.WithMessage("failed to verify match status"))
		return false
	}

	if !hasRunning {
		return true
	}

	// подтягивается название активной игры для информативного сообщения
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
	return false
}

// saveUploadedFile пишет исходник на диск, добавляя shebang интерпретируемым
// языкам и делая файл исполняемым для песочницы. false — ответ уже записан
func (h *ProgramHandler) saveUploadedFile(w http.ResponseWriter, fileContent []byte, language, filePath string) bool {
	// на всякий случай директория гарантируется (safety net для Docker volumes)
	// 0750 — group read/execute, other — нет; appuser внутри worker'а
	// единственный потребитель этой директории
	if err := os.MkdirAll(h.uploadDir, 0o750); err != nil {
		h.log.Error("Failed to ensure upload directory", zap.Error(err), zap.String("dir", h.uploadDir))
		writeError(w, errors.ErrInternal.WithMessage("не удалось сохранить файл: директория загрузок недоступна"))
		return false
	}

	// сохранение файла
	// #nosec G304 -- filePath формируется из h.uploadDir + {teamID/gameID/programID}[:8] +
	// canonicalExtension(language); ни один компонент не контролируется пользователем
	// напрямую (UUID-prefixes, hardcoded ext). Path-traversal невозможен.
	dst, err := os.Create(filePath)
	if err != nil {
		h.log.Error("Failed to create file", zap.Error(err), zap.String("path", filePath))
		writeError(w, errors.ErrInternal.WithMessage("не удалось сохранить файл"))
		return false
	}
	defer dst.Close()

	// дописывается shebang интерпретируемым языкам, если его ещё нет
	shebang := getShebang(language)
	if shebang != "" && !bytes.HasPrefix(fileContent, []byte("#!")) {
		if _, err := dst.WriteString(shebang); err != nil {
			h.log.Error("Failed to write shebang", zap.Error(err))
			os.Remove(filePath)
			writeError(w, errors.ErrInternal.WithMessage("не удалось сохранить файл"))
			return false
		}
	}

	if _, err := dst.Write(fileContent); err != nil {
		h.log.Error("Failed to write file", zap.Error(err))
		// подчищается частично записанный файл
		os.Remove(filePath)
		writeError(w, errors.ErrInternal.WithMessage("failed to save file"))
		return false
	}

	// файл делается исполняемым
	// #nosec G302 -- бот-программа должна быть executable внутри Docker-sandbox'а;
	// 0o750 даёт rwx только owner+group (appuser + docker), other - 0.
	if err := os.Chmod(filePath, 0o750); err != nil {
		h.log.Warn("Failed to make file executable", zap.Error(err), zap.String("path", filePath))
	}

	return true
}

// validateProgramSource прогоняет исходник через статический анализ (codescan)
// и при CODESCAN_STRICT=true с запрещёнными API-вызовами возвращает сообщение
// об отказе, иначе nil.
//
// проверка синтаксиса и компиляция тут не выполняются: недоверенный код никогда
// не должен попадать в тулчейны на хосте API-процесса. программа создаётся в
// статусе compiling, а собирает её worker уже в Docker-песочнице
func (h *ProgramHandler) validateProgramSource(language, filePath string) *string {
	scanner := codescan.ScannerFor(language)
	if scanner == nil {
		return nil
	}

	// #nosec G304 -- тот же filePath что собран выше из uuid, не пользовательский ввод
	src, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	findings := scanner.Scan(string(src))
	if len(findings) == 0 {
		return nil
	}

	for _, f := range findings {
		h.log.Warn("Code scan finding",
			zap.String("file", filePath),
			zap.String("language", language),
			zap.Int("line", f.Line),
			zap.String("level", string(f.Level)),
			zap.String("pattern", f.Pattern),
			zap.String("message", f.Message),
		)
	}

	if os.Getenv("CODESCAN_STRICT") == "true" && codescan.HasForbidden(findings) {
		msg := "Обнаружены запрещённые API-вызовы; загрузка отклонена (CODESCAN_STRICT)"
		return &msg
	}

	return nil
}

// registerTournamentParticipant регистрирует программу как участника турнира;
// ошибки логируются, но upload не проваливают (участие опционально)
func (h *ProgramHandler) registerTournamentParticipant(ctx context.Context, program *models.Program, tournamentID uuid.UUID) {
	if h.tournamentRepo == nil {
		return
	}

	// берётся program.ID (а не локальный programID), т.к. CreateWithAtomicVersion может перегенерировать его при retry
	participant := &models.TournamentParticipant{
		ID:           uuid.New(),
		TournamentID: tournamentID,
		ProgramID:    program.ID,
		Rating:       1500, // стартовый рейтинг ELO
	}

	if err := h.tournamentRepo.AddParticipant(ctx, participant); err != nil {
		h.log.Warn("Failed to add program as tournament participant (may already exist)",
			zap.Error(err),
			zap.String("program_id", program.ID.String()),
			zap.String("tournament_id", tournamentID.String()),
		)
		// ошибка не возвращается — программа уже создана, участие опционально
	} else {
		h.log.Info("Program registered as tournament participant",
			zap.String("program_id", program.ID.String()),
			zap.String("tournament_id", tournamentID.String()),
		)
	}
}

// handleFileUpload — основной путь загрузки бота: multipart -> проверки доступа
// и блокировок -> запись на диск -> codescan -> запись в БД -> очередь компиляции.
//
// статусная модель: программа рождается в compiling и уходит в очередь; worker
// собирает её в песочнице и переводит в ready или failed. исключение — строгий
// codescan (CODESCAN_STRICT): при запрещённых API-вызовах компилировать нечего,
// и запись сразу создаётся в failed
func (h *ProgramHandler) handleFileUpload(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	// размер тела жёстко ограничивается ещё до чтения формы
	r.Body = http.MaxBytesReader(w, r.Body, h.maxFileSize)

	// парсинг формы и извлечение данных
	form := h.parseUploadForm(w, r)
	if form == nil {
		return
	}

	// пользователь должен состоять в команде, команда — не дисквалифицирована
	if !h.validateTeamAccess(w, r, form.teamID, userID) {
		return
	}

	// турнир должен быть активен
	if !h.validateTournamentActive(w, r, form.tournamentID) {
		return
	}

	// блокировки загрузки (завершённый раунд, идущие матчи)
	if !h.validateUploadNotBlocked(w, r, form.tournamentID, form.gameID) {
		return
	}

	// определение языка по расширению
	language := detectLanguage(form.filename)
	if language == LangUnknown {
		writeError(w, errors.ErrInvalidInput.WithMessage("unsupported file extension"))
		return
	}

	// уникальный путь для файла. берётся канонический (hardcoded) extension из
	// language, а не raw из form.filename — так в имя файла не пролезут shell-
	// метасимволы (напр. "Test.java;rm -rf /")
	programID := uuid.New()
	ext := canonicalExtension(language)
	if ext == "" {
		writeError(w, errors.ErrInvalidInput.WithMessage("unsupported file extension"))
		return
	}
	fileName := fmt.Sprintf("%s_%s_%s%s", form.teamID.String()[:8], form.gameID.String()[:8], programID.String()[:8], ext)
	filePath := filepath.Join(h.uploadDir, fileName)

	// сохранение файла на диск
	if !h.saveUploadedFile(w, form.fileContent, language, filePath) {
		return
	}

	// статический анализ исходника (codescan); компиляция и проверка синтаксиса
	// идут асинхроно в Docker-песочнице worker'а
	scanError := h.validateProgramSource(language, filePath)

	status := models.ProgramCompiling
	if scanError != nil {
		// запрещённые API при CODESCAN_STRICT: компилировать нечего
		status = models.ProgramFailed
	}

	// запись в БД с атомарным назначением версии
	program := &models.Program{
		ID:           programID,
		UserID:       userID,
		TeamID:       &form.teamID,
		TournamentID: &form.tournamentID,
		GameID:       &form.gameID,
		Name:         form.name,
		GameType:     "",       // заполнится из game
		CodePath:     filePath, // исходник; после компиляции worker заменит на бинарник
		FilePath:     &filePath,
		Language:     language,
		Status:       status,
		ErrorMessage: scanError,
	}

	if err := h.programRepo.CreateWithAtomicVersion(r.Context(), program); err != nil {
		h.log.LogError("Failed to create program", err)
		// удаление загруженного файла при ошибке
		os.Remove(filePath)
		writeError(w, err)
		return
	}

	// программа автоматически регистрируется как участник турнира
	h.registerTournamentParticipant(r.Context(), program, form.tournamentID)

	// программа ставится в очередь компиляции. при ошибке enqueue ничего не
	// теряется: compile-worker периодически возвращает в очередь программы,
	// зависшие в статусе compiling
	if status == models.ProgramCompiling && h.compileQueue != nil {
		if err := h.compileQueue.Enqueue(r.Context(), program.ID); err != nil {
			h.log.LogError("Failed to enqueue compile task, stuck-recovery will retry", err,
				zap.String("program_id", program.ID.String()),
			)
		}
	}

	// важно: матчи не создаются автоматически при загрузке программы!
	// админ запускает их вручную кнопкой "Run All Matches"
	// POST /api/v1/tournaments/{id}/run-matches.

	h.log.Info("Program uploaded",
		zap.String("program_id", program.ID.String()),
		zap.String("user_id", userID.String()),
		zap.String("team_id", form.teamID.String()),
		zap.String("file", form.filename),
		zap.String("status", string(program.Status)),
		zap.Int("version", program.Version),
	)

	writeJSON(w, http.StatusCreated, program)
}

// @Summary Мои программы
// @Description Возвращает список программ текущего пользователя
// @Tags programs
// @Produce json
// @Security BearerAuth
// @Success 200 {array} models.Program
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

// @Summary Получить программу
// @Description Возвращает программу по ID (владелец или админ)
// @Tags programs
// @Produce json
// @Param id path string true "Program ID" format(uuid)
// @Security BearerAuth
// @Success 200 {object} models.Program
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

	id, ok := parseUUIDParam(w, r, "id", "program")
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

	// админы видят любую программу, остальные — только свои
	userRole, _ := r.Context().Value(middleware.RoleKey).(models.Role)
	if userRole != models.RoleAdmin && program.UserID != userID {
		writeError(w, errors.ErrForbidden.WithMessage("you don't own this program"))
		return
	}

	writeJSON(w, http.StatusOK, program)
}

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

	id, ok := parseUUIDParam(w, r, "id", "program")
	if !ok {
		return
	}

	// админы скачивают любую программу
	userRole, _ := r.Context().Value(middleware.RoleKey).(models.Role)
	if userRole != models.RoleAdmin {
		// остальным проверяется владение
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

	// файл должен быть привязан к записи
	if program.FilePath == nil || *program.FilePath == "" {
		writeError(w, errors.ErrNotFound.WithMessage("program file not found"))
		return
	}

	filePath := *program.FilePath

	// путь файла обязан лежать внутри upload-директории
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

	// проверка, что файл существует (absFilePath для defense-in-depth)
	// #nosec G703 -- absFilePath провалидирован через HasPrefix(absUploadDir) выше.
	if _, err := os.Stat(absFilePath); os.IsNotExist(err) {
		h.log.Error("Program file does not exist", zap.String("path", absFilePath))
		writeError(w, errors.ErrNotFound.WithMessage("program file not found on disk"))
		return
	}

	// открытие файла
	// #nosec G304 G703 -- absFilePath провалидирован через HasPrefix(absUploadDir)
	// выше; path-traversal невозможен.
	file, err := os.Open(absFilePath)
	if err != nil {
		h.log.Error("Failed to open file", zap.Error(err))
		writeError(w, errors.ErrInternal.WithMessage("failed to read file"))
		return
	}
	defer file.Close()

	// имя файла для скачивания
	filename := filepath.Base(filePath)
	if program.Name != "" {
		ext := filepath.Ext(filePath)
		if filepath.Ext(program.Name) == ext {
			filename = program.Name
		} else {
			filename = program.Name + ext
		}
	}

	// имя файла санитизируется для безопасного использования в заголовке
	safeName := strings.Map(func(r rune) rune {
		if r == '"' || r == '\\' || r == '\r' || r == '\n' {
			return '_'
		}
		return r
	}, filename)

	// заголовки скачивания
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", safeName))
	w.Header().Set("Content-Type", "application/octet-stream")

	// файл стримится в response
	if _, err := io.Copy(w, file); err != nil {
		h.log.Error("Failed to send file", zap.Error(err))
		// уже начали отправлять — ошибку вернуть уже нельзя
		return
	}

	h.log.Info("Program downloaded",
		zap.String("program_id", id.String()),
		zap.String("user_id", userID.String()),
	)
}

// @Summary Версии программ
// @Description Возвращает все версии программ для команды и игры
// @Tags programs
// @Produce json
// @Param team_id query string true "Team ID" format(uuid)
// @Param game_id query string true "Game ID" format(uuid)
// @Security BearerAuth
// @Success 200 {array} models.Program
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

	teamID, ok := parseQueryUUID(w, r, "team_id")
	if !ok {
		return
	}

	gameID, ok := parseQueryUUID(w, r, "game_id")
	if !ok {
		return
	}

	programs, err := h.programRepo.GetAllVersionsByTeamAndGame(r.Context(), teamID, gameID)
	if err != nil {
		h.log.LogError("Failed to get program versions", err)
		writeError(w, err)
		return
	}

	// достаточно, чтобы хотя бы одна версия принадлежала пользователю
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
