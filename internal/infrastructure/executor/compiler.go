package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"go.uber.org/zap"
)

// Compiler компилирует загруженные программы в изолированном Docker-контейнере
// (builder-образ с тулчейнами). Раньше компиляция шла на хосте API-процесса:
// компиляционная бомба подвешивала HTTP-хендлер, а #include "/etc/passwd"
// выводил содержимое файлов хоста в сообщение об ошибке. В песочнице
// компилятору доступен только каталог сборки конкретной программы.
type Compiler struct {
	dockerClient   *client.Client
	builderImage   string
	programsPath   string // каталог программ в процессе worker'а
	hostPrograms   string // тот же каталог на хосте (Docker-in-Docker)
	compileTimeout time.Duration
	log            *logger.Logger
}

// CompileResult - итог компиляции.
type CompileResult struct {
	OK       bool
	ExecPath string // путь к исполняемому файлу (бинарник, wrapper или исходник)
	Log      string // вывод компилятора при неудаче (обрезан)
}

// compilePlan описывает, как собирать программу конкретного языка.
type compilePlan struct {
	SourceName   string   // имя исходника внутри /build
	Cmd          []string // команда компиляции/проверки в контейнере
	ArtifactName string   // имя артефакта в /build при успехе ("" - артефакт не нужен, исполняется исходник)
}

// buildContainerPath - фиксированный путь каталога сборки внутри builder-контейнера.
const buildContainerPath = "/build"

// matchContainerPath - путь каталога программ внутри контейнера матча
// (см. Executor.containerPath); java-wrapper ссылается на classpath этим путём.
const matchContainerPath = "/programs"

// compileLogLimit - максимальная длина сообщения компилятора, сохраняемого
// в program.error_message и показываемого пользователю.
const compileLogLimit = 1500

// langJava - имя языка Java в domain.Program.Language (goconst).
const langJava = "java"

// javaClassNameRe допускает только валидные Java-идентификаторы (allowlist
// против инъекций в wrapper-скрипт).
var javaClassNameRe = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

// javaClassDeclRe выделяет имя первого top-level класса Java из исходника.
var javaClassDeclRe = regexp.MustCompile(`(?m)^\s*(?:public\s+|final\s+|abstract\s+|static\s+)*class\s+([A-Za-z_$][A-Za-z0-9_$]*)`)

// NewCompiler создаёт sandbox-компилятор. builderImage - Docker-образ
// с тулчейнами (gcc/g++/go/rustc/javac + интерпретаторы для syntax-check).
func NewCompiler(builderImage, programsPath, hostProgramsPath string, compileTimeout time.Duration, log *logger.Logger) (*Compiler, error) {
	dockerClient, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	if hostProgramsPath == "" {
		hostProgramsPath = programsPath
	}
	if compileTimeout <= 0 {
		compileTimeout = 120 * time.Second
	}

	return &Compiler{
		dockerClient:   dockerClient,
		builderImage:   builderImage,
		programsPath:   filepath.Clean(programsPath),
		hostPrograms:   filepath.Clean(hostProgramsPath),
		compileTimeout: compileTimeout,
		log:            log,
	}, nil
}

// Close освобождает docker-клиент.
func (c *Compiler) Close() error {
	return c.dockerClient.Close()
}

