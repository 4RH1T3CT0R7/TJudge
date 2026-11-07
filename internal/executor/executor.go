package executor

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"go.uber.org/zap"
)

// Executor гоняет матч в изолированном докер-контейнере. код участников
// недоверенный, поэтому контейнер максимально урезан (см. hostConfig в runInDocker)
type Executor struct {
	config           config.ExecutorConfig
	dockerClient     *client.Client
	programsPath     string // путь к программам внутри worker-контейнера
	hostProgramsPath string // путь на реальном хосте, нужен для docker-in-docker
	containerPath    string // путь внутри контейнера tjudge-cli
	log              *logger.Logger
}

// NewExecutor создаёт executor
func NewExecutor(cfg config.ExecutorConfig, programsPath, hostProgramsPath string, log *logger.Logger) (*Executor, error) {
	// Engine API в seccomp= ждёт сам JSON профиля, путь к файлу понимает только
	// docker CLI. файл читается один раз на старте, дальше в конфиге содержимое
	if cfg.SeccompProfile != "" {
		profile, err := loadSeccompProfile(cfg.SeccompProfile)
		if err != nil {
			return nil, err
		}
		cfg.SeccompProfile = profile
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	// если хостовый путь не задан - он совпадает с programsPath (не-DinD случай)
	if hostProgramsPath == "" {
		hostProgramsPath = programsPath
	}

	if cfg.Verbose && cfg.DefaultIterations > maxTranscriptIters {
		log.Warn("Match transcripts disabled: too many iterations",
			zap.Int("iterations", cfg.DefaultIterations), zap.Int("max", maxTranscriptIters))
	}

	return &Executor{
		config:           cfg,
		dockerClient:     cli,
		programsPath:     filepath.Clean(programsPath),
		hostProgramsPath: filepath.Clean(hostProgramsPath),
		containerPath:    "/programs", // фиксированный путь внутри контейнера
		log:              log,
	}, nil
}

// loadSeccompProfile читает seccomp-профиль и возвращает его JSON одной строкой
func loadSeccompProfile(path string) (string, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- путь из env EXECUTOR_SECCOMP_PROFILE
	if err != nil {
		return "", fmt.Errorf("failed to read seccomp profile: %w", err)
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return "", fmt.Errorf("invalid seccomp profile %s: %w", path, err)
	}
	return buf.String(), nil
}

// Execute прогоняет матч через tjudge-cli
func (e *Executor) Execute(ctx context.Context, match *models.Match, program1Path, program2Path string) (*models.MatchResult, error) {
	e.log.Debug("Executing match",
		zap.String("match_id", match.ID.String()),
		zap.String("game_type", match.GameType),
		zap.String("program1", program1Path),
		zap.String("program2", program2Path),
	)

	start := time.Now()

	programs, binds, err := e.programMounts(program1Path, program2Path)
	if err != nil {
		return nil, err
	}

	result, err := e.runInDocker(ctx, match.GameType, programs[0], programs[1], binds)
	if err != nil {
		return nil, fmt.Errorf("failed to run match: %w", err)
	}

	result.MatchID = match.ID
	result.Duration = time.Since(start)

	e.log.Debug("Match executed",
		zap.String("match_id", match.ID.String()),
		zap.Int("score1", result.Score1),
		zap.Int("score2", result.Score2),
		zap.Int("winner", result.Winner),
		zap.Int("error_code", result.ErrorCode),
		zap.String("error_message", result.ErrorMessage),
		zap.Duration("duration", result.Duration),
	)

	return result, nil
}

// Check - самопроверка программы: матч первым игроком против эталонного бота
// игры, который лежит в образе (см. docker/tjudge/sandbox.sh)
func (e *Executor) Check(ctx context.Context, gameType, programPath string) (*models.MatchResult, error) {
	programs, binds, err := e.programMounts(programPath)
	if err != nil {
		return nil, err
	}
	return e.runInDocker(ctx, gameType, programs[0], sandboxRefBots+gameType, binds)
}

// runInDocker поднимает контейнер, ждёт матч и разбирает результат
func (e *Executor) runInDocker(ctx context.Context, gameType, program1, program2 string, binds []string) (*models.MatchResult, error) {
	// формат: tjudge-cli <game_type> [OPTIONS] <PROGRAM1> <PROGRAM2>
	cmd := e.buildCommand(gameType, program1, program2)

	exitCode, stdout, stderrRaw, err := e.runContainer(ctx, cmd, buildMatchHostConfig(e.config, binds), e.config.Timeout, maxLogSize)
	if errors.Is(err, errContainerTimeout) {
		// истёк собственный лимит матча - зависшая/медленная программа, это её
		// вина, поэтому ошибка терминальная (fmt.Errorf), не infra
		return nil, fmt.Errorf("match execution timeout")
	}
	if err != nil {
		return nil, err
	}

	transcript, stderrRaw := splitTranscript(stderrRaw)
	result, err := e.parseResult(exitCode, stdout, sanitizeStderr(stderrRaw))
	if result != nil {
		result.Transcript = transcript
	}
	return result, err
}

// errContainerTimeout - контейнер не уложился в свой лимит при живом ctx воркера
var errContainerTimeout = errors.New("container timeout")

// runContainer поднимает матч-контейнер с аргументами точки входа cmd и ждёт его
// выхода не дольше timeout. Возвращает код выхода и логи. Сбои окружения - infra,
// отмена ctx воркера - тоже infra (матч повторится), свой лимит - errContainerTimeout
// вместе с логами, что успели набраться (пачке они нужны для сыгранных матчей)
func (e *Executor) runContainer(ctx context.Context, cmd []string, hostConfig *container.HostConfig, timeout time.Duration, maxLog int) (int64, string, string, error) {
	e.log.Debug("Creating container",
		zap.Strings("cmd", cmd),
		zap.Strings("binds", hostConfig.Binds),
		zap.String("image", e.config.DockerImage),
	)

	// у матча свой дедлайн поверх ctx воркера: по тому, какой из них истёк,
	// видно, виновата программа или окружение
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	containerConfig := &container.Config{
		Image: e.config.DockerImage,
		// точка входа образа разводит ботов по своим uid (docker/tjudge/sandbox.sh).
		// задана явно: в старом образе без неё контейнер не стартует (infra-ошибка,
		// матч повторится), а не проваливает каждый матч
		Entrypoint: []string{sandboxEntrypoint},
		User:       "0:0",
		Cmd:        cmd,
		Tty:        false,
		Labels:     containerLabels(),
	}

	resp, err := e.dockerClient.ContainerCreate(
		execCtx,
		containerConfig,
		hostConfig,
		nil,
		nil,
		"",
	)
	if err != nil {
		return 0, "", "", infraErrorf("failed to create container: %w", err)
	}

	containerID := resp.ID
	// стоит сразу после create - сработает на любом выходе; контейнеры убитого
	// воркера подбирает RemoveOrphans на старте
	defer e.cleanup(containerID)

	if err := e.dockerClient.ContainerStart(execCtx, containerID, container.StartOptions{}); err != nil {
		return 0, "", "", infraErrorf("failed to start container: %w", err)
	}

	statusCh, errCh := e.dockerClient.ContainerWait(execCtx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		// по истечении execCtx ошибка ожидания - это тот же дедлайн, разбор ниже
		if execCtx.Err() == nil {
			if err != nil {
				// логи забираются даже при ошибке
				_, stderr, logErr := e.getContainerLogs(ctx, containerID, maxLogSize)
				if logErr == nil && stderr != "" {
					_, stderr = splitTranscript(stderr)
					return 0, "", "", fmt.Errorf("container error: %s", strings.TrimSpace(sanitizeStderr(stderr)))
				}
				return 0, "", "", infraErrorf("error waiting for container: %w", err)
			}
			// errCh с nil - так быть не должно, считается инфра-ошибкой
			return 0, "", "", infraErrorf("container %s: wait returned nil error without status", containerID)
		}
	case status := <-statusCh:
		stdout, stderr, err := e.getContainerLogs(ctx, containerID, maxLog)
		if err != nil {
			// логи недоступны из-за docker api - это окружение, а не программа,
			// матч можно спокойно повторить
			return 0, "", "", infraErrorf("container exited with code %d, failed to get logs: %w", status.StatusCode, err)
		}

		// логи целиком в лог не идут, только размеры
		e.log.Debug("Container finished",
			zap.String("container_id", containerID),
			zap.Int64("exit_code", status.StatusCode),
			zap.Int("stdout_len", len(stdout)),
			zap.Int("stderr_len", len(stderr)),
		)
		return status.StatusCode, stdout, stderr, nil
	case <-execCtx.Done():
	}

	// контейнер убивает отложенный cleanup (remove с Force)
	if ctx.Err() != nil {
		// отменён ctx воркера (shutdown, общий таймаут обработки) - программа
		// тут ни при чём, матч надо повторить, а не записывать ей таймаут
		return 0, "", "", infraErrorf("match interrupted: %w", ctx.Err())
	}
	// логи до удаления контейнера, ошибка чтения тут не важна
	stdout, stderr, _ := e.getContainerLogs(ctx, containerID, maxLog)
	return 0, stdout, stderr, errContainerTimeout
}

// BatchItem - матч пачки и пути его программ
type BatchItem struct {
	Match        *models.Match
	Program1Path string
	Program2Path string
}

// BatchResult - итог матча пачки, как у Execute: результат или ошибка.
// InfraError - матч не сыгран, его повторяют
type BatchResult struct {
	Result *models.MatchResult
	Err    error
}

// batchMargin - запас контейнеру пачки сверх лимита матча. Время на сами матчи
// точка входа держит в лимите одного матча (sandbox.sh --batch), запас уходит
// на старт контейнера, уборку за ботами и вывод итогов
const batchMargin = 10 * time.Second

// ExecuteBatch играет матчи по очереди в одном контейнере (sandbox.sh --batch):
// создание и удаление контейнера стоит дороже самого матча. Ошибка - сбой всей
// пачки (infra, все матчи повторяются), иначе итоги матчей в порядке items
func (e *Executor) ExecuteBatch(ctx context.Context, items []BatchItem) ([]BatchResult, error) {
	start := time.Now()
	results := make([]BatchResult, len(items))
	cmd := append([]string{"--batch", strconv.Itoa(max(1, int(e.config.Timeout/time.Second)))}, e.commandOptions()...)
	cmd = append(cmd, "--")
	var binds []string
	// индексы items, ушедших в контейнер, по порядку записей точки входа
	var played []int
	for i, it := range items {
		programs, b, err := e.programMounts(it.Program1Path, it.Program2Path)
		if err != nil {
			// как у Execute: программы нет на диске - ошибка матча, а не пачки
			results[i].Err = fmt.Errorf("failed to run match: %w", err)
			continue
		}
		for _, bind := range b {
			if !slices.Contains(binds, bind) {
				binds = append(binds, bind)
			}
		}
		cmd = append(cmd, it.Match.GameType, programs[0], programs[1])
		played = append(played, i)
	}
	if len(played) == 0 {
		return results, nil
	}

	_, stdout, _, err := e.runContainer(ctx, cmd, batchHostConfig(e.config, binds), e.config.Timeout+batchMargin, len(played)*maxLogSize)
	if err != nil && !errors.Is(err, errContainerTimeout) {
		// сбой окружения: итогов нет, все матчи повторяются
		return nil, infraErrorf("batch failed: %w", err)
	}
	// контейнер, не уложившийся в лимит, успел записать итоги части матчей:
	// они в силе, остальные без записи повторяются
	records := parseBatchOutput(stdout)
	share := time.Since(start) / time.Duration(len(played))
	for k, i := range played {
		results[i] = e.batchResult(records[k], items[i].Match, share)
	}
	return results, nil
}

// batchResult переводит запись точки входа в итог матча. Нет записи - матч не
// сыгран (контейнер упал раньше, старый образ без --batch), его повторят
func (e *Executor) batchResult(rec *batchRecord, match *models.Match, d time.Duration) BatchResult {
	if rec == nil {
		return BatchResult{Err: infraErrorf("failed to run match: no result in batch output")}
	}
	switch rec.status {
	case "done":
		transcript, stderr := splitTranscript(rec.stderr)
		result, err := e.parseResult(rec.code, rec.stdout, sanitizeStderr(stderr))
		if err != nil {
			return BatchResult{Err: fmt.Errorf("failed to run match: %w", err)}
		}
		result.Transcript = transcript
		result.MatchID = match.ID
		result.Duration = d
		return BatchResult{Result: result}
	case "timeout":
		// свой полный лимит, как у одиночного матча - вина программы
		return BatchResult{Err: fmt.Errorf("failed to run match: match execution timeout")}
	default:
		return BatchResult{Err: infraErrorf("failed to run match: not played in batch (%s)", rec.status)}
	}
}

// batchRecord - итог матча из вывода sandbox.sh --batch
type batchRecord struct {
	status string
	code   int64
	stdout string
	stderr string
}

// parseBatchOutput разбирает записи sandbox.sh --batch по номерам матчей:
// @@begin <номер> <статус> <код>, stdout в base64, @@stderr, stderr в base64,
// @@end. Битая запись пропускается, её матч считается несыгранным, как и матч
// с двумя записями: точка входа пишет по одной, вторая - признак подделки
func parseBatchOutput(out string) map[int]*batchRecord {
	records := make(map[int]*batchRecord)
	seen := make(map[int]bool)
	var (
		cur  *batchRecord
		idx  int
		part int
		body [2]strings.Builder
	)
	for line := range strings.SplitSeq(out, "\n") {
		switch {
		case strings.HasPrefix(line, "@@begin "):
			cur = nil
			f := strings.Fields(line)
			if len(f) != 4 {
				continue
			}
			n, err1 := strconv.Atoi(f[1])
			code, err2 := strconv.ParseInt(f[3], 10, 64)
			if err1 != nil || err2 != nil {
				continue
			}
			if seen[n] {
				delete(records, n)
				continue
			}
			seen[n] = true
			cur, idx, part = &batchRecord{status: f[2], code: code}, n, 0
			body[0].Reset()
			body[1].Reset()
		case cur == nil:
		case line == "@@stderr":
			part = 1
		case line == "@@end":
			stdout, err1 := base64.StdEncoding.DecodeString(body[0].String())
			stderr, err2 := base64.StdEncoding.DecodeString(body[1].String())
			if err1 == nil && err2 == nil {
				cur.stdout, cur.stderr = string(stdout), string(stderr)
				records[idx] = cur
			}
			cur = nil
		default:
			body[part].WriteString(line)
		}
	}
	return records
}

// раскладка матч-контейнера, общая с docker/tjudge/sandbox.sh
const (
	sandboxEntrypoint = "/usr/local/bin/sandbox"
	// сюда монтируются файлы программ, каталог доступен только root
	sandboxMountPath = "/mnt/programs"
	// код выхода точки входа при сбое подготовки песочницы
	sandboxSetupFailed = 125
	// путь эталонного бота игры: /refbots/<игра>
	sandboxRefBots = "/refbots/"
)

// buildMatchHostConfig собирает докер-hostConfig для матч-контейнера. вынесено
// отдельно чтобы флаги можно было проверить тестом - каждая строка тут отдельная
// линия обороны против чужого кода, значения трогать нельзя
func buildMatchHostConfig(cfg config.ExecutorConfig, binds []string) *container.HostConfig {
	securityOpts := []string{
		"no-new-privileges:true", // без setuid-эскалации
	}

	// seccomp-профиль лежит в deployments/security, по дефолту не подключён,
	// включается env-ом EXECUTOR_SECCOMP_PROFILE. тут уже JSON, файл прочитан в NewExecutor
	if cfg.SeccompProfile != "" {
		securityOpts = append(securityOpts, "seccomp="+cfg.SeccompProfile)
	}

	// apparmor так же - опционально, только при заданном профиле
	if cfg.AppArmorProfile != "" {
		securityOpts = append(securityOpts, "apparmor="+cfg.AppArmorProfile)
	}

	nproc := (cfg.PidsLimit - 4) / 2
	if cfg.PidsLimit <= 0 { // pids без лимита
		nproc = 64
	}

	return &container.HostConfig{
		Resources: container.Resources{
			CPUQuota:       cfg.CPUQuota,
			CPUPeriod:      100000, // период cpu 100мс (из доки docker)
			CPUShares:      sandboxCPUShares,
			Memory:         cfg.MemoryLimit,
			MemorySwap:     cfg.MemoryLimit, // == Memory: swap запрещён, лимит памяти не обойти
			PidsLimit:      &cfg.PidsLimit,  // защита от fork-bomb
			CpusetCpus:     cfg.CPUSetCPUs,
			OomKillDisable: new(false), // oom-killer включён, runaway убивается а не висит
			// BlkioWeight на macOS не поддерживается (cgroups v2)
			Ulimits: []*container.Ulimit{
				{Name: "nofile", Soft: 1024, Hard: 1024}, // хватает python + subprocess
				// nproc считается на uid бота: каждому из двух своя половина
				// PidsLimit за вычетом запаса на tjudge-cli, иначе один бот
				// выбирает весь бюджет контейнера и соперник не может стартовать
				{Name: "nproc", Soft: nproc, Hard: nproc},
				{Name: "core", Soft: 0, Hard: 0}, // без core-дампов
				// точка входа копирует в /programs артефакт до maxArtifactSize
				{Name: "fsize", Soft: maxArtifactSize, Hard: maxArtifactSize},
			},
		},
		// только файлы двух программ матча и только на чтение (см. programMounts)
		Binds:          binds,
		NetworkMode:    "none", // сети нет - ни эксфильтрации, ни скачивания
		ReadonlyRootfs: true,   // корень только на чтение, писать можно лишь в tmpfs
		SecurityOpt:    securityOpts,
		CapDrop:        []string{"ALL"}, // все capabilities сняты
		// кроме нужных точке входа (скопировать программы из /mnt/programs,
		// отдать их uid ботов и сбросить root до этих uid) и tjudge-cli (KILL:
		// добить бота под чужим uid, иначе он ждёт его выхода до таймаута матча).
		// сами боты работают без capabilities. чтение чужих файлов через
		// DAC_OVERRIDE: DAC_READ_SEARCH открыла бы в дефолтном seccomp докера
		// open_by_handle_at, а с ним любой файл хостовой ФС за bind-ами
		CapAdd: []string{"CHOWN", "DAC_OVERRIDE", "SETUID", "SETGID", "KILL"},
		Tmpfs: map[string]string{
			"/tmp": "rw,nosuid,size=64m", // writable /tmp, но nosuid (без эскалации) и капнут
			// копии программ под uid ботов: две программы до 32мб (maxArtifactSize).
			// 0711 - список файлов ботам не виден
			"/programs": "rw,exec,nosuid,nodev,size=80m,mode=0711",
		},
		AutoRemove: false, // не автоудалять - сперва надо забрать логи, потом cleanup
	}
}

// batchHostConfig - hostConfig контейнера пачки (sandbox.sh --batch): тот же, что
// у матча, плюс то, что не даёт боту оставить состояние следующему матчу пачки.
// У одиночного матча этого нет: если среда не примет sysctl (rootless docker),
// упадут только пачки, и воркер доиграет их матчи по одному
func batchHostConfig(cfg config.ExecutorConfig, binds []string) *container.HostConfig {
	hc := buildMatchHostConfig(cfg, binds)
	// FOWNER: уборка от root удаляет файлы ботов и в их sticky-каталогах
	hc.CapAdd = append(hc.CapAdd, "FOWNER")
	// итоги матчей: боты могут забить /tmp и /programs, а сюда им хода нет
	hc.Tmpfs["/mnt/results"] = "rw,noexec,nosuid,nodev,size=2m,mode=0700"
	// SysV IPC и POSIX-очереди переживают процесс: очередей и семафоров нет
	// вовсе, сегмент общей памяти живёт, пока к нему подключён процесс. своё
	// пространство IPC - условие этих sysctl
	hc.IpcMode = container.IPCModePrivate
	hc.Sysctls = map[string]string{
		"kernel.msgmni":          "0",
		"kernel.sem":             "0 0 0 0",
		"kernel.shm_rmid_forced": "1",
		"fs.mqueue.queues_max":   "0",
	}
	return hc
}

// maxLogSize - сколько логов матча читается с каждого потока. у пачки - на матч
const maxLogSize = 1 << 20

// getContainerLogs читает stdout/stderr контейнера, не больше maxSize с потока
func (e *Executor) getContainerLogs(ctx context.Context, containerID string, maxSize int) (string, string, error) {
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     false,
	}

	logs, err := e.dockerClient.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return "", "", err
	}
	defer logs.Close()

	// демультиплексирование через stdcopy. на stdout и stderr - отдельные лимитчики
	// по maxSize: общий LimitReader при большом stdout молча обрезал stderr до нуля и
	// терял сообщение об ошибке
	var stdoutBuf, stderrBuf bytes.Buffer
	stdoutW := &limitWriter{w: &stdoutBuf, n: maxSize}
	stderrW := &limitWriter{w: &stderrBuf, n: maxSize}
	if _, err := stdcopy.StdCopy(stdoutW, stderrW, logs); err != nil {
		return "", "", fmt.Errorf("failed to read container logs: %w", err)
	}

	return stdoutBuf.String(), stderrBuf.String(), nil
}

