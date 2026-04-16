package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/domain/codescan"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
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
	// LangUnknown — sentinel для нераспознанного расширения файла.
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

// javaClassNameRe допускает только валидные Java identifier-ы.
// Строгое allowlist-регулярное выражение используется в compileIfNeeded для
// предотвращения shell-injection при сборке wrapper-скрипта.
var javaClassNameRe = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

// javaClassDeclRe выделяет имя первого top-level класса Java из исходника.
// Используется для UR bug_008: filename на диске (<hex>_<hex>_<hex>.java) не
// совпадает с declared class, поэтому javac отвергает `public class Main`,
// а wrapper пытается запустить несуществующий класс с hex-именем. Теперь
// Java-файл копируется в <ClassName>.java перед javac.
//
// Поддерживает `public class X`, `class X`, с модификаторами `final/abstract`.
// Не обрабатывает вложенные классы и edge-cases (multi-class в одном файле —
// тогда берётся первый).
var javaClassDeclRe = regexp.MustCompile(`(?m)^\s*(?:public\s+|final\s+|abstract\s+|static\s+)*class\s+([A-Za-z_$][A-Za-z0-9_$]*)`)

// extractJavaClassName возвращает имя первого top-level класса в Java-source,
// либо пустую строку если не найдено. Используется до вызова javac для
// переименования файла.
func extractJavaClassName(source string) string {
	// Игнорируем содержимое внутри /* */ и // комментариев.
	// Простой stripper: не обрабатывает nested-строковые литералы с "//" —
	// приемлемо для идентификации class declaration.
	cleaned := stripJavaComments(source)
	m := javaClassDeclRe.FindStringSubmatch(cleaned)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func stripJavaComments(s string) string {
	// Удаляем /* ... */ блоки
	blockRe := regexp.MustCompile(`/\*[\s\S]*?\*/`)
	s = blockRe.ReplaceAllString(s, "")
	// Удаляем // до конца строки
	lineRe := regexp.MustCompile(`//[^\n]*`)
	return lineRe.ReplaceAllString(s, "")
}

// shellSingleQuote квотирует строку для безопасного встраивания в /bin/sh.
// Заменяет каждую одинарную кавычку на '\” и оборачивает результат в '…'.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
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

// uploadFormData holds the parsed multipart form data for file upload.
type uploadFormData struct {
	fileContent  []byte
	filename     string
	name         string
	teamID       uuid.UUID
	tournamentID uuid.UUID
	gameID       uuid.UUID
}

// parseUploadForm parses the multipart form, extracts the file and form fields,
// and validates required fields. Returns nil on error (response already written).
func (h *ProgramHandler) parseUploadForm(w http.ResponseWriter, r *http.Request) *uploadFormData {
	// Парсим multipart form
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

// validateTeamAccess checks that the user is a member of the team and the team is not disqualified.
// Returns false on error (response already written).
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

// validateTournamentActive checks that the tournament is in active status.
// Returns false on error (response already written).
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

// validateUploadNotBlocked checks round completion and running matches in manual mode.
// In auto-round mode, uploads are never blocked. Returns false on error (response already written).
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

	// В авто-режиме загрузка НЕ блокируется матчами — новая программа будет подхвачена в следующем раунде.
	if autoRoundEnabled {
		return true
	}

	// В ручном режиме — сохраняем оригинальную логику блокировки.
	if !h.validateRoundNotCompleted(w, r, tournamentID, gameID) {
		return false
	}
	return h.validateNoRunningMatches(w, r, tournamentID, gameID)
}

// validateRoundNotCompleted checks that the round for this game has not been completed yet.
// Returns false if round is completed and upload is blocked (response already written).
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

// validateNoRunningMatches checks that no matches are currently running for the tournament.
// Returns false if matches are running and upload is blocked (response already written).
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

// saveUploadedFile writes the file content to disk, prepending a shebang for interpreted languages.
// Returns true on success, or false on error (response already written).
func (h *ProgramHandler) saveUploadedFile(w http.ResponseWriter, fileContent []byte, language, filePath string) bool {
	// Убеждаемся что директория существует (safety net для Docker volumes)
	if err := os.MkdirAll(h.uploadDir, 0755); err != nil {
		h.log.Error("Failed to ensure upload directory", zap.Error(err), zap.String("dir", h.uploadDir))
		writeError(w, errors.ErrInternal.WithMessage("не удалось сохранить файл: директория загрузок недоступна"))
		return false
	}

	// Сохраняем файл
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

	// Делаем файл исполняемым
	if err := os.Chmod(filePath, 0755); err != nil {
		h.log.Warn("Failed to make file executable", zap.Error(err), zap.String("path", filePath))
	}

	return true
}

// validateAndCompileProgram runs syntax checks and compilation for the uploaded program.
// Returns the executable path and an optional syntax/compilation error message.
func (h *ProgramHandler) validateAndCompileProgram(language, filePath string) (execPath string, syntaxError *string) {
	execPath = filePath

	// P2.18: defense-in-depth — сканируем исходник на подозрительные API-вызовы.
	// По-умолчанию только warn-лог; при CODESCAN_STRICT=true отказываем upload'у.
	if scanner := codescan.ScannerFor(language); scanner != nil {
		if src, err := os.ReadFile(filePath); err == nil {
			findings := scanner.Scan(string(src))
			if len(findings) > 0 {
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
					syntaxError = &msg
					return execPath, syntaxError
				}
			}
		}
	}

	// Проверяем синтаксис для всех поддерживаемых языков
	if errMsg := validateSyntax(language, filePath); errMsg != "" {
		syntaxError = &errMsg
		h.log.Info("Syntax error detected",
			zap.String("file", filePath),
			zap.String("language", language),
			zap.String("error", errMsg),
		)
		return execPath, syntaxError
	}

	// Компилируем программу для компилируемых языков
	if compiled, compileErr := compileIfNeeded(language, filePath, h.log); compileErr != "" {
		syntaxError = &compileErr
		h.log.Info("Compilation failed",
			zap.String("file", filePath),
			zap.String("language", language),
			zap.String("error", compileErr),
		)
	} else if compiled != "" {
		execPath = compiled
		h.log.Info("Program compiled",
			zap.String("source", filePath),
			zap.String("binary", compiled),
		)
	}

	return execPath, syntaxError
}

