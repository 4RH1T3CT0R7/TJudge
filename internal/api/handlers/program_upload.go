package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/domain/codescan"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Константы поддерживаемых языков программирования
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
	// LangUnknown - sentinel для нераспознанного расширения файла.
	// Выделен в const по требованию goconst (используется в 3+ местах).
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

// canonicalExtension возвращает безопасное (hardcoded) расширение для языка.
// Используется для генерации имён файлов на диске вместо небезопасного
// filepath.Ext(form.filename), которое может содержать shell-метасимволы.
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

// uploadFormData содержит разобранные данные multipart-формы для загрузки файла.
type uploadFormData struct {
	fileContent  []byte
	filename     string
	name         string
	teamID       uuid.UUID
	tournamentID uuid.UUID
	gameID       uuid.UUID
}

// parseUploadForm разбирает multipart-форму, извлекает файл и поля формы,
// валидирует обязательные поля. Возвращает nil при ошибке (ответ уже записан).
func (h *ProgramHandler) parseUploadForm(w http.ResponseWriter, r *http.Request) *uploadFormData {
	// Парсим multipart form.
	// #nosec G120 -- h.maxFileSize ограничивает размер form, плюс routes.go
	// применяет middleware.MaxBodySize(10 << 20) на /programs роуте. Double-bound.
	if err := r.ParseMultipartForm(h.maxFileSize); err != nil {
		h.log.Info("Failed to parse multipart form", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithMessage("file too large or invalid form"))
		return nil
	}

	// Получаем файл
	file, header, err := r.FormFile("file")
	if err != nil {
		h.log.Info("Failed to get file from form", zap.Error(err))
		writeError(w, errors.ErrInvalidInput.WithMessage("file is required"))
		return nil
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

	// Читаем всё содержимое файла в память (ограничено maxFileSize)
	fileContent, err := io.ReadAll(file)
	if err != nil {
		h.log.Error("Failed to read uploaded file", zap.Error(err))
		writeError(w, errors.ErrInternal.WithMessage("не удалось прочитать файл"))
		return nil
	}

	// Если имя не указано, используем имя файла
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

// validateTeamAccess проверяет, что пользователь состоит в команде и команда не дисквалифицирована.
// Возвращает false при ошибке (ответ уже записан).
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

// validateTournamentActive проверяет, что турнир в активном статусе.
// Возвращает false при ошибке (ответ уже записан).
func (h *ProgramHandler) validateTournamentActive(w http.ResponseWriter, r *http.Request, tournamentID uuid.UUID) bool {
	if h.tournamentStatus == nil {
		return true
	}

	t, err := h.tournamentStatus.GetByID(r.Context(), tournamentID)
	if err != nil {
		h.log.LogError("Failed to get tournament status", err)
		writeError(w, errors.ErrInternal.WithMessage("failed to verify tournament status"))
		return false
	}
	if t.Status != domain.TournamentActive {
		writeError(w, errors.ErrForbidden.WithMessage("загрузка программ запрещена: турнир ещё не начался"))
		return false
	}

	return true
}

// validateUploadNotBlocked проверяет завершение раунда и выполняющиеся матчи в ручном режиме.
// В авто-режиме загрузки не блокируются. Возвращает false при ошибке (ответ уже записан).
func (h *ProgramHandler) validateUploadNotBlocked(w http.ResponseWriter, r *http.Request, tournamentID, gameID uuid.UUID) bool {
	// Проверяем, включён ли авто-раунд для этой игры
	autoRoundEnabled := false
	if h.autoRoundChecker != nil {
		var autoRoundErr error
		autoRoundEnabled, autoRoundErr = h.autoRoundChecker.IsAutoRoundEnabled(r.Context(), tournamentID, gameID)
		if autoRoundErr != nil {
			h.log.Warn("Failed to check auto-round status, defaulting to manual mode",
				zap.Error(autoRoundErr),
				zap.String("tournament_id", tournamentID.String()),
				zap.String("game_id", gameID.String()),
			)
		}
	}

	// В авто-режиме загрузка НЕ блокируется матчами - новая программа будет подхвачена в следующем раунде.
	if autoRoundEnabled {
		return true
	}

	// В ручном режиме сохраняем оригинальную логику блокировки.
	if !h.validateRoundNotCompleted(w, r, tournamentID, gameID) {
		return false
	}
	return h.validateNoRunningMatches(w, r, tournamentID, gameID)
}

// validateRoundNotCompleted проверяет, что раунд для этой игры ещё не завершён.
// Возвращает false, если раунд завершён и загрузка заблокирована (ответ уже записан).
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
		// Продолжаем, если не можем проверить статус раунда
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

// validateNoRunningMatches проверяет, что в турнире сейчас нет выполняющихся матчей.
// Возвращает false, если матчи выполняются и загрузка заблокирована (ответ уже записан).
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
	return false
}

// saveUploadedFile записывает содержимое файла на диск, добавляя shebang для интерпретируемых языков.
// Возвращает true при успехе или false при ошибке (ответ уже записан).
func (h *ProgramHandler) saveUploadedFile(w http.ResponseWriter, fileContent []byte, language, filePath string) bool {
	// Убеждаемся, что директория существует (safety net для Docker volumes).
	// 0750 - group read/execute, other - нет; appuser внутри worker'а
	// единственный потребитель этой директории.
	if err := os.MkdirAll(h.uploadDir, 0o750); err != nil {
		h.log.Error("Failed to ensure upload directory", zap.Error(err), zap.String("dir", h.uploadDir))
		writeError(w, errors.ErrInternal.WithMessage("не удалось сохранить файл: директория загрузок недоступна"))
		return false
	}

	// Сохраняем файл.
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

	// Добавляем shebang для интерпретируемых языков (если его нет)
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
		// Удаляем частично записанный файл
		os.Remove(filePath)
		writeError(w, errors.ErrInternal.WithMessage("failed to save file"))
		return false
	}

	// Делаем файл исполняемым.
	// #nosec G302 -- бот-программа должна быть executable внутри Docker-sandbox'а;
	// 0o750 даёт rwx только owner+group (appuser + docker), other - 0.
	if err := os.Chmod(filePath, 0o750); err != nil {
		h.log.Warn("Failed to make file executable", zap.Error(err), zap.String("path", filePath))
	}

	return true
}