// limitWriter оборачивает io.Writer и режет общий объём записи. когда бюджет
// исчерпан, Write возвращает len(p) без ошибки - иначе stdcopy примет это за
// short-write и оборвёт чтение второго потока (у него свой независимый бюджет)
type limitWriter struct {
	w io.Writer
	n int // сколько байт ещё можно записать
}

func (lw *limitWriter) Write(p []byte) (int, error) {
	if lw.n <= 0 {
		return len(p), nil // тихо отбрасывается
	}
	if len(p) > lw.n {
		// пишется только то, что влезло в бюджет
		written, err := lw.w.Write(p[:lw.n])
		lw.n -= written
		if err != nil {
			return written, err
		}
		return len(p), nil
	}
	written, err := lw.w.Write(p)
	lw.n -= written
	return written, err
}

// sanitizeForDB убирает null-байты и битый UTF-8: и то и другое рушит INSERT в
// postgres. битый UTF-8 дают вывод бота и обрезка stderr посреди символа
func sanitizeForDB(s string) string {
	return strings.ToValidUTF8(strings.ReplaceAll(s, "\x00", ""), "\uFFFD")
}

// ansiEscapeRe ловит ansi-эскейпы (цветовые последовательности типа \x1b[31m)
var ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// maxStderrSize - сколько stderr остаётся после чистки (4кб)
const maxStderrSize = 4096

