//go:build integration

package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// разведчик сотрудничает, только если ни одна атака на соперника и
// tjudge-cli не удалась, иначе предаёт: утечка видна по счёту матча
const scoutBot = `#!/usr/bin/env python3
import os, signal, socket

move = "COOPERATE"

def attack(f):
    global move
    try:
        f()
        move = "DEFECT"
    except OSError:
        pass

attack(lambda: open("/programs/OPP", "rb").read())
attack(lambda: os.listdir("/programs"))
attack(lambda: os.listdir("/mnt/programs"))
attack(lambda: open("/usr/local/bin/tjudge-cli", "ab"))
attack(lambda: socket.create_connection(("1.1.1.1", 53), 1))
for pid in os.listdir("/proc"):
    if pid.isdigit() and int(pid) != os.getpid():
        attack(lambda: open("/proc/%s/environ" % pid, "rb").read())
        attack(lambda: os.kill(int(pid), 0))
if os.stat("/programs/OPP").st_uid == os.getuid():
    move = "DEFECT"
# kill(-1) успешен, даже если никого не задел; удачный выстрел
# виден по упавшему сопернику
try:
    os.kill(-1, signal.SIGKILL)
except OSError:
    pass

n = int(input())
for _ in range(n):
    print(move, flush=True)
    input()
`

const cooperatorBot = `#!/bin/sh
read -r n
i=0
while [ "$i" -lt "$n" ]; do
    echo COOPERATE
    read -r _
    i=$((i + 1))
done
`

// живой матч на настоящем докере с образом из TJUDGE_SANDBOX_IMAGE (например
// tjudge-cli:latest после compose build), без переменной тест пропускается.
// проверяет всю цепочку: конфиг контейнера из runInDocker и точку входа образа
func TestSandbox_BotsIsolated(t *testing.T) {
	image := os.Getenv("TJUDGE_SANDBOX_IMAGE")
	if image == "" {
		t.Skip("TJUDGE_SANDBOX_IMAGE не задан")
	}

	dir := t.TempDir()
	scout := filepath.Join(dir, "scout.py")
	coop := filepath.Join(dir, "coop")
	require.NoError(t, os.WriteFile(scout, []byte(strings.ReplaceAll(scoutBot, "OPP", "coop")), 0o700))
	require.NoError(t, os.WriteFile(coop, []byte(cooperatorBot), 0o700))

	log, _ := logger.New("error", "json")
	e, err := NewExecutor(config.ExecutorConfig{
		DockerImage:       image,
		Timeout:           time.Minute,
		CPUQuota:          100000,
		MemoryLimit:       256 << 20,
		PidsLimit:         64,
		DefaultIterations: 10,
	}, dir, "", log)
	require.NoError(t, err)

	// разведчик побывает и первым, и вторым игроком
	for _, pair := range [][2]string{{scout, coop}, {coop, scout}} {
		res, err := e.Execute(context.Background(), &models.Match{ID: uuid.New(), GameType: "dilemma"}, pair[0], pair[1])
		require.NoError(t, err)
		assert.Equal(t, 0, res.ErrorCode, res.ErrorMessage)
		assert.Equal(t, [2]int{50, 50}, [2]int{res.Score1, res.Score2}, "бот добрался до соперника")
	}
}

// сотрудничает и на каждом ходу пишет в stderr 20 КБ: за матч больше буфера
// pipe, который tjudge-cli не читает
const chattyBot = `#!/usr/bin/env python3
import sys
n = int(input())
for i in range(n):
    sys.stderr.write("SECRET-%d " % i + "x" * 20000 + "\n")
    print("COOPERATE", flush=True)
    input()
`

const crashBot = `#!/usr/bin/env python3
n = int(input())
print("COOPERATE", flush=True)
input()
1 / 0
`

func newLiveExecutor(t *testing.T, dir string) *Executor {
	t.Helper()
	image := os.Getenv("TJUDGE_SANDBOX_IMAGE")
	if image == "" {
		t.Skip("TJUDGE_SANDBOX_IMAGE не задан")
	}
	log, _ := logger.New("error", "json")
	e, err := NewExecutor(config.ExecutorConfig{
		DockerImage:       image,
		Timeout:           time.Minute,
		CPUQuota:          100000,
		MemoryLimit:       256 << 20,
		PidsLimit:         64,
		DefaultIterations: 10,
	}, dir, "", log)
	require.NoError(t, err)
	return e
}

func writeBots(t *testing.T, dir string, bots map[string]string) {
	t.Helper()
	for name, src := range bots {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(src), 0o700))
	}
}

// отладочный вывод в stderr больше не вешает бота, а в текст ошибки попадает
// хвост stderr только упавшей стороны
func TestSandbox_BotStderr(t *testing.T) {
	dir := t.TempDir()
	e := newLiveExecutor(t, dir)
	writeBots(t, dir, map[string]string{"chatty.py": chattyBot, "crash.py": crashBot, "coop": cooperatorBot})
	run := func(p1, p2 string) *models.MatchResult {
		res, err := e.Execute(context.Background(), &models.Match{ID: uuid.New(), GameType: "dilemma"},
			filepath.Join(dir, p1), filepath.Join(dir, p2))
		require.NoError(t, err)
		return res
	}

	res := run("chatty.py", "coop")
	assert.Equal(t, 0, res.ErrorCode, res.ErrorMessage)
	assert.Equal(t, [2]int{50, 50}, [2]int{res.Score1, res.Score2})

	for side, pair := range [][2]string{{"crash.py", "chatty.py"}, {"chatty.py", "crash.py"}} {
		res = run(pair[0], pair[1])
		assert.Equal(t, side+1, res.ErrorCode)
		assert.Contains(t, res.ErrorMessage, "--- stderr программы (последние 2 КБ) ---")
		assert.Contains(t, res.ErrorMessage, "ZeroDivisionError: division by zero")
		assert.NotContains(t, res.ErrorMessage, "SECRET", "в ошибку попал stderr соперника")
	}
}

// самопроверка: программа играет против эталонного бота из образа
func TestSandbox_Check(t *testing.T) {
	dir := t.TempDir()
	e := newLiveExecutor(t, dir)
	writeBots(t, dir, map[string]string{"crash.py": crashBot, "coop": cooperatorBot})

	res, err := e.Check(context.Background(), "dilemma", filepath.Join(dir, "coop"))
	require.NoError(t, err)
	assert.Equal(t, 0, res.ErrorCode, res.ErrorMessage)
	assert.Equal(t, [2]int{50, 50}, [2]int{res.Score1, res.Score2}, "эталон дилеммы - tit for tat")

	res, err = e.Check(context.Background(), "dilemma", filepath.Join(dir, "crash.py"))
	require.NoError(t, err)
	assert.Equal(t, 1, res.ErrorCode)
	assert.Contains(t, res.ErrorMessage, "ZeroDivisionError")
}
