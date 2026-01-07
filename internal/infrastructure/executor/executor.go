package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"go.uber.org/zap"
)

// Executor выполняет матчи в изолированных Docker контейнерах
type Executor struct {
	config           config.ExecutorConfig
	dockerClient     *client.Client
	programsPath     string // Путь к директории с программами внутри worker контейнера
	hostProgramsPath string // Путь на реальном хосте для Docker-in-Docker
	containerPath    string // Путь внутри контейнера tjudge-cli
	log              *logger.Logger
}

// NewExecutor создаёт новый executor
func NewExecutor(cfg config.ExecutorConfig, programsPath, hostProgramsPath string, log *logger.Logger) (*Executor, error) {
	// Создаём Docker клиент
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	// Если hostProgramsPath не указан, используем programsPath
	if hostProgramsPath == "" {
		hostProgramsPath = programsPath
	}

	return &Executor{
		config:           cfg,
		dockerClient:     cli,
		programsPath:     programsPath,
		hostProgramsPath: hostProgramsPath,
		containerPath:    "/programs", // Фиксированный путь внутри контейнера
		log:              log,
	}, nil
}

// Execute выполняет матч через tjudge-cli
func (e *Executor) Execute(ctx context.Context, match *domain.Match, program1Path, program2Path string) (*domain.MatchResult, error) {
	e.log.Info("Executing match",
		zap.String("match_id", match.ID.String()),
		zap.String("game_type", match.GameType),
		zap.String("program1", program1Path),
		zap.String("program2", program2Path),
	)

	start := time.Now()

	// Преобразуем пути к программам для использования внутри контейнера
	containerProgram1 := e.hostToContainerPath(program1Path)
	containerProgram2 := e.hostToContainerPath(program2Path)

	// Создаём контекст с таймаутом
	execCtx, cancel := context.WithTimeout(ctx, e.config.Timeout)
	defer cancel()

	// Запускаем матч в Docker контейнере
	result, err := e.runInDocker(execCtx, match.GameType, containerProgram1, containerProgram2)
	if err != nil {
		return nil, fmt.Errorf("failed to run match: %w", err)
	}

	result.MatchID = match.ID
	result.Duration = time.Since(start)

	e.log.Info("Match executed",
		zap.String("match_id", match.ID.String()),
		zap.Int("score1", result.Score1),
		zap.Int("score2", result.Score2),
		zap.Int("winner", result.Winner),
		zap.Duration("duration", result.Duration),
	)

	return result, nil
}

// runInDocker запускает матч в Docker контейнере
func (e *Executor) runInDocker(ctx context.Context, gameType, program1, program2 string) (*domain.MatchResult, error) {
	// Формируем команду для tjudge-cli
	// Формат: tjudge-cli <game_type> [OPTIONS] <PROGRAM1> <PROGRAM2>
	cmd := e.buildCommand(gameType, program1, program2)

	// Конфигурация контейнера
	containerConfig := &container.Config{
		Image: e.config.DockerImage,
		Cmd:   cmd,
		Tty:   false,
	}

	// Ограничения ресурсов и безопасности
	securityOpts := []string{
		"no-new-privileges:true", // Запрещаем повышение привилегий
	}

	// Добавляем seccomp профиль если указан
	if e.config.SeccompProfile != "" {
		securityOpts = append(securityOpts, "seccomp="+e.config.SeccompProfile)
	}

	// Добавляем AppArmor профиль если указан
	if e.config.AppArmorProfile != "" {
		securityOpts = append(securityOpts, "apparmor="+e.config.AppArmorProfile)
	}

	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			CPUQuota:       e.config.CPUQuota,
			CPUPeriod:      100000, // 100ms period
			Memory:         e.config.MemoryLimit,
			MemorySwap:     e.config.MemoryLimit, // Запрещаем swap
			PidsLimit:      &e.config.PidsLimit,
			CpusetCpus:     e.config.CPUSetCPUs, // Ограничиваем ядра CPU
			OomKillDisable: boolPtr(false),      // Разрешаем OOM killer
			// BlkioWeight не поддерживается на macOS (cgroups v2)
			Ulimits: []*container.Ulimit{
				{Name: "nofile", Soft: 64, Hard: 64},
				{Name: "nproc", Soft: 32, Hard: 32},
				{Name: "core", Soft: 0, Hard: 0},
				{Name: "fsize", Soft: 10485760, Hard: 10485760},
			},
		},
		// Монтируем директорию с программами (только для чтения)
		// Используем hostProgramsPath для Docker-in-Docker сценария
		Binds: []string{
			fmt.Sprintf("%s:%s:ro", e.hostProgramsPath, e.containerPath),
		},
		NetworkMode:    "none", // Отключаем сеть
		ReadonlyRootfs: true,   // Только для чтения root filesystem
		SecurityOpt:    securityOpts,
		CapDrop:        []string{"ALL"}, // Убираем все capabilities
		Tmpfs: map[string]string{
			"/tmp": "rw,noexec,nosuid,size=64m", // Временная директория для записи
		},
		AutoRemove: false, // Отключаем автоудаление чтобы получить логи
	}

	// Создаём контейнер
	resp, err := e.dockerClient.ContainerCreate(
		ctx,
		containerConfig,
		hostConfig,
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	containerID := resp.ID
	defer e.cleanup(containerID) // Удаляем контейнер после получения логов

	// Запускаем контейнер
	if err := e.dockerClient.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Ждём завершения
	statusCh, errCh := e.dockerClient.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			// Пытаемся получить логи даже при ошибке
			_, stderr, logErr := e.getContainerLogs(ctx, containerID)
			if logErr == nil && stderr != "" {
				return nil, fmt.Errorf("container error: %s", strings.TrimSpace(stderr))
			}
			return nil, fmt.Errorf("error waiting for container: %w", err)
		}
	case status := <-statusCh:
		// Получаем логи контейнера
		stdout, stderr, err := e.getContainerLogs(ctx, containerID)
		if err != nil {
			// Если не можем получить логи, возвращаем код выхода
			return nil, fmt.Errorf("container exited with code %d, failed to get logs: %w", status.StatusCode, err)
		}

		// Парсим результат
		return e.parseResult(status.StatusCode, stdout, stderr)
	case <-ctx.Done():
		// Таймаут - останавливаем контейнер
		_ = e.dockerClient.ContainerStop(context.Background(), containerID, container.StopOptions{})
		return nil, fmt.Errorf("match execution timeout")
	}

	return nil, fmt.Errorf("unexpected execution flow")
}