// stderrHeadSize - сколько начала stderr остаётся при обрезке, остальное
// бюджета отдаётся концу
const stderrHeadSize = 1024

// sanitizeStderr чистит сырой stderr: снимает ansi-эскейпы и режет до 4кб.
// режется середина: в конце итоговая строка tjudge-cli и хвост stderr бота
// от sandbox.sh (около 2,2кб), перед ними бывает длинный вывод (ход бота
// целиком в «unknown action»)
func sanitizeStderr(raw string) string {
	cleaned := ansiEscapeRe.ReplaceAllString(raw, "")

	if len(cleaned) > maxStderrSize {
		const cut = "\n...(truncated)...\n"
		tail := maxStderrSize - stderrHeadSize - len(cut)
		cleaned = cleaned[:stderrHeadSize] + cut + cleaned[len(cleaned)-tail:]
	}

	return cleaned
}

// sandboxBotStderr - начало строки, которой sandbox.sh отделяет хвост stderr
// упавшего бота
const sandboxBotStderr = "--- stderr программы"

// строки -v у tjudge-cli: ход стороны ([>] - программа 1, [<] - 2), ставка
// аукциона, очки итерации. остальные ([init], [result], счёт нарастающим
// итогом) транскрипту не нужны
var (
	verboseLineRe = regexp.MustCompile(`^\[(init|>|<|iter-\d+|result|drop)\] `)
	moveLineRe    = regexp.MustCompile(`^\[([><])\] \w+: (\w+)$`)
	bidLineRe     = regexp.MustCompile(`^\[iter-\d+\] (left|right) bid: (-?\d+)$`)
	pointsLineRe  = regexp.MustCompile(`^\[iter-\d+\] result: \((-?\d+), (-?\d+)\)$`)
)

