package executor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// матч-контейнер гоняет чужой код, так что каждый флаг тут - линия обороны.
// тест ловит тихую потерю флага при рефакторе (иначе такое видно только через
// docker inspect, ни один другой юнит их не проверяет)
func TestBuildMatchHostConfig_SandboxFlags(t *testing.T) {
	cfg := config.ExecutorConfig{
		CPUQuota:    100000,
		MemoryLimit: 536870912,
		PidsLimit:   100,
		CPUSetCPUs:  "",
	}
	binds := []string{"/host/programs/a:/mnt/programs/a:ro", "/host/programs/b:/mnt/programs/b:ro"}
	hc := buildMatchHostConfig(cfg, binds)

	assert.Equal(t, []string{"ALL"}, []string(hc.CapDrop), "должны сниматься все capabilities")
	// только то, что нужно точке входа и tjudge-cli для ботов под своими uid
	assert.ElementsMatch(t, []string{"CHOWN", "DAC_OVERRIDE", "SETUID", "SETGID", "KILL"}, []string(hc.CapAdd))
	assert.Equal(t, "none", string(hc.NetworkMode), "сеть должна быть отключена")
	assert.True(t, hc.ReadonlyRootfs, "корень только на чтение")
	assert.Equal(t, cfg.MemoryLimit, hc.Memory)
	assert.Equal(t, cfg.MemoryLimit, hc.MemorySwap, "swap запрещён: MemorySwap == Memory")
	require.NotNil(t, hc.PidsLimit)
	assert.Equal(t, int64(100), *hc.PidsLimit, "лимит pid против fork-bomb")
	require.NotNil(t, hc.OomKillDisable)
	assert.False(t, *hc.OomKillDisable, "oom-killer должен быть включён")
	assert.Equal(t, int64(100000), hc.CPUPeriod)
	assert.Equal(t, cfg.CPUQuota, hc.CPUQuota)
	assert.False(t, hc.AutoRemove, "автоудаление off - сперва логи, потом cleanup")

	// tmpfs /tmp писабельный, но без setuid и с лимитом
	tmp, ok := hc.Tmpfs["/tmp"]
	require.True(t, ok, "должен быть tmpfs на /tmp")
	assert.Contains(t, tmp, "nosuid")
	assert.Contains(t, tmp, "size=64m")

	// копии программ: исполняемые, без setuid, список файлов ботам закрыт
	progs, ok := hc.Tmpfs["/programs"]
	require.True(t, ok, "должен быть tmpfs на /programs")
	for _, opt := range []string{"exec", "nosuid", "nodev", "mode=0711"} {
		assert.Contains(t, strings.Split(progs, ","), opt)
	}

	// монтируются только переданные файлы программ, на чтение
	assert.Equal(t, binds, hc.Binds)

	// без профилей в конфиге - только no-new-privileges
	assert.Equal(t, []string{"no-new-privileges:true"}, hc.SecurityOpt)

	// ulimits
	limits := map[string][2]int64{}
	for _, u := range hc.Ulimits {
		limits[u.Name] = [2]int64{u.Soft, u.Hard}
	}
	assert.Equal(t, [2]int64{1024, 1024}, limits["nofile"])
	assert.Equal(t, [2]int64{64, 64}, limits["nproc"])
	assert.Equal(t, [2]int64{0, 0}, limits["core"])
	assert.Equal(t, [2]int64{10485760, 10485760}, limits["fsize"])
}

// seccomp/apparmor добавляются в SecurityOpt только когда профиль задан env-ом.
// в seccomp= уходит содержимое профиля: демон разбирает значение как JSON
func TestBuildMatchHostConfig_ProfilesOptional(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seccomp.json")
	require.NoError(t, os.WriteFile(path, []byte("{\n  \"defaultAction\": \"SCMP_ACT_ERRNO\"\n}\n"), 0o600))
	profile, err := loadSeccompProfile(path)
	require.NoError(t, err)

	cfg := config.ExecutorConfig{
		SeccompProfile:  profile,
		AppArmorProfile: "tjudge-profile",
	}
	hc := buildMatchHostConfig(cfg, nil)
	assert.Equal(t, []string{
		"no-new-privileges:true",
		`seccomp={"defaultAction":"SCMP_ACT_ERRNO"}`,
		"apparmor=tjudge-profile",
	}, hc.SecurityOpt)
}

