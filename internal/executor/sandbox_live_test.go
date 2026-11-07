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

// перед падением пишет в stderr строку под вывод судьи: в транскрипт она
// попасть не должна
const crashBot = `#!/usr/bin/env python3
import sys
n = int(input())
print("COOPERATE", flush=True)
input()
sys.stderr.write("[<] decision: Defect\n")
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
		Verbose:           true,
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
	require.NotNil(t, res.Transcript)
	assert.Len(t, res.Transcript.Moves[0], 10)
	assert.Equal(t, res.Transcript.Moves[0], res.Transcript.Moves[1])
	assert.Len(t, res.Transcript.Points[1], 10)

	// ходы обрываются на падении: второй игрок при этом уже не ходит
	moves := [][][]int{{{1}, {1}}, {{1, 1}, {1}}}
	for side, pair := range [][2]string{{"crash.py", "chatty.py"}, {"chatty.py", "crash.py"}} {
		res = run(pair[0], pair[1])
		assert.Equal(t, side+1, res.ErrorCode)
		assert.Contains(t, res.ErrorMessage, "--- stderr программы (последние 2 КБ) ---")
		assert.Contains(t, res.ErrorMessage, "ZeroDivisionError: division by zero")
		assert.NotContains(t, res.ErrorMessage, "SECRET", "в ошибку попал stderr соперника")
		assert.NotContains(t, res.ErrorMessage, "[iter-00]", "в ошибку попал вывод -v")
		require.NotNil(t, res.Transcript)
		assert.Equal(t, moves[side], res.Transcript.Moves, "в транскрипт попал stderr бота")
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

// бот на ruby сотрудничает, только если RubyGems не загружен
const rubyNoGemsBot = `#!/usr/bin/env ruby
abort "RubyGems загружен" if defined?(Gem)
STDOUT.sync = true
gets.to_i.times do
  puts "COOPERATE"
  gets
end
`

// ruby стартует без RubyGems: с ним первый ход занимает 50-400 мс и на
// холодном или загруженном хосте не укладывается в 200 мс судьи
func TestSandbox_RubyWithoutGems(t *testing.T) {
	dir := t.TempDir()
	e := newLiveExecutor(t, dir)
	writeBots(t, dir, map[string]string{"bot.rb": rubyNoGemsBot, "coop": cooperatorBot})

	res, err := e.Execute(context.Background(), &models.Match{ID: uuid.New(), GameType: "dilemma"},
		filepath.Join(dir, "coop"), filepath.Join(dir, "bot.rb"))
	require.NoError(t, err)
	assert.Equal(t, 0, res.ErrorCode, res.ErrorMessage)
	assert.Equal(t, [2]int{50, 50}, [2]int{res.Score1, res.Score2})
}

// бот, которому на запуск нужно 0,5 с (JVM или интерпретатор на загруженной
// машине): первый ход ждёт запуска до 2 с, дальше ходы по 200 мс
const slowStartBot = `#!/bin/sh
sleep 0.5
read -r n
i=0
while [ "$i" -lt "$n" ]; do
    echo COOPERATE
    read -r _
    i=$((i + 1))
done
`

func TestSandbox_SlowStart(t *testing.T) {
	dir := t.TempDir()
	e := newLiveExecutor(t, dir)
	writeBots(t, dir, map[string]string{"slow": slowStartBot, "coop": cooperatorBot})

	res, err := e.Execute(context.Background(), &models.Match{ID: uuid.New(), GameType: "dilemma"},
		filepath.Join(dir, "slow"), filepath.Join(dir, "coop"))
	require.NoError(t, err)
	assert.Equal(t, 0, res.ErrorCode, res.ErrorMessage)
	assert.Equal(t, [2]int{50, 50}, [2]int{res.Score1, res.Score2})
}

// оставляет следы, которые пережили бы матч в общем контейнере: файлы в /tmp
// и /dev/shm (в каталоге без прав и в sticky-каталоге), объекты SysV IPC и
// отвязанные неотслеживаемые процессы, которые выбирают весь RLIMIT_NPROC
// своего uid: так команда уборки от этого uid не смогла бы даже запуститься.
// потом обычный кооператор
const planterBot = `#!/usr/bin/env python3
import ctypes, os
for d in ("/tmp", "/dev/shm"):
    with open(d + "/planted", "w") as f:
        f.write("secret")
    os.chmod(d + "/planted", 0o666)