// splitTranscript отделяет от stderr строки -v и собирает из них транскрипт
// (nil, если ходов нет). остаток - тот же stderr, что без -v: итоговая строка
// ошибки и хвост stderr бота для текста ошибки. stderr бота за маркером
// sandbox.sh не разбирается: ходы берутся только из вывода судьи.
// у стороны без ходов - пустой массив, а не null в JSON
func splitTranscript(stderr string) (*models.Transcript, string) {
	moves := [][]int{{}, {}}
	var points [][]int
	var rest strings.Builder
	// -v любой игры начинается с [init]. без неё начало потока срезал tail в
	// sandbox.sh (ход бота целиком в «unknown action»), и ходы шли бы не с первой итерации
	started := false
	lines := strings.SplitAfter(stderr, "\n")
	for i, raw := range lines {
		line := strings.TrimSuffix(raw, "\n")
		if strings.HasPrefix(line, sandboxBotStderr) {
			rest.WriteString(strings.Join(lines[i:], ""))
			break
		}
		if !verboseLineRe.MatchString(line) {
			rest.WriteString(raw)
			continue
		}
		if strings.HasPrefix(line, "[init] ") {
			started = true
		} else if m := moveLineRe.FindStringSubmatch(line); m != nil {
			side := 0
			if m[1] == "<" {
				side = 1
			}
			if v, ok := parseMove(m[2]); ok {
				moves[side] = append(moves[side], v)
			}
		} else if m := bidLineRe.FindStringSubmatch(line); m != nil {
			side := 0
			if m[1] == "right" {
				side = 1
			}
			v, _ := strconv.Atoi(m[2])
			moves[side] = append(moves[side], v)
		} else if m := pointsLineRe.FindStringSubmatch(line); m != nil {
			if points == nil {
				points = [][]int{nil, nil}
			}
			for side := range 2 {
				v, _ := strconv.Atoi(m[side+1])
				points[side] = append(points[side], v)
			}
		}
	}
	if !started || (len(moves[0]) == 0 && len(moves[1]) == 0) {
		return nil, rest.String()
	}
	return &models.Transcript{Moves: moves, Points: points}, rest.String()
}