// нечитаемый или битый профиль - отказ на старте, а не infra-ошибка каждого матча
func TestLoadSeccompProfile_Invalid(t *testing.T) {
	_, err := loadSeccompProfile(filepath.Join(t.TempDir(), "missing.json"))
	assert.Error(t, err)

	path := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0o600))
	_, err = loadSeccompProfile(path)
	assert.Error(t, err)
}

// в матч монтируются только файлы двух программ (и классы java), а не весь
// каталог программ со всеми командами, и не туда, откуда их запускают боты
func TestProgramMounts_OnlyMatchPrograms(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"t1_g_p1", "t1_g_p1.c", "t2_g_p2", "t2_g_p2.java", "t3_g_p3.py"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600))
	}
	require.NoError(t, os.Mkdir(filepath.Join(dir, "t2_g_p2_classes"), 0o750))

	log, _ := logger.New("error", "json")
	e := &Executor{programsPath: dir, hostProgramsPath: "/host/programs", containerPath: "/programs", log: log}

	paths, binds, err := e.programMounts(filepath.Join(dir, "t1_g_p1"), filepath.Join(dir, "t2_g_p2"))
	require.NoError(t, err)
	assert.Equal(t, []string{"/programs/t1_g_p1", "/programs/t2_g_p2"}, paths)
	assert.Equal(t, []string{
		"/host/programs/t1_g_p1:/mnt/programs/t1_g_p1:ro",
		"/host/programs/t2_g_p2:/mnt/programs/t2_g_p2:ro",
		"/host/programs/t2_g_p2_classes:/mnt/programs/t2_g_p2_classes:ro",
	}, binds)

	// одна и та же программа с обеих сторон - один bind, иначе докер откажет
	_, binds, err = e.programMounts(filepath.Join(dir, "t3_g_p3.py"), filepath.Join(dir, "t3_g_p3.py"))
	require.NoError(t, err)
	assert.Equal(t, []string{"/host/programs/t3_g_p3.py:/mnt/programs/t3_g_p3.py:ro"}, binds)

	// сам каталог программ и отсутствующий файл не монтируются
	_, _, err = e.programMounts(dir, filepath.Join(dir, "t1_g_p1"))
	assert.Error(t, err)
	_, _, err = e.programMounts(filepath.Join(dir, "gone"), filepath.Join(dir, "t1_g_p1"))
	assert.Error(t, err)
}

// builder жирнее матча (память/pids/tmpfs), но так же изолирован
func TestBuildBuilderHostConfig_SandboxFlags(t *testing.T) {
	hc := buildBuilderHostConfig("/host/build/xyz")

	assert.Equal(t, []string{"ALL"}, []string(hc.CapDrop))
	assert.Equal(t, "none", string(hc.NetworkMode))
	assert.True(t, hc.ReadonlyRootfs)
	assert.Equal(t, []string{"no-new-privileges:true"}, hc.SecurityOpt)
	assert.Equal(t, int64(1<<30), hc.Memory)
	assert.Equal(t, int64(1<<30), hc.MemorySwap)
	require.NotNil(t, hc.PidsLimit)
	assert.Equal(t, int64(256), *hc.PidsLimit)
	assert.Equal(t, int64(1e9), hc.NanoCPUs, "сборка ограничена одним ядром")
	assert.False(t, hc.AutoRemove)

	limits := map[string][2]int64{}
	for _, u := range hc.Ulimits {
		limits[u.Name] = [2]int64{u.Soft, u.Hard}
	}
	assert.Equal(t, [2]int64{64 << 20, 64 << 20}, limits["fsize"])
	assert.Equal(t, [2]int64{4096, 4096}, limits["nofile"])

	tmp := hc.Tmpfs["/tmp"]
	assert.Contains(t, tmp, "nosuid")
	assert.Contains(t, tmp, "size=512m")

	// монтируется только каталог сборки одной программы, rw
	require.Len(t, hc.Binds, 1)
	assert.Equal(t, "/host/build/xyz:/build:rw", hc.Binds[0])
}
