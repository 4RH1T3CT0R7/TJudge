package events

import (
	"context"
	"reflect"
	"sync"

	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"go.uber.org/zap"
)

// Handler обрабатывает доменные события.
type Handler interface {
	Handle(ctx context.Context, event any) error
}

// Bus публикует доменные события и рассылает их подписанным обработчикам.
type Bus interface {
	Publish(ctx context.Context, event any)
	Subscribe(handler Handler, eventTypes ...any)
}

// subscription связывает обработчик с набором интересующих его типов событий.
type subscription struct {
	handler    Handler
	eventTypes map[reflect.Type]struct{}
}

// SyncBus - синхронная внутрипроцессная шина событий.
// Publish последовательно вызывает все подходящие обработчики.
// Ошибки обработчиков логируются, но никогда не пробрасываются отправителю.
type SyncBus struct {
	mu            sync.RWMutex
	subscriptions []subscription
	log           *logger.Logger
}

// NewSyncBus создаёт новую синхронную шину событий.
func NewSyncBus(log *logger.Logger) *SyncBus {
	return &SyncBus{log: log}
}

// Subscribe регистрирует обработчик для указанных типов событий.
// eventTypes должны быть нулевыми экземплярами структур событий (например, TournamentCreated{}).
func (b *SyncBus) Subscribe(handler Handler, eventTypes ...any) {
	types := make(map[reflect.Type]struct{}, len(eventTypes))
	for _, et := range eventTypes {
		types[reflect.TypeOf(et)] = struct{}{}
	}

	b.mu.Lock()
	b.subscriptions = append(b.subscriptions, subscription{
		handler:    handler,
		eventTypes: types,
	})
	b.mu.Unlock()
}

// Publish рассылает событие всем обработчикам, подписанным на его тип.
// Ошибки обработчиков логируются на уровне ERROR, но не останавливают других обработчиков и не возвращаются вызывающему.
func (b *SyncBus) Publish(ctx context.Context, event any) {
	eventType := reflect.TypeOf(event)

	b.mu.RLock()
	subs := make([]subscription, len(b.subscriptions))
	copy(subs, b.subscriptions)
	b.mu.RUnlock()

	for _, sub := range subs {
		if _, ok := sub.eventTypes[eventType]; !ok {
			continue
		}
		if err := sub.handler.Handle(ctx, event); err != nil {
			b.log.Error("Event handler error",
				zap.String("event_type", eventType.Name()),
				zap.Error(err),
			)
		}
	}
}

// NoopBus - заглушка шины событий для тестов.
type NoopBus struct{}

func (NoopBus) Publish(context.Context, any) {}
func (NoopBus) Subscribe(Handler, ...any)    {}