// parseMove переводит ход из вывода -v в число: решения дилеммы - в
// models.MoveCooperate/MoveDefect, остальные ходы уже числа
func parseMove(s string) (int, bool) {
	switch s {
	case "Cooperate":
		return models.MoveCooperate, true
	case "Defect":
		return models.MoveDefect, true
	}
	v, err := strconv.Atoi(s)
	return v, err == nil
}

// parseResult разбирает вывод tjudge-cli
func (e *Executor) parseResult(exitCode int64, stdout, stderr string) (*models.MatchResult, error) {
	result := &models.MatchResult{
		ErrorCode: int(exitCode),
	}

	// песочница не поднялась (нет capabilities, старый образ) - программы
	// не запускались, матч надо повторить
	if exitCode == sandboxSetupFailed {
		return nil, infraErrorf("sandbox setup failed: %s", strings.TrimSpace(stderr))
	}

	// ненулевой код - это валидный терминальный результат, не ошибка парсинга
	if exitCode != 0 {
		// собирается развёрнутое сообщение об ошибке из stdout и stderr
		var errorParts []string

		// по коду понятно чья программа упала
		switch exitCode {
		case 1:
			errorParts = append(errorParts, "Программа 1 завершилась с ошибкой:")
			result.Winner = 2 // побеждает программа 2
		case 2:
			errorParts = append(errorParts, "Программа 2 завершилась с ошибкой:")
			result.Winner = 1 // побеждает программа 1
		default:
			// системная ошибка с неизвестным кодом. ErrorCode уже выставлен выше,
			// winner остаётся 0 (ничья), матч запишется как failed
			errorParts = append(errorParts, fmt.Sprintf("Ошибка выполнения (код %d):", exitCode))
		}

		// stderr - основной источник ошибки
		if stderrClean := strings.TrimSpace(stderr); stderrClean != "" {
			errorParts = append(errorParts, "\n--- stderr ---\n"+stderrClean)
		}

		// иногда полезное сыпется и в stdout
		if stdoutClean := strings.TrimSpace(stdout); stdoutClean != "" {
			// stdout берётся только если это не просто счёт
			if !strings.Contains(stdoutClean, " ") || len(stdoutClean) > 20 {
				errorParts = append(errorParts, "\n--- stdout ---\n"+stdoutClean)
			}
		}

		result.ErrorMessage = sanitizeForDB(strings.Join(errorParts, ""))

		return result, nil
	}

	// формат счёта: "10 15"
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

	// защита от мусорного вывода бота: потолок растёт вместе с числом итераций,
	// чтобы честные high-iteration прогоны не резались. 1000 очков на итерацию -
	// щедрая верхняя оценка для любой игры, пол 100000 покрывает дефолтные 100 итераций
	maxScore := max(e.config.DefaultIterations*1000, 100_000)
	if score1 > maxScore || score1 < -maxScore || score2 > maxScore || score2 < -maxScore {
		return nil, fmt.Errorf("scores out of bounds [-%d, %d]: %d, %d", maxScore, maxScore, score1, score2)
	}

	result.Score1 = score1
	result.Score2 = score2

	if score1 > score2 {
		result.Winner = 1
	} else if score2 > score1 {
		result.Winner = 2
	} else {
		result.Winner = 0 // ничья
	}

	return result, nil
}

