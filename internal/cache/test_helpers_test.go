package cache

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// поднимаем Cache поверх miniredis, сервер сам закрывается по концу теста
func setupTestCache(t *testing.T) *Cache {
	t.Helper()

	mr := miniredis.RunT(t)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	log, _ := logger.New("error", "json")
	m := metrics.New()

	return &Cache{
		client:  client,
		log:     log,
		metrics: m,
	}
}

// то же, но отдаём и miniredis чтобы крутить время (FastForward)
func setupTestCacheWithMR(t *testing.T) (*Cache, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	log, _ := logger.New("error", "json")
	m := metrics.New()

	return &Cache{
		client:  client,
		log:     log,
		metrics: m,
	}, mr
}
