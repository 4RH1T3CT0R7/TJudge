package cache

import (
	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// NewFromClient - Cache поверх готового redis.Client, для тестов на miniredis
func NewFromClient(client *redis.Client) *Cache {
	log, _ := logger.New("error", "json")
	m := metrics.New()
	return &Cache{
		client:  client,
		log:     log,
		metrics: m,
	}
}
