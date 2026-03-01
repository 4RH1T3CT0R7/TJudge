package events

import (
	"context"
	"reflect"
	"sync"

	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"go.uber.org/zap"
)

// Handler processes domain events.
type Handler interface {
	Handle(ctx context.Context, event any) error
}

// Bus publishes domain events and dispatches them to subscribed handlers.
type Bus interface {
	Publish(ctx context.Context, event any)
	Subscribe(handler Handler, eventTypes ...any)
}

// subscription binds a handler to a set of event types it cares about.
type subscription struct {
	handler    Handler
	eventTypes map[reflect.Type]struct{}
}

// SyncBus is a synchronous, in-process event bus.
// Publish calls all matching handlers sequentially.
// Handler errors are logged but never propagated to the publisher.
type SyncBus struct {
	mu            sync.RWMutex
	subscriptions []subscription
	log           *logger.Logger
}

// NewSyncBus creates a new synchronous event bus.
func NewSyncBus(log *logger.Logger) *SyncBus {
	return &SyncBus{log: log}
}

// Subscribe registers a handler for the given event types.
// eventTypes should be zero-value instances of event structs (e.g., TournamentCreated{}).
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

// Publish dispatches an event to all handlers subscribed to its type.
// Handler errors are logged at ERROR level but do not stop other handlers or propagate to the caller.
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

// NoopBus is a no-op event bus for tests.
type NoopBus struct{}

func (NoopBus) Publish(context.Context, any) {}
func (NoopBus) Subscribe(Handler, ...any)    {}
