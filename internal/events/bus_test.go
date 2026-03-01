package events

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testEvent is a simple event for testing.
type testEvent struct {
	Value string
}

// otherEvent is an unrelated event type.
type otherEvent struct{}

// capturingHandler records all events it receives.
type capturingHandler struct {
	events []any
}

func (h *capturingHandler) Handle(_ context.Context, event any) error {
	h.events = append(h.events, event)
	return nil
}

// failingHandler always returns an error.
type failingHandler struct {
	called int32
}

func (h *failingHandler) Handle(_ context.Context, _ any) error {
	atomic.AddInt32(&h.called, 1)
	return errors.New("handler failed")
}

func newTestLogger(t *testing.T) *logger.Logger {
	t.Helper()
	log, err := logger.New("error", "json")
	require.NoError(t, err)
	return log
}

func TestSyncBus_PublishDispatchesToSubscribedHandlers(t *testing.T) {
	bus := NewSyncBus(newTestLogger(t))

	h := &capturingHandler{}
	bus.Subscribe(h, testEvent{})

	bus.Publish(context.Background(), testEvent{Value: "hello"})

	require.Len(t, h.events, 1)
	assert.Equal(t, testEvent{Value: "hello"}, h.events[0])
}

func TestSyncBus_UnsubscribedHandlerNotCalled(t *testing.T) {
	bus := NewSyncBus(newTestLogger(t))

	h := &capturingHandler{}
	bus.Subscribe(h, testEvent{}) // subscribed to testEvent only

	bus.Publish(context.Background(), otherEvent{})

	assert.Empty(t, h.events)
}

func TestSyncBus_MultipleHandlersSameEvent(t *testing.T) {
	bus := NewSyncBus(newTestLogger(t))

	h1 := &capturingHandler{}
	h2 := &capturingHandler{}
	bus.Subscribe(h1, testEvent{})
	bus.Subscribe(h2, testEvent{})

	bus.Publish(context.Background(), testEvent{Value: "both"})

	assert.Len(t, h1.events, 1)
	assert.Len(t, h2.events, 1)
}

func TestSyncBus_HandlerErrorLoggedNotPropagated(t *testing.T) {
	bus := NewSyncBus(newTestLogger(t))

	failing := &failingHandler{}
	after := &capturingHandler{}

	bus.Subscribe(failing, testEvent{})
	bus.Subscribe(after, testEvent{})

	// Should not panic; after-handler should still be called
	bus.Publish(context.Background(), testEvent{Value: "ok"})

	assert.Equal(t, int32(1), atomic.LoadInt32(&failing.called))
	assert.Len(t, after.events, 1)
}

func TestSyncBus_HandlerSubscribedToMultipleEvents(t *testing.T) {
	bus := NewSyncBus(newTestLogger(t))

	h := &capturingHandler{}
	bus.Subscribe(h, testEvent{}, otherEvent{})

	bus.Publish(context.Background(), testEvent{Value: "a"})
	bus.Publish(context.Background(), otherEvent{})

	assert.Len(t, h.events, 2)
}

func TestSyncBus_NoSubscribers(t *testing.T) {
	bus := NewSyncBus(newTestLogger(t))

	// Should not panic when no one is subscribed
	bus.Publish(context.Background(), testEvent{Value: "nobody listening"})
}

func TestSyncBus_RealEventTypes(t *testing.T) {
	bus := NewSyncBus(newTestLogger(t))

	h := &capturingHandler{}
	bus.Subscribe(h, ParticipantJoined{}, MatchResultProcessed{})

	id := uuid.New()
	bus.Publish(context.Background(), ParticipantJoined{
		TournamentID:  id,
		ProgramID:     uuid.New(),
		InitialRating: 1500,
	})
	bus.Publish(context.Background(), MatchResultProcessed{
		TournamentID: id,
		MatchID:      uuid.New(),
		Winner:       1,
	})

	require.Len(t, h.events, 2)
	assert.IsType(t, ParticipantJoined{}, h.events[0])
	assert.IsType(t, MatchResultProcessed{}, h.events[1])
}

func TestNoopBus_DoesNothing(t *testing.T) {
	bus := NoopBus{}

	h := &capturingHandler{}
	bus.Subscribe(h, testEvent{})
	bus.Publish(context.Background(), testEvent{Value: "ignored"})

	assert.Empty(t, h.events)
}
