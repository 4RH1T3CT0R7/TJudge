package cache

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// setupTestCache creates a Cache backed by miniredis for unit tests.
// The miniredis server is automatically closed when the test ends.
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

// setupTestCacheWithMR creates a Cache and returns the underlying miniredis
// instance so that tests can manipulate time (e.g. FastForward).
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
