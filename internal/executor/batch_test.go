package executor

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// запись sandbox.sh --batch об одном матче
func batchRecordText(k int, status string, code int, stdout, stderr string) string {
	return fmt.Sprintf("@@begin %d %s %d\n%s\n@@stderr\n%s\n@@end\n", k, status, code,
		base64.StdEncoding.EncodeToString([]byte(stdout)), base64.StdEncoding.EncodeToString([]byte(stderr)))
}

func TestParseBatchOutput(t *testing.T) {
	out := batchRecordText(0, "done", 0, "10 5\n", "") +
		// base64 sandbox.sh переносит по 76 символов
		"@@begin 1 done 1\n" + "QQ==\n@@stderr\nbGVmdCBwbGF5ZXIg\nZXJyb3I=\n@@end\n" +
		// битый base64: запись пропускается, матч считается несыгранным
		"@@begin 2 done 0\n!!!\n@@stderr\n@@end\n" +
		// обрезанная запись без @@end
		"@@begin 3 done 0\nMTAgNQ==\n"

	records := parseBatchOutput(out)

	require.Len(t, records, 2)
	assert.Equal(t, &batchRecord{status: "done", code: 0, stdout: "10 5\n"}, records[0])
	assert.Equal(t, &batchRecord{status: "done", code: 1, stdout: "A", stderr: "left player error"}, records[1])

	// вторая запись того же матча - подделка: матч не считается сыгранным
	records = parseBatchOutput(batchRecordText(0, "done", 0, "10 5\n", "") + batchRecordText(0, "done", 0, "99 0\n", "") +
		batchRecordText(0, "done", 0, "99 0\n", "") + batchRecordText(1, "done", 0, "1 1\n", ""))
	assert.NotContains(t, records, 0)
	assert.Contains(t, records, 1)
}

// контейнер пачки: создаётся с командой --batch, отдаёт записи в логах
func batchDocker(t *testing.T, stdout string, cmd chan<- []string) func(w http.ResponseWriter, r *http.Request, path string) {
	return func(w http.ResponseWriter, r *http.Request, path string) {
		switch {
		case path == "/containers/create":
			var body struct{ Cmd []string }
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			cmd <- body.Cmd
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"Id":"b1","Warnings":[]}`))
		case strings.HasSuffix(path, "/wait"):
			_, _ = w.Write([]byte(`{"StatusCode":0}`))
		case strings.HasSuffix(path, "/logs"):
			w.Header().Set("Content-Type", "application/vnd.docker.multiplexed-stream")
			_, _ = stdcopy.NewStdWriter(w, stdcopy.Stdout).Write([]byte(stdout))
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

func TestExecuteBatch(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a", "b", "c"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o700))
	}
	stdout := batchRecordText(0, "done", 0, "10 5\n", "") +
		batchRecordText(1, "done", 1, "", "left player error: crash\n") +
		batchRecordText(2, "timeout", 124, "", "") +
		batchRecordText(3, "skipped", 0, "", "")
	// записи 4 нет: контейнер не дошёл до матча
	cmd := make(chan []string, 1)
	e, calls := fakeDocker(t, config.ExecutorConfig{Timeout: 30 * time.Second, DefaultIterations: 100}, batchDocker(t, stdout, cmd))
	e.programsPath, e.hostProgramsPath = dir, dir

	match := func() *models.Match { return &models.Match{ID: uuid.New(), GameType: "dilemma"} }
	items := []BatchItem{
		{Match: match(), Program1Path: dir + "/a", Program2Path: dir + "/b"},
		{Match: match(), Program1Path: dir + "/b", Program2Path: dir + "/a"},
		// программы нет на диске: ошибка этого матча, в контейнер он не идёт
		{Match: match(), Program1Path: dir + "/a", Program2Path: dir + "/missing"},
		{Match: match(), Program1Path: dir + "/a", Program2Path: dir + "/c"},
		{Match: match(), Program1Path: dir + "/c", Program2Path: dir + "/a"},
		{Match: match(), Program1Path: dir + "/b", Program2Path: dir + "/c"},
	}

	results, err := e.ExecuteBatch(context.Background(), items)
	require.NoError(t, err)
	require.Len(t, results, len(items))

	assert.Equal(t, []string{"--batch", "30", "-i", "100", "--",
		"dilemma", "/programs/a", "/programs/b",
		"dilemma", "/programs/b", "/programs/a",
		"dilemma", "/programs/a", "/programs/c",
		"dilemma", "/programs/c", "/programs/a",
		"dilemma", "/programs/b", "/programs/c",
	}, <-cmd)

	require.NoError(t, results[0].Err)
	assert.Equal(t, [3]int{10, 5, 1}, [3]int{results[0].Result.Score1, results[0].Result.Score2, results[0].Result.Winner})
	assert.Equal(t, items[0].Match.ID, results[0].Result.MatchID)

	// ошибка программы - результат с победой соперника, как у одиночного
	require.NoError(t, results[1].Err)
	assert.Equal(t, 1, results[1].Result.ErrorCode)
	assert.Equal(t, 2, results[1].Result.Winner)
	assert.Contains(t, results[1].Result.ErrorMessage, "crash")

	require.Error(t, results[2].Err)
	assert.False(t, IsInfraError(results[2].Err))

	// свой полный лимит - вина программы
	require.Error(t, results[3].Err)
	assert.False(t, IsInfraError(results[3].Err))
	assert.Contains(t, results[3].Err.Error(), "timeout")

	// не сыгран (время пачки, нет записи) - повторяется
	for _, i := range []int{4, 5} {
		require.Error(t, results[i].Err)
		assert.True(t, IsInfraError(results[i].Err), "матч %d", i)
	}
	assert.Contains(t, calls(), "DELETE /containers/b1")
}

// контейнер пачки не уложился в свой лимит: матчи без записи повторяются, без
// вины программ
func TestExecuteBatch_ContainerTimeoutIsInfra(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a"), []byte("#!/bin/sh\n"), 0o700))
	e, _ := fakeDocker(t, config.ExecutorConfig{Timeout: time.Second}, hangingMatch)
	e.programsPath, e.hostProgramsPath = dir, dir

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	start := time.Now()
	results, err := e.ExecuteBatch(ctx, []BatchItem{{Match: &models.Match{ID: uuid.New(), GameType: "dilemma"}, Program1Path: dir + "/a", Program2Path: dir + "/a"}})

	require.NoError(t, err)
	require.Error(t, results[0].Err)
	assert.True(t, IsInfraError(results[0].Err))
	// лимит контейнера пачки - лимит матча плюс запас
	assert.GreaterOrEqual(t, time.Since(start), time.Second+batchMargin)
}