// validateProgramSource выполняет статический анализ исходника (codescan).
// Возвращает сообщение об отказе при CODESCAN_STRICT=true и запрещённых
// API-вызовах, иначе nil.
//
// Проверка синтаксиса и компиляция здесь НЕ выполняются: недоверенный код
// никогда не должен попадать в тулчейны на хосте API-процесса. Программа
// создаётся в статусе compiling, и worker собирает её в Docker-песочнице.
func (h *ProgramHandler) validateProgramSource(language, filePath string) *string {
	scanner := codescan.ScannerFor(language)
	if scanner == nil {
		return nil
	}

	// #nosec G304 -- filePath тот же, что мы сами создали выше (UUID-based).
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

// registerTournamentParticipant регистрирует программу как участника турнира.
// Ошибки логируются, но не проваливают upload.
func (h *ProgramHandler) registerTournamentParticipant(ctx context.Context, program *domain.Program, tournamentID uuid.UUID) {
	if h.tournamentRepo == nil {
		return
	}

	// Используем program.ID (а не локальный programID), т.к. CreateWithAtomicVersion может перегенерировать его при retry.
	participant := &domain.TournamentParticipant{
		ID:           uuid.New(),
		TournamentID: tournamentID,
		ProgramID:    program.ID,
		Rating:       1500, // Начальный рейтинг ELO
	}

	if err := h.tournamentRepo.AddParticipant(ctx, participant); err != nil {
		h.log.Warn("Failed to add program as tournament participant (may already exist)",
			zap.Error(err),
			zap.String("program_id", program.ID.String()),
			zap.String("tournament_id", tournamentID.String()),
		)
		// Не возвращаем ошибку - программа уже создана, участие опционально
	} else {
		h.log.Info("Program registered as tournament participant",
			zap.String("program_id", program.ID.String()),
			zap.String("tournament_id", tournamentID.String()),
		)
	}
}

// handleFileUpload обрабатывает загрузку файла
func (h *ProgramHandler) handleFileUpload(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	// Ограничиваем размер файла
	r.Body = http.MaxBytesReader(w, r.Body, h.maxFileSize)

	// Парсим форму и извлекаем данные
	form := h.parseUploadForm(w, r)
	if form == nil {
		return
	}

	// Проверяем доступ команды
	if !h.validateTeamAccess(w, r, form.teamID, userID) {
		return
	}

	// Проверяем статус турнира
	if !h.validateTournamentActive(w, r, form.tournamentID) {
		return
	}

	// Проверяем блокировки загрузки (раунды, матчи)
	if !h.validateUploadNotBlocked(w, r, form.tournamentID, form.gameID) {
		return
	}

	// Определяем язык по расширению
	language := detectLanguage(form.filename)
	if language == LangUnknown {
		writeError(w, errors.ErrInvalidInput.WithMessage("unsupported file extension"))
		return
	}

	// Создаём уникальный путь для файла. Используем канонический (hardcoded)
	// extension из language, а не raw из form.filename; это предотвращает
	// внедрение shell-метасимволов в имя файла (e.g. "Test.java;rm -rf /").
	programID := uuid.New()
	ext := canonicalExtension(language)
	if ext == "" {
		writeError(w, errors.ErrInvalidInput.WithMessage("unsupported file extension"))
		return
	}
	fileName := fmt.Sprintf("%s_%s_%s%s", form.teamID.String()[:8], form.gameID.String()[:8], programID.String()[:8], ext)
	filePath := filepath.Join(h.uploadDir, fileName)

	// Сохраняем файл на диск
	if !h.saveUploadedFile(w, form.fileContent, language, filePath) {
		return
	}

	// Статический анализ исходника (codescan). Компиляция и проверка
	// синтаксиса выполняются асинхронно в Docker-песочнице worker'а.
	scanError := h.validateProgramSource(language, filePath)

	status := domain.ProgramCompiling
	if scanError != nil {
		// Запрещённые API при CODESCAN_STRICT: компилировать нечего.
		status = domain.ProgramFailed
	}

	// Создаём запись в БД с атомарным назначением версии
	program := &domain.Program{
		ID:           programID,
		UserID:       userID,
		TeamID:       &form.teamID,
		TournamentID: &form.tournamentID,
		GameID:       &form.gameID,
		Name:         form.name,
		GameType:     "",       // Заполнится из game
		CodePath:     filePath, // Исходник; после компиляции worker заменит на бинарник
		FilePath:     &filePath,
		Language:     language,
		Status:       status,
		ErrorMessage: scanError,
	}

	if err := h.programRepo.CreateWithAtomicVersion(r.Context(), program); err != nil {
		h.log.LogError("Failed to create program", err)
		// Удаляем загруженный файл при ошибке
		os.Remove(filePath)
		writeError(w, err)
		return
	}

	// Автоматически регистрируем программу как участника турнира
	h.registerTournamentParticipant(r.Context(), program, form.tournamentID)

	// Ставим программу в очередь компиляции. При ошибке enqueue ничего не
	// теряется: compile-worker периодически возвращает в очередь программы,
	// зависшие в статусе compiling.
	if status == domain.ProgramCompiling && h.compileQueue != nil {
		if err := h.compileQueue.Enqueue(r.Context(), program.ID); err != nil {
			h.log.LogError("Failed to enqueue compile task, stuck-recovery will retry", err,
				zap.String("program_id", program.ID.String()),
			)
		}
	}

	// ВАЖНО: Матчи НЕ создаются автоматически при загрузке программы!
	// Администратор должен вручную запустить матчи через кнопку "Run All Matches"
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
