package websocket

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNewNoopBroadcaster(t *testing.T) {
	b := NewNoopBroadcaster()
	assert.NotNil(t, b)
}

func TestNoopBroadcaster_Broadcast_DoesNotPanic(t *testing.T) {
	b := NewNoopBroadcaster()

	assert.NotPanics(t, func() {
		b.Broadcast(uuid.New(), "leaderboard_update", map[string]string{"key": "value"})
	})
}

func TestNoopBroadcaster_Broadcast_NilPayload(t *testing.T) {
	b := NewNoopBroadcaster()

	assert.NotPanics(t, func() {
		b.Broadcast(uuid.Nil, "", nil)
	})
}

func TestNoopBroadcaster_ImplementsBroadcaster(t *testing.T) {
	var _ Broadcaster = (*NoopBroadcaster)(nil)
}
