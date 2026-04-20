package executor

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Интерфейс пула контейнеров для executor'а.
// Полноценная реализация с reuse пока отложена. Текущая активная реализация -
// NoReusePool: один контейнер на матч.

// PooledContainer - описание контейнера, захваченного из пула.
type PooledContainer struct {
	ID      string
	Created time.Time
	UsedBy  uuid.UUID
}

// PoolStats - снимок состояния пула для метрик.
type PoolStats struct {
	Free         int
	Busy         int
	TotalCreated int
}

// ContainerPool - интерфейс захвата/возврата контейнеров.
// Acquire блокируется до тех пор, пока не появится свободный или ctx.Done.
// Release возвращает контейнер в пул; Drop помечает "нечистым" и удаляет.
type ContainerPool interface {
	Acquire(ctx context.Context) (*PooledContainer, error)
	Release(c *PooledContainer)
	Drop(c *PooledContainer)
	Stats() PoolStats
}

// NoReusePool - реализация по умолчанию: каждый Acquire создаёт свежий
// контейнер, Release/Drop - kill. Отражает текущее поведение executor'а.
//
// Это заглушка, которая позволяет переходить на ContainerPool-интерфейс
// без изменения runtime-поведения. Полноценная warm-reuse реализация
// будет вторым шагом.
type NoReusePool struct{}

// NewNoReusePool возвращает pool без reuse.
func NewNoReusePool() *NoReusePool { return &NoReusePool{} }

// Acquire всегда возвращает "виртуальный" контейнер; реальное создание делает
// caller через существующий путь executor.Run. Это удобно для постепенной миграции.
func (*NoReusePool) Acquire(_ context.Context) (*PooledContainer, error) {
	return &PooledContainer{Created: time.Now()}, nil
}

// Release - no-op, контейнер будет убит caller'ом после матча.
func (*NoReusePool) Release(_ *PooledContainer) {}

// Drop - no-op, аналогично Release в no-reuse режиме.
func (*NoReusePool) Drop(_ *PooledContainer) {}

// Stats возвращает нулевую статистику: pool не хранит состояние.
func (*NoReusePool) Stats() PoolStats { return PoolStats{} }

// Compile-time проверка интерфейса.
var _ ContainerPool = (*NoReusePool)(nil)
