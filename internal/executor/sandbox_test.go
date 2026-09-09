package executor

import (
	"testing"

	"github.com/bmstu-itstech/tjudge/internal/config"
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
	hc := buildMatchHostConfig(cfg, "/host/programs", "/programs")

	assert.Equal(t, []string{"ALL"}, []string(hc.CapDrop), "должны сниматься все capabilities")
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

	// программы монтируются только на чтение
	require.Len(t, hc.Binds, 1)
	assert.Equal(t, "/host/programs:/programs:ro", hc.Binds[0])

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

// seccomp/apparmor добавляются в SecurityOpt только когда профиль задан env-ом
func TestBuildMatchHostConfig_ProfilesOptional(t *testing.T) {
	cfg := config.ExecutorConfig{
		SeccompProfile:  "/sec/seccomp.json",
		AppArmorProfile: "tjudge-profile",
	}
	hc := buildMatchHostConfig(cfg, "/p", "/programs")
	assert.Equal(t, []string{
		"no-new-privileges:true",
		"seccomp=/sec/seccomp.json",
		"apparmor=tjudge-profile",
	}, hc.SecurityOpt)
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
	assert.False(t, hc.AutoRemove)

	tmp := hc.Tmpfs["/tmp"]
	assert.Contains(t, tmp, "nosuid")
	assert.Contains(t, tmp, "size=512m")

	// монтируется только каталог сборки одной программы, rw
	require.Len(t, hc.Binds, 1)
	assert.Equal(t, "/host/build/xyz:/build:rw", hc.Binds[0])
}