// sandboxCPUShares - вес CPU песочниц матча и сборки, ниже обычного 1024 (на
// cgroup v2 docker переводит его в cpu.weight, тоже ниже стандартного 100).
// при переподписке хоста они уступают dockerd, postgres и api, которых иначе
// душили сотни ботов, а между собой делят ядра поровну. на неперегруженном
// хосте вес ни на что не влияет: квоту CPUQuota он не меняет
const sandboxCPUShares = 512

// removeTimeout - сколько ждать удаления контейнера. на перегруженном демоне
// (сотни параллельных матчей) удаление шло дольше 10 с, и запрос отваливался по
// таймауту с ошибкой в логе, не дождавшись результата. дольше 30 с нельзя: пока
// контейнер удаляется, матч числится running, а recovery сбрасывает running
// старше WORKER_TIMEOUT + 30s, то есть не раньше EXECUTOR_TIMEOUT + 50s
const removeTimeout = 30 * time.Second

// cleanup force-удаляет контейнер (живой убивается сразу, без SIGTERM и
// 10с ожидания stop), ошибку только логирует
func (e *Executor) cleanup(containerID string) {
	ctx, cancel := context.WithTimeout(context.Background(), removeTimeout)
	defer cancel()

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

// programMounts переводит пути программ в пути внутри контейнера и собирает
// bind-ы только под них: исполняемый файл и, у java, каталог классов рядом.
// весь каталог программ монтировать нельзя - бот прочитал бы исходники и
// бинарники других команд. файлы монтируются в sandboxMountPath, а по путям
// /programs/<имя> их раскладывает точка входа образа
func (e *Executor) programMounts(paths ...string) ([]string, []string, error) {
	containerPaths := make([]string, 0, len(paths))
	var binds []string
	for i, p := range paths {
		cp, err := e.hostToContainerPath(p)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid program%d path: %w", i+1, err)
		}
		p = filepath.Clean(p)
		// без файла докер молча создал бы на его месте пустой каталог
		if _, err := os.Stat(p); err != nil {
			return nil, nil, fmt.Errorf("program%d not found: %w", i+1, err)
		}
		containerPaths = append(containerPaths, cp)
		binds = e.appendBind(binds, cp)

		// java-wrapper ссылается на <имя>_classes рядом с собой (см. installArtifact)
		if st, err := os.Stat(p + javaClassesSuffix); err == nil && st.IsDir() {
			binds = e.appendBind(binds, cp+javaClassesSuffix)
		}
	}
	return containerPaths, binds, nil
}

