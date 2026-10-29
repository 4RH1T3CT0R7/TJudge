package worker

// MatchSlots - сколько матчей разом помещается в cpus ядер, если матч-контейнер
// получает квоту cpuQuota мкс на 100 мс (EXECUTOR_CPU_QUOTA, 0 - без квоты,
// считается ядром). больше воркеров только хуже: боты не укладываются в лимит
// хода и проигрывают по таймауту, а демон docker не успевает создавать и
// удалять контейнеры. на хосте с 8 ГБ 200 матчей разом давали таймауты удаления
func MatchSlots(cpuQuota int64, cpus int) int {
	perMatch := 1.0
	if cpuQuota > 0 {
		perMatch = float64(cpuQuota) / 100000
	}
	return max(1, int(float64(cpus)/perMatch))
}
