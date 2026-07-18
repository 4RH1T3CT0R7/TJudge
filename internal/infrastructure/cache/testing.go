package cache

import (
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/redis/go-redis/v9"
)

// NewFromClient creates a Cache from an existing redis.Client.
// Intended for use in tests with miniredis.
func NewFromClient(client *redis.Client) *Cache {
	log, _ := logger.New("error", "json")
	m := metrics.New()
	return &Cache{
		client:  client,
		log:     log,
		metrics: m,
	}
}