// appendBind добавляет ro-bind в sandboxMountPath для пути внутри контейнера,
// источник на хосте считается от hostProgramsPath (docker-in-docker). дубль
// точки монтирования докер отвергает, поэтому повтор пропускается
func (e *Executor) appendBind(binds []string, containerPath string) []string {
	rel := strings.TrimPrefix(containerPath, e.containerPath)
	bind := fmt.Sprintf("%s%s:%s%s:ro", e.hostProgramsPath, rel, sandboxMountPath, rel)
	if slices.Contains(binds, bind) {
		return binds
	}
	return append(binds, bind)
}

// hostToContainerPath переводит путь на хосте в путь внутри контейнера.
// Clean нормализует, а пути вне programsPath отвергаются - defense-in-depth
// против path traversal
func (e *Executor) hostToContainerPath(hostPath string) (string, error) {
	cleaned := filepath.Clean(hostPath)

	// именно поддиректория (префикс вместе с сепаратором), иначе сосед вроде
	// /data/programs-evil прошёл бы как «поддиректория» /data/programs
	prefix := e.programsPath + string(filepath.Separator)
	if strings.HasPrefix(cleaned, prefix) {
		return e.containerPath + cleaned[len(e.programsPath):], nil
	}

	// путь вне programsPath - отказ
	return "", fmt.Errorf("path %q is outside programs directory %q", cleaned, e.programsPath)
}