// registerTournamentParticipant registers the program as a tournament participant.
// Errors are logged but do not fail the upload.
func (h *ProgramHandler) registerTournamentParticipant(ctx context.Context, program *domain.Program, tournamentID uuid.UUID) {
	if h.tournamentRepo == nil {
		return
	}

	// Use program.ID (not local programID) since CreateWithAtomicVersion may regenerate it on retry
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
	// extension из language, а не raw из form.filename — это предотвращает
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

	// Проверяем синтаксис и компилируем
	execPath, syntaxError := h.validateAndCompileProgram(language, filePath)

	// Создаём запись в БД с атомарным назначением версии
	program := &domain.Program{
		ID:           programID,
		UserID:       userID,
		TeamID:       &form.teamID,
		TournamentID: &form.tournamentID,
		GameID:       &form.gameID,
		Name:         form.name,
		GameType:     "",       // Заполнится из game
		CodePath:     execPath, // Путь к исполняемому файлу (бинарник или скрипт)
		FilePath:     &filePath,
		Language:     language,
		ErrorMessage: syntaxError,
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

	// ВАЖНО: Матчи НЕ создаются автоматически при загрузке программы!
	// Администратор должен вручную запустить матчи через кнопку "Run All Matches"
	// POST /api/v1/tournaments/{id}/run-matches

	h.log.Info("Program uploaded",
		zap.String("program_id", program.ID.String()),
		zap.String("user_id", userID.String()),
		zap.String("team_id", form.teamID.String()),
		zap.String("file", form.filename),
		zap.Int("version", program.Version),
	)

	writeJSON(w, http.StatusCreated, program)
}

// syntaxCheckTimeout is the maximum time allowed for a syntax check command.
const syntaxCheckTimeout = 10 * time.Second

// runSyntaxCheck выполняет проверку синтаксиса с помощью внешней команды.
// Возвращает сообщение об ошибке или пустую строку, если синтаксис корректен.
func runSyntaxCheck(command string, args []string, defaultMsg string) string {
	_, lookErr := exec.LookPath(command)
	if lookErr != nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), syntaxCheckTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "Syntax check timed out"
		}
		errorMsg := strings.TrimSpace(string(output))
		if errorMsg == "" {
			errorMsg = defaultMsg
		}
		if len(errorMsg) > 500 {
			errorMsg = errorMsg[:500] + "..."
		}
		return errorMsg
	}
	return ""
}

// validateSyntax проверяет синтаксис файла в зависимости от языка
// Возвращает сообщение об ошибке или пустую строку, если синтаксис корректен
func validateSyntax(language, filePath string) string {
	switch language {
	case LangPython:
		return runSyntaxCheck("python3", []string{"-m", "py_compile", filePath}, "Синтаксическая ошибка в Python коде")
	case LangJavaScript:
		return runSyntaxCheck("node", []string{"--check", filePath}, "Синтаксическая ошибка в JavaScript коде")
	case LangRuby:
		return runSyntaxCheck("ruby", []string{"-c", filePath}, "Синтаксическая ошибка в Ruby коде")
	case LangPHP:
		return runSyntaxCheck("php", []string{"-l", filePath}, "Синтаксическая ошибка в PHP коде")
	case LangLua:
		return runSyntaxCheck("luac", []string{"-p", filePath}, "Синтаксическая ошибка в Lua коде")
	case LangC:
		return runSyntaxCheck("gcc", []string{"-fsyntax-only", filePath}, "Ошибка компиляции C")
	case LangCpp:
		return runSyntaxCheck("g++", []string{"-fsyntax-only", filePath}, "Ошибка компиляции C++")
	case LangJava:
		return runSyntaxCheck("javac", []string{"-Xlint:none", "-d", "/tmp", filePath}, "Ошибка компиляции Java")
	default:
		return ""
	}
}

