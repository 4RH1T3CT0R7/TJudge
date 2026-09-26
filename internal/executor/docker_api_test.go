package executor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var apiVersionRe = regexp.MustCompile(`^/v[0-9.]+`)

// fakeDocker поднимает подделку Docker Engine API: handler получает метод и
// путь без префикса версии. возвращает executor на этом клиенте и журнал запросов
func fakeDocker(t *testing.T, cfg config.ExecutorConfig, handler func(w http.ResponseWriter, r *http.Request, path string)) (*Executor, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := apiVersionRe.ReplaceAllString(r.URL.Path, "")
		mu.Lock()
		calls = append(calls, r.Method+" "+path)
		mu.Unlock()
		handler(w, r, path)
	}))
	t.Cleanup(srv.Close)

	cli, err := client.NewClientWithOpts(
		client.WithHost("tcp://"+strings.TrimPrefix(srv.URL, "http://")),
		client.WithVersion("1.47"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cli.Close() })

	log, _ := logger.New("error", "json")
	e := &Executor{config: cfg, dockerClient: cli, containerPath: "/programs", log: log}
	return e, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), calls...)
	}
}

// контейнер матча, который не завершается сам
func hangingMatch(w http.ResponseWriter, r *http.Request, path string) {
	switch {
	case path == "/containers/create":
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"Id":"c1","Warnings":[]}`))
	case strings.HasSuffix(path, "/wait"):
		<-r.Context().Done()
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// собственный лимит матча истёк - виновата программа, результат терминальный
func TestRunInDocker_OwnTimeoutIsProgramError(t *testing.T) {
	e, calls := fakeDocker(t, config.ExecutorConfig{Timeout: 400 * time.Millisecond}, hangingMatch)

	_, err := e.runInDocker(context.Background(), "dilemma", "/programs/a", "/programs/b", nil)

	require.Error(t, err)
	assert.False(t, IsInfraError(err))
	assert.Contains(t, err.Error(), "timeout")
	assert.Contains(t, calls(), "DELETE /containers/c1", "контейнер удаляется и по таймауту")
}

// отменён ctx воркера (shutdown, общий таймаут) - программа не виновата,
// матч должен вернуться в pending, а не получить таймаут
func TestRunInDocker_ParentCancelIsInfra(t *testing.T) {
	e, calls := fakeDocker(t, config.ExecutorConfig{Timeout: time.Minute}, hangingMatch)

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	_, err := e.runInDocker(ctx, "dilemma", "/programs/a", "/programs/b", nil)

	require.Error(t, err)
	assert.True(t, IsInfraError(err))
	assert.Contains(t, calls(), "DELETE /containers/c1")
}

// точка входа песочницы и root заданы явно: без них образ запустил бы
// tjudge-cli и ботов под одним uid
func TestRunInDocker_SandboxEntrypoint(t *testing.T) {
	type createBody struct {
		Entrypoint []string
		User       string
	}
	created := make(chan createBody, 1)
	e, _ := fakeDocker(t, config.ExecutorConfig{Timeout: 200 * time.Millisecond}, func(w http.ResponseWriter, r *http.Request, path string) {
		if path == "/containers/create" {
			var body createBody
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			created <- body
		}
		hangingMatch(w, r, path)
	})

	_, _ = e.runInDocker(context.Background(), "dilemma", "/programs/a", "/programs/b", nil)

	body := <-created
	assert.Equal(t, []string{sandboxEntrypoint}, body.Entrypoint)
	assert.Equal(t, "0:0", body.User)
}

// compilerOn - компилятор на том же подставном докере, каталог программ временный
func compilerOn(e *Executor, dir string, timeout time.Duration) *Compiler {
	return &Compiler{dockerClient: e.dockerClient, programsPath: dir, hostPrograms: dir, compileTimeout: timeout, log: e.log}
}

// сборка упёрлась в свой лимит времени - вина программы, а не инфры
func TestRunBuilder_OwnTimeoutIsCompileError(t *testing.T) {
	e, calls := fakeDocker(t, config.ExecutorConfig{}, hangingMatch)
	dir := t.TempDir()
	c := compilerOn(e, dir, 400*time.Millisecond)

	code, out, err := c.runBuilder(context.Background(), []string{"gcc"}, dir)

	require.NoError(t, err)
	assert.Equal(t, int64(1), code)
	assert.Contains(t, out, "лимит времени")
	assert.Contains(t, calls(), "DELETE /containers/c1")
}

// остановка воркера посреди сборки - программа остаётся compiling, сборку повторят
func TestRunBuilder_ParentCancelIsInfra(t *testing.T) {
	e, calls := fakeDocker(t, config.ExecutorConfig{}, hangingMatch)
	dir := t.TempDir()
	c := compilerOn(e, dir, time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	_, _, err := c.runBuilder(ctx, []string{"gcc"}, dir)

	require.Error(t, err)
	assert.True(t, IsInfraError(err))
	assert.Contains(t, calls(), "DELETE /containers/c1")
}

// "компилятор" записал бинарник больше maxArtifactSize - сборка отвергается,
// бинарник на место программы не ставится
func TestCompile_RejectsOversizedArtifact(t *testing.T) {
	e, _ := fakeDocker(t, config.ExecutorConfig{}, func(w http.ResponseWriter, r *http.Request, path string) {
		switch {
		case path == "/containers/create":
			var body struct{ HostConfig struct{ Binds []string } }
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			hostDir, _, _ := strings.Cut(body.HostConfig.Binds[0], ":")
			f, err := os.Create(filepath.Join(hostDir, "out"))
			if assert.NoError(t, err) {
				assert.NoError(t, f.Truncate(maxArtifactSize+1))
				_ = f.Close()
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"Id":"c1","Warnings":[]}`))
		case strings.HasSuffix(path, "/wait"):
			_, _ = w.Write([]byte(`{"StatusCode":0}`))
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	})
	dir := t.TempDir()
	src := filepath.Join(dir, "bot.c")
	require.NoError(t, os.WriteFile(src, []byte("int main(){}"), 0o600))

	res, err := compilerOn(e, dir, time.Minute).Compile(context.Background(),
		&models.Program{ID: uuid.New(), Language: "c", CodePath: src})

	require.NoError(t, err)
	assert.False(t, res.OK)
	assert.Contains(t, res.Log, "результат сборки больше")
	assert.NoFileExists(t, filepath.Join(dir, "bot"))
}

