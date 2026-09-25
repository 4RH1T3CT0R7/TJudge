package queue

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bmstu-itstech/tjudge/internal/cache"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestCompileQueue(t *testing.T) (*CompileQueue, *cache.Cache) {
	t.Helper()
	mr := miniredis.RunT(t)
	c := cache.NewFromClient(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	t.Cleanup(func() { _ = c.Close() })
	return NewCompileQueue(c, testLogger()), c
}

// LPUSH + BRPOP: задачи выходят в порядке постановки
func TestCompileQueue_EnqueueDequeueFIFO(t *testing.T) {
	q, _ := setupTestCompileQueue(t)
	ctx := context.Background()

	first, second := uuid.New(), uuid.New()
	require.NoError(t, q.Enqueue(ctx, first))
	require.NoError(t, q.Enqueue(ctx, second))

	size, err := q.Size(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), size)

	for _, want := range []uuid.UUID{first, second} {
		task, err := q.Dequeue(ctx, time.Second)
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.Equal(t, want, task.ProgramID)
	}

	size, err = q.Size(ctx)
	require.NoError(t, err)
	assert.Zero(t, size)
}

// битая задача выбрасывается без ошибки: программу вернёт stuck-recovery,
// а воркер компиляции не должен падать на мусоре
func TestCompileQueue_DequeueDropsCorruptTask(t *testing.T) {
	q, c := setupTestCompileQueue(t)
	ctx := context.Background()

	require.NoError(t, c.LPush(ctx, compileQueueKey, "not-json"))

	task, err := q.Dequeue(ctx, time.Second)
	require.NoError(t, err)
	assert.Nil(t, task)

	size, err := q.Size(ctx)
	require.NoError(t, err)
	assert.Zero(t, size)
}
