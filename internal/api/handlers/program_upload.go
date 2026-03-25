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
	"strings"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

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

	// Проверяем что пользователь является членом команды
	if h.teamChecker == nil {
		h.log.Error("Team membership checker not configured")
		writeError(w, errors.ErrInternal.WithMessage("authorization service unavailable"))
		return
	}
	isMember, err := h.teamChecker.IsUserInTeam(r.Context(), teamID, userID)
	if err != nil {
		h.log.LogError("Failed to check team membership", err)
		writeError(w, errors.ErrInternal.WithMessage("failed to verify team membership"))
		return
	}
	if !isMember {
		writeError(w, errors.ErrForbidden.WithMessage("you are not a member of this team"))
		return
	}

	// Проверяем, что команда не дисквалифицирована
	disqualified, err := h.teamChecker.IsTeamDisqualified(r.Context(), teamID)
	if err != nil {
		h.log.LogError("Failed to check team disqualification", err)
		writeError(w, errors.ErrInternal.WithMessage("failed to verify team status"))
		return
	}
	if disqualified {
		writeError(w, errors.ErrForbidden.WithMessage("команда дисквалифицирована"))
		return
	}

	// Проверяем, что турнир активен (начат)
	if h.tournamentStatus != nil {
		t, err := h.tournamentStatus.GetByID(r.Context(), tournamentID)
		if err != nil {
			h.log.LogError("Failed to get tournament status", err)
			writeError(w, errors.ErrInternal.WithMessage("failed to verify tournament status"))
			return
		}
		if t.Status != domain.TournamentActive {
			writeError(w, errors.ErrForbidden.WithMessage("загрузка программ запрещена: турнир ещё не начался"))
			return
		}
	}

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
	// В ручном режиме — сохраняем оригинальную логику блокировки.
	if !autoRoundEnabled {
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
	}

	// Если имя не указано, используем имя файла
	if name == "" {
		name = header.Filename
	}

	// Определяем язык по расширению
	language := detectLanguage(header.Filename)

	// Создаём уникальный путь для файла (version будет назначена атомарно при INSERT)
	programID := uuid.New()
	ext := filepath.Ext(header.Filename)
	fileName := fmt.Sprintf("%s_%s_%s%s", teamID.String()[:8], gameID.String()[:8], programID.String()[:8], ext)
	filePath := filepath.Join(h.uploadDir, fileName)

	// Убеждаемся что директория существует (safety net для Docker volumes)
	if err := os.MkdirAll(h.uploadDir, 0755); err != nil {
		h.log.Error("Failed to ensure upload directory", zap.Error(err), zap.String("dir", h.uploadDir))
		writeError(w, errors.ErrInternal.WithMessage("не удалось сохранить файл: директория загрузок недоступна"))
		return
	}

	// Сохраняем файл
	dst, err := os.Create(filePath)
	if err != nil {
		h.log.Error("Failed to create file", zap.Error(err), zap.String("path", filePath))
		writeError(w, errors.ErrInternal.WithMessage("не удалось сохранить файл"))
		return
	}
	defer dst.Close()

	// Читаем всё содержимое файла в память (ограничено maxFileSize)
	fileContent, err := io.ReadAll(file)
	if err != nil {
		h.log.Error("Failed to read uploaded file", zap.Error(err))
		os.Remove(filePath)
		writeError(w, errors.ErrInternal.WithMessage("не удалось прочитать файл"))
		return
	}

	// Добавляем shebang для интерпретируемых языков (если его нет)
	shebang := getShebang(language)
	if shebang != "" && !bytes.HasPrefix(fileContent, []byte("#!")) {
		if _, err := dst.WriteString(shebang); err != nil {
			h.log.Error("Failed to write shebang", zap.Error(err))
			os.Remove(filePath)
			writeError(w, errors.ErrInternal.WithMessage("не удалось сохранить файл"))
			return
		}
	}

	if _, err := dst.Write(fileContent); err != nil {
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

	// Проверяем синтаксис для всех поддерживаемых языков
	var syntaxError *string
	if errMsg := validateSyntax(language, filePath); errMsg != "" {
		syntaxError = &errMsg
		h.log.Info("Syntax error detected",
			zap.String("file", filePath),
			zap.String("language", language),
			zap.String("error", errMsg),
		)
	}

	// Компилируем программу для компилируемых языков
	execPath := filePath // Путь к исполняемому файлу (может измениться после компиляции)
	if syntaxError == nil {
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
	}

	// Создаём запись в БД с атомарным назначением версии
	program := &domain.Program{
		ID:           programID,
		UserID:       userID,
		TeamID:       &teamID,
		TournamentID: &tournamentID,
		GameID:       &gameID,
		Name:         name,
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
	// Use program.ID (not local programID) since CreateWithAtomicVersion may regenerate it on retry
	if h.tournamentRepo != nil {
		participant := &domain.TournamentParticipant{
			ID:           uuid.New(),
			TournamentID: tournamentID,
			ProgramID:    program.ID,
			Rating:       1500, // Начальный рейтинг ELO
		}

		if err := h.tournamentRepo.AddParticipant(r.Context(), participant); err != nil {
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

	// ВАЖНО: Матчи НЕ создаются автоматически при загрузке программы!
	// Администратор должен вручную запустить матчи через кнопку "Run All Matches"
	// POST /api/v1/tournaments/{id}/run-matches

	h.log.Info("Program uploaded",
		zap.String("program_id", program.ID.String()),
		zap.String("user_id", userID.String()),
		zap.String("team_id", teamID.String()),
		zap.String("file", header.Filename),
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
	case "python":
		return runSyntaxCheck("python3", []string{"-m", "py_compile", filePath}, "Синтаксическая ошибка в Python коде")
	case "javascript":
		return runSyntaxCheck("node", []string{"--check", filePath}, "Синтаксическая ошибка в JavaScript коде")
	case "ruby":
		return runSyntaxCheck("ruby", []string{"-c", filePath}, "Синтаксическая ошибка в Ruby коде")
	case "php":
		return runSyntaxCheck("php", []string{"-l", filePath}, "Синтаксическая ошибка в PHP коде")
	case "lua":
		return runSyntaxCheck("luac", []string{"-p", filePath}, "Синтаксическая ошибка в Lua коде")
	case "c":
		return runSyntaxCheck("gcc", []string{"-fsyntax-only", filePath}, "Ошибка компиляции C")
	case "cpp":
		return runSyntaxCheck("g++", []string{"-fsyntax-only", filePath}, "Ошибка компиляции C++")
	case "java":
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
	case "c":
		if _, err := exec.LookPath("gcc"); err != nil {
			log.Warn("gcc not found, skipping compilation")
			return "", ""
		}
		cmd = exec.Command("gcc", "-O2", "-o", outputPath, sourcePath, "-lm")
	case "cpp":
		if _, err := exec.LookPath("g++"); err != nil {
			log.Warn("g++ not found, skipping compilation")
			return "", ""
		}
		cmd = exec.Command("g++", "-O2", "-o", outputPath, sourcePath)
	case "go":
		if _, err := exec.LookPath("go"); err != nil {
			log.Warn("go not found, skipping compilation")
			return "", ""
		}
		cmd = exec.Command("go", "build", "-o", outputPath, sourcePath)
	case "rust":
		if _, err := exec.LookPath("rustc"); err != nil {
			log.Warn("rustc not found, skipping compilation")
			return "", ""
		}
		cmd = exec.Command("rustc", "-O", "-o", outputPath, sourcePath)
	case "java":
		if _, err := exec.LookPath("javac"); err != nil {
			log.Warn("javac not found, skipping compilation")
			return "", ""
		}
		// Java: компилируем .java → .class, затем создаём wrapper-скрипт
		javacCmd := exec.Command("javac", sourcePath)
		var javacStderr bytes.Buffer
		javacCmd.Stderr = &javacStderr
		if err := javacCmd.Run(); err != nil {
			errMsg := strings.TrimSpace(javacStderr.String())
			if errMsg == "" {
				errMsg = err.Error()
			}
			return "", fmt.Sprintf("Ошибка компиляции Java: %s", errMsg)
		}
		// Создаём wrapper-скрипт для запуска java -cp <dir> <ClassName>
		className := strings.TrimSuffix(filepath.Base(sourcePath), ".java")
		classDir := filepath.Dir(sourcePath)
		wrapper := fmt.Sprintf("#!/bin/sh\njava -cp %s %s \"$@\"\n", classDir, className)
		if err := os.WriteFile(outputPath, []byte(wrapper), 0755); err != nil {
			return "", fmt.Sprintf("Ошибка создания wrapper: %s", err.Error())
		}
		return outputPath, ""
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