func TestEnsureImage(t *testing.T) {
	t.Run("есть локально", func(t *testing.T) {
		e, calls := fakeDocker(t, config.ExecutorConfig{}, func(w http.ResponseWriter, _ *http.Request, _ string) {
			_, _ = w.Write([]byte(`{"Id":"sha256:1"}`))
		})
		require.NoError(t, e.EnsureImage(context.Background(), "tjudge-cli:latest"))
		assert.Equal(t, []string{"GET /images/tjudge-cli:latest/json"}, calls())
	})

	t.Run("нет локально - pull", func(t *testing.T) {
		e, calls := fakeDocker(t, config.ExecutorConfig{}, func(w http.ResponseWriter, _ *http.Request, path string) {
			if path == "/images/create" {
				_, _ = w.Write([]byte(`{"status":"Pulling"}` + "\n" + `{"status":"Downloaded"}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"No such image"}`))
		})
		require.NoError(t, e.EnsureImage(context.Background(), "ghcr.io/x/tjudge-executor:1.0"))
		assert.Contains(t, calls(), "POST /images/create")
	})

	t.Run("ошибка посреди pull", func(t *testing.T) {
		e, _ := fakeDocker(t, config.ExecutorConfig{}, func(w http.ResponseWriter, _ *http.Request, path string) {
			if path == "/images/create" {
				_, _ = w.Write([]byte(`{"status":"Pulling"}` + "\n" + `{"error":"pull access denied"}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"No such image"}`))
		})
		err := e.EnsureImage(context.Background(), "tjudge-builder:latest")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pull access denied")
	})
}

// на старте удаляются контейнеры этого воркера и мёртвых воркеров, контейнеры
// живой соседней реплики остаются
func TestRemoveOrphans(t *testing.T) {
	list := `[
		{"Id":"own","Labels":{"tjudge.managed":"true","tjudge.owner":"` + ownerID + `"}},
		{"Id":"dead","Labels":{"tjudge.managed":"true","tjudge.owner":"gone"}},
		{"Id":"alive","Labels":{"tjudge.managed":"true","tjudge.owner":"sibling"}}
	]`
	e, calls := fakeDocker(t, config.ExecutorConfig{}, func(w http.ResponseWriter, r *http.Request, path string) {
		switch path {
		case "/containers/json":
			assert.Contains(t, r.URL.Query().Get("filters"), "tjudge.managed=true")
			_, _ = w.Write([]byte(list))
		case "/containers/sibling/json":
			_, _ = w.Write([]byte(`{"Id":"sibling","State":{"Running":true}}`))
		case "/containers/gone/json":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"No such container"}`))
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	})

	removed, err := e.RemoveOrphans(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 2, removed)
	got := calls()
	assert.Contains(t, got, "DELETE /containers/own")
	assert.Contains(t, got, "DELETE /containers/dead")
	assert.NotContains(t, got, "DELETE /containers/alive")
}
