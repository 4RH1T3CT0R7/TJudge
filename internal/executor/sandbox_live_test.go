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