// getContainerLogs получает логи контейнера
func (e *Executor) getContainerLogs(ctx context.Context, containerID string) (string, string, error) {
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	}

	logs, err := e.dockerClient.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return "", "", err
	}
	defer logs.Close()

	// Читаем логи
	var stdout, stderr bytes.Buffer

	// Docker мультиплексирует stdout и stderr
	// Первые 8 байт - заголовок, затем данные
	buf := make([]byte, 8192)
	for {
		n, err := logs.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", err
		}

		if n > 8 {
			// Байт 0 указывает тип потока: 1=stdout, 2=stderr
			streamType := buf[0]
			data := buf[8:n]

			if streamType == 1 {
				stdout.Write(data)
			} else if streamType == 2 {
				stderr.Write(data)
			}
		}
	}

	return stdout.String(), stderr.String(), nil
}

// sanitizeForDB очищает строку от символов, недопустимых в PostgreSQL (null bytes)
func sanitizeForDB(s string) string {
	return strings.ReplaceAll(s, "\x00", "")
}

// parseResult парсит результат выполнения tjudge-cli
func (e *Executor) parseResult(exitCode int64, stdout, stderr string) (*domain.MatchResult, error) {
	result := &domain.MatchResult{
		ErrorCode: int(exitCode),
	}

	// Если есть ошибка
	if exitCode != 0 {
		// Sanitize error message - remove null bytes that break PostgreSQL
		result.ErrorMessage = sanitizeForDB(strings.TrimSpace(stderr))

		// Определяем winner по коду ошибки
		// 1 - ошибка программы 1, 2 - ошибка программы 2
		if exitCode == 1 {
			result.Winner = 2 // Побеждает программа 2
		} else if exitCode == 2 {
			result.Winner = 1 // Побеждает программа 1
		}

		return result, nil
	}

	// Парсим счёт из stdout
	// Формат: "10 15"
	scores := strings.Fields(strings.TrimSpace(stdout))
	if len(scores) != 2 {
		return nil, fmt.Errorf("invalid output format: expected 2 scores, got: %s", stdout)
	}

	score1, err := strconv.Atoi(scores[0])
	if err != nil {
		return nil, fmt.Errorf("invalid score1: %s", scores[0])
	}

	score2, err := strconv.Atoi(scores[1])
	if err != nil {
		return nil, fmt.Errorf("invalid score2: %s", scores[1])
	}

	result.Score1 = score1
	result.Score2 = score2

	// Определяем победителя
	if score1 > score2 {
		result.Winner = 1
	} else if score2 > score1 {
		result.Winner = 2
	} else {
		result.Winner = 0 // Ничья
	}

	return result, nil
}

// cleanup удаляет контейнер
func (e *Executor) cleanup(containerID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Останавливаем контейнер если он всё ещё работает
	_ = e.dockerClient.ContainerStop(ctx, containerID, container.StopOptions{})

	// Удаляем контейнер
	err := e.dockerClient.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force: true,
	})
	if err != nil {
		e.log.Error("Failed to remove container",
			zap.Error(err),
			zap.String("container_id", containerID),
		)
	}
}

// hostToContainerPath преобразует путь на хосте в путь внутри контейнера
func (e *Executor) hostToContainerPath(hostPath string) string {
	// Если путь начинается с programsPath, заменяем на containerPath
	if strings.HasPrefix(hostPath, e.programsPath) {
		return strings.Replace(hostPath, e.programsPath, e.containerPath, 1)
	}
	// Если путь не в programsPath, оставляем как есть (для обратной совместимости)
	return hostPath
}

// buildCommand формирует команду для запуска tjudge-cli
// Формат: tjudge-cli <game_type> [OPTIONS] <PROGRAM1> <PROGRAM2>
func (e *Executor) buildCommand(gameType, program1, program2 string) []string {
	cmd := []string{e.config.TJudgePath, gameType}

	// Добавляем количество итераций
	if e.config.DefaultIterations > 0 {
		cmd = append(cmd, "-i", strconv.Itoa(e.config.DefaultIterations))
	}

	// Добавляем verbose режим
	if e.config.Verbose {
		cmd = append(cmd, "-v")
	}

	// Добавляем пути к программам
	cmd = append(cmd, program1, program2)

	return cmd
}

// Close закрывает Docker клиент
func (e *Executor) Close() error {
	if e.dockerClient != nil {
		return e.dockerClient.Close()
	}
	return nil
}

// boolPtr возвращает указатель на bool
func boolPtr(b bool) *bool {
	return &b
}