os.mkdir("/tmp/locked")
open("/tmp/locked/planted", "w").close()
os.chmod("/tmp/locked", 0)
os.mkdir("/tmp/sticky")
open("/tmp/sticky/planted", "w").close()
os.chmod("/tmp/sticky", 0o1777)
libc = ctypes.CDLL(None, use_errno=True)
libc.msgget(4242, 0o1666)
libc.semget(4242, 1, 0o1666)
libc.shmget(4242, 4096, 0o1666)
if os.fork() == 0:
    os.setsid()
    libc.prctl(4, 0)  # PR_SET_DUMPABLE: /proc процесса переходит к root
    while True:
        try:
            if os.fork() == 0:
                libc.prctl(4, 0)
                os.execvp("sleep", ["sleep", "300"])
        except OSError:
            break
    os.execvp("sleep", ["sleep", "300"])
n = int(input())
for _ in range(n):
    print("COOPERATE", flush=True)
    input()
`

// предаёт, если нашёл следы прошлого матча пачки: счёт это покажет
const finderBot = `#!/usr/bin/env python3
import ctypes, os
found = False
for d in ("/tmp", "/dev/shm"):
    if os.listdir(d):
        found = True
for pid in os.listdir("/proc"):
    try:
        if pid.isdigit() and b"sleep" in open("/proc/%s/cmdline" % pid, "rb").read():
            found = True
    except OSError:
        pass
libc = ctypes.CDLL(None, use_errno=True)
if libc.msgget(4242, 0) >= 0 or libc.semget(4242, 0, 0) >= 0 or libc.shmget(4242, 0, 0) >= 0:
    found = True
move = "DEFECT" if found else "COOPERATE"
n = int(input())
for _ in range(n):
    print(move, flush=True)
    input()
`

// пачка в одном контейнере: следы бота не доживают до следующего матча, а
// падение программы касается только её матча
func TestSandbox_BatchIsolated(t *testing.T) {
	dir := t.TempDir()
	e := newLiveExecutor(t, dir)
	writeBots(t, dir, map[string]string{"planter.py": planterBot, "finder.py": finderBot, "crash.py": crashBot, "coop": cooperatorBot})

	pairs := [][2]string{{"planter.py", "coop"}, {"finder.py", "coop"}, {"crash.py", "coop"}, {"coop", "finder.py"}}
	var items []BatchItem
	for _, p := range pairs {
		items = append(items, BatchItem{Match: &models.Match{ID: uuid.New(), GameType: "dilemma"},
			Program1Path: filepath.Join(dir, p[0]), Program2Path: filepath.Join(dir, p[1])})
	}
	results, err := e.ExecuteBatch(context.Background(), items)
	require.NoError(t, err)

	for i, want := range []int{0, 0, 1, 0} {
		require.NoError(t, results[i].Err, "матч %d", i)
		res := results[i].Result
		assert.Equal(t, want, res.ErrorCode, "матч %d: %s", i, res.ErrorMessage)
		if want == 0 {
			assert.Equal(t, [2]int{50, 50}, [2]int{res.Score1, res.Score2}, "матч %d: бот нашёл следы прошлого матча", i)
			require.NotNil(t, res.Transcript, "матч %d", i)
		}
	}
	assert.Contains(t, results[2].Result.ErrorMessage, "ZeroDivisionError")
}

// отвечает в срок каждого хода (200 мс), но матч длиннее лимита
const slowMovesBot = `#!/bin/sh
read -r n
i=0
while [ "$i" -lt "$n" ]; do
    sleep 0.15
    echo COOPERATE
    read -r _
    i=$((i + 1))
done
`

// первый матч пачки получает весь лимит и по нему проигрывает таймаут, на
// следующий времени пачки не осталось: он не сыгран и повторится
func TestSandbox_BatchTimeout(t *testing.T) {
	dir := t.TempDir()
	e := newLiveExecutor(t, dir)
	e.config.Timeout = 2 * time.Second
	e.config.DefaultIterations = 100
	writeBots(t, dir, map[string]string{"slow": slowMovesBot, "coop": cooperatorBot})

	match := func(p1, p2 string) BatchItem {
		return BatchItem{Match: &models.Match{ID: uuid.New(), GameType: "dilemma"},
			Program1Path: filepath.Join(dir, p1), Program2Path: filepath.Join(dir, p2)}
	}
	results, err := e.ExecuteBatch(context.Background(), []BatchItem{match("slow", "coop"), match("coop", "coop")})
	require.NoError(t, err)

	require.Error(t, results[0].Err)
	assert.False(t, IsInfraError(results[0].Err), results[0].Err.Error())
	assert.Contains(t, results[0].Err.Error(), "timeout")
	require.Error(t, results[1].Err)
	assert.True(t, IsInfraError(results[1].Err), results[1].Err.Error())
}
