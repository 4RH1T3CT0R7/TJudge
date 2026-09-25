package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/cache"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const compileQueueKey = "queue:compile"

// пока задача программы лежит в очереди, повторный Enqueue её не дублирует
// (stuck-recovery каждой реплики, админский requeue). ttl - страховка на
// случай, если ключ не сняли при извлечении
const (
	compileDedupPrefix = "queue:compile:dedup:"
	compileDedupTTL    = 10 * time.Minute
)

type CompileTask struct {
	ProgramID uuid.UUID `json:"program_id"`
}

// CompileQueue - простая очередь компиляции на редисе (lpush/brpop).
// потеря задачи не страшна: compile-worker периодически подбирает программы
// зависшие в compiling (GetStuckCompiling), так что подстрахованы
type CompileQueue struct {
	cache *cache.Cache
	log   *logger.Logger
}

func NewCompileQueue(c *cache.Cache, log *logger.Logger) *CompileQueue {
	return &CompileQueue{cache: c, log: log}
}

func (q *CompileQueue) Enqueue(ctx context.Context, programID uuid.UUID) error {
	payload, err := json.Marshal(CompileTask{ProgramID: programID})
	if err != nil {
		return fmt.Errorf("failed to marshal compile task: %w", err)
	}

	dedupKey := compileDedupPrefix + programID.String()
	isNew, err := q.cache.SetNX(ctx, dedupKey, "1", compileDedupTTL)
	if err != nil {
		return fmt.Errorf("failed to check compile dedup: %w", err)
	}
	if !isNew {
		return nil // задача уже в очереди
	}

	if err := q.cache.LPush(ctx, compileQueueKey, string(payload)); err != nil {
		// откат, иначе следующие Enqueue молча пропускались бы до ttl
		_ = q.cache.Del(ctx, dedupKey)
		return fmt.Errorf("failed to enqueue compile task: %w", err)
	}

	q.log.Info("Compile task enqueued", zap.String("program_id", programID.String()))
	return nil
}

// Dequeue блокирующе забирает задачу из очереди (BRPOP с таймаутом).
// Возвращает nil без ошибки, если за timeout задач не появилось.
func (q *CompileQueue) Dequeue(ctx context.Context, timeout time.Duration) (*CompileTask, error) {
	result, err := q.cache.BRPop(ctx, timeout, compileQueueKey)
	if err != nil {
		return nil, fmt.Errorf("failed to dequeue compile task: %w", err)
	}
	if len(result) < 2 {
		return nil, nil // таймаут - очередь пуста
	}

	var task CompileTask
	if err := json.Unmarshal([]byte(result[1]), &task); err != nil {
		q.log.Error("Failed to unmarshal compile task, dropping",
			zap.Error(err),
			zap.String("payload", result[1]),
		)
		return nil, nil // повреждённая задача: stuck-recovery вернёт программу в очередь
	}

	if err := q.cache.Del(ctx, compileDedupPrefix+task.ProgramID.String()); err != nil {
		q.log.Warn("Failed to remove compile dedup key", zap.Error(err),
			zap.String("program_id", task.ProgramID.String()))
	}

	return &task, nil
}

// Size возвращает текущую длину очереди компиляции.
func (q *CompileQueue) Size(ctx context.Context) (int64, error) {
	return q.cache.LLen(ctx, compileQueueKey)
}