// compileIfNeeded компилирует исходный код для компилируемых языков.
// Возвращает (путь к бинарнику, "") при успехе или ("", сообщение об ошибке) при ошибке.
// Для интерпретируемых языков возвращает ("", "").
func compileIfNeeded(language, sourcePath string, log *logger.Logger) (string, string) {
	outputPath := strings.TrimSuffix(sourcePath, filepath.Ext(sourcePath))

	var cmd *exec.Cmd
	switch language {
	case LangC:
		if _, err := exec.LookPath("gcc"); err != nil {
			log.Warn("gcc not found, skipping compilation")
			return "", ""
		}
		cmd = exec.Command("gcc", "-O2", "-o", outputPath, sourcePath, "-lm")
	case LangCpp:
		if _, err := exec.LookPath("g++"); err != nil {
			log.Warn("g++ not found, skipping compilation")
			return "", ""
		}
		cmd = exec.Command("g++", "-O2", "-o", outputPath, sourcePath)
	case LangGo:
		if _, err := exec.LookPath("go"); err != nil {
			log.Warn("go not found, skipping compilation")
			return "", ""
		}
		cmd = exec.Command("go", "build", "-o", outputPath, sourcePath)
	case LangRust:
		if _, err := exec.LookPath("rustc"); err != nil {
			log.Warn("rustc not found, skipping compilation")
			return "", ""
		}
		cmd = exec.Command("rustc", "-O", "-o", outputPath, sourcePath)
	case LangJava:
		if _, err := exec.LookPath("javac"); err != nil {
			log.Warn("javac not found, skipping compilation")
			return "", ""
		}
		// UR bug_008: имя файла на диске — <hex>_<hex>_<hex>.java (из UUID-ов),
		// но javac требует, чтобы `public class Foo` лежал в `Foo.java`,
		// а java-runtime ищет класс по declared name. Поэтому:
		// 1) Парсим declared class name из source.
		// 2) Переименовываем файл в <ClassName>.java в том же dir.
		// 3) Запускаем javac, wrapper использует declared name.
		srcBytes, readErr := os.ReadFile(sourcePath)
		if readErr != nil {
			return "", fmt.Sprintf("Не удалось прочитать Java-исходник: %s", readErr.Error())
		}
		className := extractJavaClassName(string(srcBytes))
		if className == "" {
			return "", "В Java-файле не найден объявление class X"
		}
		if !javaClassNameRe.MatchString(className) {
			return "", fmt.Sprintf("Недопустимое имя Java-класса: %q", className)
		}
		classDir := filepath.Dir(sourcePath)
		properPath := filepath.Join(classDir, className+".java")
		if properPath != sourcePath {
			if err := os.Rename(sourcePath, properPath); err != nil {
				return "", fmt.Sprintf("Не удалось переименовать Java-файл: %s", err.Error())
			}
		}
		// Java: компилируем .java → .class, затем создаём wrapper-скрипт
		javacCmd := exec.Command("javac", properPath)
		var javacStderr bytes.Buffer
		javacCmd.Stderr = &javacStderr
		if err := javacCmd.Run(); err != nil {
			errMsg := strings.TrimSpace(javacStderr.String())
			if errMsg == "" {
				errMsg = err.Error()
			}
			return "", fmt.Sprintf("Ошибка компиляции Java: %s", errMsg)
		}
		// Создаём wrapper-скрипт для запуска java -cp <dir> <ClassName>.
		// SECURITY: className уже прошёл regex-валидацию выше; classDir
		// квотируется через shellSingleQuote.
		wrapperPath := strings.TrimSuffix(properPath, ".java")
		wrapper := fmt.Sprintf("#!/bin/sh\nexec java -cp %s %s \"$@\"\n", shellSingleQuote(classDir), className)
		if err := os.WriteFile(wrapperPath, []byte(wrapper), 0755); err != nil {
			return "", fmt.Sprintf("Ошибка создания wrapper: %s", err.Error())
		}
		return wrapperPath, ""
	default:
		return "", ""
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return "", fmt.Sprintf("Ошибка компиляции: %s", errMsg)
	}

	// Делаем бинарник исполняемым
	_ = os.Chmod(outputPath, 0755)

	return outputPath, ""
}
