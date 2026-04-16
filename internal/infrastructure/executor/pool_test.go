package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Smoke-тест для NoReusePool — проверяет что интерфейс реализован корректно
// и не создаёт состояние между вызовами (P2.5).
func TestNoReusePool_AcquireReleaseRoundTrip(t *testing.T) {
	p := NewNoReusePool()
	ctx := context.Background()

	c1, err := p.Acquire(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, c1)

	c2, err := p.Acquire(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, c2)
	assert.NotSame(t, c1, c2, "каждый Acquire возвращает новый объект")

	p.Release(c1)
	p.Drop(c2)

	// Stats — zero для no-reuse pool.
	stats := p.Stats()
	assert.Zero(t, stats.Free)
	assert.Zero(t, stats.Busy)
	assert.Zero(t, stats.TotalCreated)
}

func TestNoReusePool_AcquireRespectsContextCancel(t *testing.T) {
	p := NewNoReusePool()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// No-reuse pool не блокирует — всегда возвращает сразу.
	c, err := p.Acquire(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, c)
}