// extractJavaClassName возвращает имя первого top-level класса в Java-исходнике.
func extractJavaClassName(source string) string {
	// Убираем комментарии, чтобы не зацепить class в /* ... */ или //.
	blockRe := regexp.MustCompile(`/\*[\s\S]*?\*/`)
	cleaned := blockRe.ReplaceAllString(source, "")
	lineRe := regexp.MustCompile(`//[^\n]*`)
	cleaned = lineRe.ReplaceAllString(cleaned, "")

	m := javaClassDeclRe.FindStringSubmatch(cleaned)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// buildCompilePlan возвращает план сборки для языка. className нужен только
// для Java. Чистая функция - покрыта unit-тестами без Docker.
func buildCompilePlan(language, className string) (*compilePlan, error) {
	bp := func(name string) string { return buildContainerPath + "/" + name }

	switch language {
	case "c":
		return &compilePlan{
			SourceName:   "main.c",
			Cmd:          []string{"gcc", "-O2", "-o", bp("out"), bp("main.c"), "-lm"},
			ArtifactName: "out",
		}, nil
	case "cpp":
		return &compilePlan{
			SourceName:   "main.cpp",
			Cmd:          []string{"g++", "-O2", "-o", bp("out"), bp("main.cpp")},
			ArtifactName: "out",
		}, nil
	case "go":
		return &compilePlan{
			SourceName:   "main.go",
			Cmd:          []string{"go", "build", "-o", bp("out"), bp("main.go")},
			ArtifactName: "out",
		}, nil
	case "rust":
		return &compilePlan{
			SourceName:   "main.rs",
			Cmd:          []string{"rustc", "-O", "-o", bp("out"), bp("main.rs")},
			ArtifactName: "out",
		}, nil
	case langJava:
		if className == "" {
			return nil, fmt.Errorf("в Java-файле не найдено объявление class X")
		}
		if !javaClassNameRe.MatchString(className) {
			return nil, fmt.Errorf("недопустимое имя Java-класса: %q", className)
		}
		return &compilePlan{
			SourceName:   className + ".java",
			Cmd:          []string{"javac", bp(className + ".java")},
			ArtifactName: className + ".class",
		}, nil
	case "python":
		return &compilePlan{SourceName: "main.py", Cmd: []string{"python3", "-m", "py_compile", bp("main.py")}}, nil
	case "javascript":
		return &compilePlan{SourceName: "main.js", Cmd: []string{"node", "--check", bp("main.js")}}, nil
	case "ruby":
		return &compilePlan{SourceName: "main.rb", Cmd: []string{"ruby", "-c", bp("main.rb")}}, nil
	case "php":
		return &compilePlan{SourceName: "main.php", Cmd: []string{"php", "-l", bp("main.php")}}, nil
	case "lua":
		return &compilePlan{SourceName: "main.lua", Cmd: []string{"luac", "-p", bp("main.lua")}}, nil
	default:
		return nil, fmt.Errorf("неподдерживаемый язык: %s", language)
	}
}

// Compile собирает программу в изолированном контейнере.
//
// Возвращает (result, nil) и для успеха, и для ошибки компиляции -
// различаются полем OK. Ошибка возвращается только при инфраструктурных
// проблемах (Docker недоступен, образ отсутствует): такие задачи безопасно
// повторить позже, программа остаётся в compiling.
func (c *Compiler) Compile(ctx context.Context, program *domain.Program) (*CompileResult, error) {
	sourcePath := program.CodePath
	if program.FilePath != nil && *program.FilePath != "" {
		sourcePath = *program.FilePath
	}

	// Java: имя класса нужно до построения плана.
	className := ""
	if program.Language == langJava {
		// #nosec G304 -- sourcePath сформирован сервером из UUID-компонентов.
		srcBytes, err := os.ReadFile(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read source: %w", err)
		}
		className = extractJavaClassName(string(srcBytes))
	}

	plan, err := buildCompilePlan(program.Language, className)
	if err != nil {
		// Ошибка плана (нет class-декларации, неизвестный язык) - вина программы.
		return &CompileResult{OK: false, Log: err.Error()}, nil
	}

	// Изолированный каталог сборки: компилятору доступен ТОЛЬКО он.
	// Монтировать весь каталог программ нельзя - #include "../чужая_команда.c"
	// читал бы исходники других команд.
	buildDir := filepath.Join(c.programsPath, "build", program.ID.String())
	if err := os.MkdirAll(buildDir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create build dir: %w", err)
	}
	defer os.RemoveAll(buildDir)

	if err := copyFile(sourcePath, filepath.Join(buildDir, plan.SourceName)); err != nil {
		return nil, fmt.Errorf("failed to copy source to build dir: %w", err)
	}

	exitCode, output, err := c.runBuilder(ctx, plan.Cmd, buildDir)
	if err != nil {
		return nil, err // инфраструктурная ошибка - задача будет повторена
	}

	if exitCode != 0 {
		logMsg := strings.TrimSpace(output)
		if logMsg == "" {
			logMsg = fmt.Sprintf("компиляция завершилась с кодом %d", exitCode)
		}
		// Пути из контейнера не несут смысла для пользователя - подчищаем.
		logMsg = strings.ReplaceAll(logMsg, buildContainerPath+"/", "")
		if len(logMsg) > compileLogLimit {
			logMsg = logMsg[:compileLogLimit] + "..."
		}
		return &CompileResult{OK: false, Log: logMsg}, nil
	}

	execPath, err := c.installArtifact(program, plan, buildDir, sourcePath, className)
	if err != nil {
		return nil, err
	}

	return &CompileResult{OK: true, ExecPath: execPath}, nil
}

// installArtifact переносит результат сборки из buildDir на постоянное место
// и возвращает путь к исполняемому файлу.
func (c *Compiler) installArtifact(program *domain.Program, plan *compilePlan, buildDir, sourcePath, className string) (string, error) {
	// Интерпретируемые языки: артефакта нет, исполняется исходник.
	if plan.ArtifactName == "" {
		return sourcePath, nil
	}

	ext := filepath.Ext(sourcePath)
	binPath := strings.TrimSuffix(sourcePath, ext)

	if program.Language == langJava {
		// .class кладём в каталог программы (имена классов разных команд
		// конфликтуют в плоском каталоге), wrapper ссылается на путь внутри
		// контейнера матча.
		classDirName := filepath.Base(binPath) + "_classes"
		classDir := filepath.Join(filepath.Dir(sourcePath), classDirName)
		if err := os.MkdirAll(classDir, 0o750); err != nil {
			return "", fmt.Errorf("failed to create class dir: %w", err)
		}
		// Переносим все .class (включая вложенные классы Foo$Bar.class).
		entries, err := os.ReadDir(buildDir)
		if err != nil {
			return "", fmt.Errorf("failed to read build dir: %w", err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".class") {
				continue
			}
			if err := os.Rename(filepath.Join(buildDir, e.Name()), filepath.Join(classDir, e.Name())); err != nil {
				return "", fmt.Errorf("failed to move class file: %w", err)
			}
		}

		containerClassDir := matchContainerPath + "/" + classDirName
		wrapper := fmt.Sprintf("#!/bin/sh\nexec java -cp '%s' %s \"$@\"\n", containerClassDir, className)
		// #nosec G306 -- wrapper должен быть исполняемым; 0o750 owner+group.
		if err := os.WriteFile(binPath, []byte(wrapper), 0o750); err != nil {
			return "", fmt.Errorf("failed to write java wrapper: %w", err)
		}
		return binPath, nil
	}

	if err := os.Rename(filepath.Join(buildDir, plan.ArtifactName), binPath); err != nil {
		return "", fmt.Errorf("failed to move binary: %w", err)
	}
	// #nosec G302 -- бинарник исполняется в Docker-sandbox; 0o750 owner+group.
	if err := os.Chmod(binPath, 0o750); err != nil {
		c.log.Warn("Failed to chmod compiled binary", zap.Error(err), zap.String("path", binPath))
	}

	return binPath, nil
}

// runBuilder запускает builder-контейнер и возвращает exit-code и логи.
// Любая ошибка Docker API - инфраструктурная (err != nil).
func (c *Compiler) runBuilder(ctx context.Context, cmd []string, buildDir string) (int64, string, error) {
	execCtx, cancel := context.WithTimeout(ctx, c.compileTimeout)
	defer cancel()

	// Путь buildDir на хосте для Docker-in-Docker.
	rel, err := filepath.Rel(c.programsPath, buildDir)
	if err != nil {
		return 0, "", fmt.Errorf("build dir outside programs path: %w", err)
	}
	hostBuildDir := filepath.Join(c.hostPrograms, rel)

	containerConfig := &container.Config{
		Image: c.builderImage,
		Cmd:   cmd,
		Env: []string{
			// Компиляторам нужен writable scratch; всё в tmpfs.
			"HOME=/tmp",
			"TMPDIR=/tmp",
			"GOCACHE=/tmp/gocache",
			"GOPATH=/tmp/go",
			"GOFLAGS=-mod=mod",
		},
		Tty: false,
	}

	pidsLimit := int64(256)
	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			Memory:     1 << 30, // 1GB: компиляторам (rustc, go) нужно больше, чем матчам
			MemorySwap: 1 << 30,
			PidsLimit:  &pidsLimit,
		},
		Binds: []string{
			fmt.Sprintf("%s:%s:rw", hostBuildDir, buildContainerPath),
		},
		NetworkMode:    "none",
		ReadonlyRootfs: true,
		SecurityOpt:    []string{"no-new-privileges:true"},
		CapDrop:        []string{"ALL"},
		Tmpfs: map[string]string{
			"/tmp": "rw,nosuid,size=512m",
		},
		AutoRemove: false,
	}

	resp, err := c.dockerClient.ContainerCreate(execCtx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return 0, "", infraErrorf("failed to create builder container: %w", err)
	}
	containerID := resp.ID
	defer func() {
		removeCtx, removeCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer removeCancel()
		_ = c.dockerClient.ContainerRemove(removeCtx, containerID, container.RemoveOptions{Force: true})
	}()

	if err := c.dockerClient.ContainerStart(execCtx, containerID, container.StartOptions{}); err != nil {
		return 0, "", infraErrorf("failed to start builder container: %w", err)
	}

	statusCh, errCh := c.dockerClient.ContainerWait(execCtx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return 0, "", infraErrorf("error waiting for builder container: %w", err)
		}
		return 0, "", infraErrorf("builder container: wait returned nil error without status")
	case status := <-statusCh:
		output := c.builderLogs(containerID)
		return status.StatusCode, output, nil
	case <-execCtx.Done():
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer stopCancel()
		_ = c.dockerClient.ContainerStop(stopCtx, containerID, container.StopOptions{})
		// Таймаут компиляции - вина программы (компиляционная бомба), не инфры.
		return 1, "компиляция превысила лимит времени", nil
	}
}

// builderLogs возвращает объединённый stdout+stderr контейнера (best-effort).
func (c *Compiler) builderLogs(containerID string) string {
	logCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logs, err := c.dockerClient.ContainerLogs(logCtx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return ""
	}
	defer logs.Close()

	data, _ := io.ReadAll(io.LimitReader(logs, 64<<10))
	return stripDockerLogHeaders(data)
}

// stripDockerLogHeaders убирает 8-байтовые заголовки docker-мультиплексора
// из сырого потока логов (без TTY docker пишет фреймы header+payload).
func stripDockerLogHeaders(data []byte) string {
	var sb strings.Builder
	for len(data) >= 8 {
		size := int(uint32(data[4])<<24 | uint32(data[5])<<16 | uint32(data[6])<<8 | uint32(data[7]))
		data = data[8:]
		if size > len(data) {
			size = len(data)
		}
		sb.Write(data[:size])
		data = data[size:]
	}
	return sb.String()
}

// copyFile копирует файл с правами 0640.
func copyFile(src, dst string) error {
	// #nosec G304 -- оба пути формируются сервером из UUID-компонентов.
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// #nosec G304
	// 0o600: исходник в build-каталоге читает только процесс worker'а
	// (builder-контейнер монтирует каталог от того же uid).
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
