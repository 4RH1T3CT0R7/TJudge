package executor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/docker/docker/client"
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