// buildCommand собирает аргументы tjudge-cli: <game_type> [OPTIONS] <PROGRAM1> <PROGRAM2>.
// точка входа передаёт их tjudge-cli как есть, заменив две последние
// программы лаунчерами. игры: см. github.com/bmstu-itstech/tjudge-cli
func (e *Executor) buildCommand(gameType, program1, program2 string) []string {
	cmd := append([]string{gameType}, e.commandOptions()...)
	return append(cmd, program1, program2)
}

// commandOptions - опции tjudge-cli, общие для всех матчей: итерации и -v
func (e *Executor) commandOptions() []string {
	var opts []string
	if e.config.DefaultIterations > 0 {
		opts = append(opts, "-i", strconv.Itoa(e.config.DefaultIterations))
	}
	if e.verbose() {
		opts = append(opts, "-v")
	}
	return opts
}

// maxTranscriptIters - потолок итераций для транскрипта. -v пишет около 100
// байт на итерацию, транскрипт в базе - около 10 байт: на 1000 итераций
// это 100 КБ stderr и 10 КБ на матч
const maxTranscriptIters = 1000

// verbose - включать ли -v: транскрипты длиннее maxTranscriptIters не пишутся
func (e *Executor) verbose() bool {
	return e.config.Verbose && e.config.DefaultIterations <= maxTranscriptIters
}

// метки контейнеров матчей и сборок. по ним воркер на старте находит
// контейнеры, брошенные убитым процессом (SIGKILL на деплое, падение)
const (
	labelManaged = "tjudge.managed"
	labelOwner   = "tjudge.owner"
)

// ownerID - hostname воркера, в докере это короткий id его контейнера.
// реплики с общим hostname (hostname: в compose, network_mode: host) на старте
// удалят живые контейнеры друг друга
var ownerID, _ = os.Hostname()

func containerLabels() map[string]string {
	return map[string]string{labelManaged: "true", labelOwner: ownerID}
}

// RemoveOrphans удаляет помеченные контейнеры, чей воркер уже не работает.
// реплики делят один docker-демон, поэтому контейнеры живого чужого воркера
// не трогаются
func (e *Executor) RemoveOrphans(ctx context.Context) (int, error) {
	list, err := e.dockerClient.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", labelManaged+"=true")),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list containers: %w", err)
	}

	alive := map[string]bool{}
	removed := 0
	for _, c := range list {
		owner := c.Labels[labelOwner]
		if owner != ownerID {
			running, seen := alive[owner]
			if !seen {
				running = e.ownerRunning(ctx, owner)
				alive[owner] = running
			}
			if running {
				continue
			}
		}
		// свои контейнеры - от прошлого процесса в этом же контейнере воркера
		if err := e.dockerClient.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true}); err != nil {
			e.log.Warn("Failed to remove orphan container", zap.String("container_id", c.ID), zap.Error(err))
			continue
		}
		removed++
	}
	return removed, nil
}

// ownerRunning - жив ли воркер-владелец. мёртвым считается только тот, кого
// демон точно не знает или кто остановлен; при сбое запроса - живым, чтобы
// не убить матчи соседней реплики
func (e *Executor) ownerRunning(ctx context.Context, owner string) bool {
	if owner == "" {
		return false
	}
	info, err := e.dockerClient.ContainerInspect(ctx, owner)
	if err != nil {
		return !cerrdefs.IsNotFound(err)
	}
	return info.ContainerJSONBase != nil && info.State != nil && info.State.Running
}

// EnsureImage проверяет образ на docker-хосте и скачивает его, если нет:
// ContainerCreate сам образы не тянет, и без этой проверки все матчи (или
// сборки) уходили бы в infra-ошибку по кругу
func (e *Executor) EnsureImage(ctx context.Context, ref string) error {
	_, err := e.dockerClient.ImageInspect(ctx, ref)
	if err == nil {
		return nil
	}
	if !cerrdefs.IsNotFound(err) {
		return fmt.Errorf("failed to inspect image %s: %w", ref, err)
	}

	e.log.Info("Image not found locally, pulling", zap.String("image", ref))
	rc, err := e.dockerClient.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", ref, err)
	}
	defer rc.Close()
	// pull идёт, пока читается поток; ошибка посреди pull приходит внутри него
	dec := json.NewDecoder(rc)
	for {
		var msg struct {
			Error string `json:"error"`
		}
		if err := dec.Decode(&msg); err == io.EOF {
			return nil
		} else if err != nil {
			return fmt.Errorf("failed to pull image %s: %w", ref, err)
		}
		if msg.Error != "" {
			return fmt.Errorf("failed to pull image %s: %s", ref, msg.Error)
		}
	}
}

// Close закрывает docker-клиент
func (e *Executor) Close() error {
	if e.dockerClient != nil {
		return e.dockerClient.Close()
	}
	return nil
}
