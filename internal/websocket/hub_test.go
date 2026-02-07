package websocket

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestHub(t *testing.T) *Hub {
	t.Helper()
	log, _ := logger.New("error", "json")
	return NewHub(log)
}

// newTestClient creates a synthetic client for testing (no real WebSocket connection)
func newTestClient(hub *Hub, tournamentID, userID uuid.UUID) *Client {
	log, _ := logger.New("error", "json")
	return &Client{
		hub:          hub,
		conn:         nil, // no real connection
		send:         make(chan []byte, 256),
		tournamentID: tournamentID,
		userID:       userID,
		log:          log,
	}
}

func startHub(t *testing.T, hub *Hub) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	// Give the hub goroutine time to start
	time.Sleep(10 * time.Millisecond)
	return cancel
}

func TestHub_RegisterSingleClient(t *testing.T) {
	hub := newTestHub(t)
	cancel := startHub(t, hub)
	defer cancel()

	tournamentID := uuid.New()
	client := newTestClient(hub, tournamentID, uuid.New())

	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	stats := hub.GetStats()
	assert.Equal(t, 1, stats["tournaments"])
	assert.Equal(t, 1, stats["total_clients"])
}

func TestHub_RegisterMultipleClientsSameTournament(t *testing.T) {
	hub := newTestHub(t)
	cancel := startHub(t, hub)
	defer cancel()

	tournamentID := uuid.New()
	c1 := newTestClient(hub, tournamentID, uuid.New())
	c2 := newTestClient(hub, tournamentID, uuid.New())

	hub.register <- c1
	hub.register <- c2
	time.Sleep(10 * time.Millisecond)

	stats := hub.GetStats()
	assert.Equal(t, 1, stats["tournaments"])
	assert.Equal(t, 2, stats["total_clients"])
}

func TestHub_RegisterDifferentTournaments(t *testing.T) {
	hub := newTestHub(t)
	cancel := startHub(t, hub)
	defer cancel()

	c1 := newTestClient(hub, uuid.New(), uuid.New())
	c2 := newTestClient(hub, uuid.New(), uuid.New())

	hub.register <- c1
	hub.register <- c2
	time.Sleep(10 * time.Millisecond)

	stats := hub.GetStats()
	assert.Equal(t, 2, stats["tournaments"])
	assert.Equal(t, 2, stats["total_clients"])
}

func TestHub_UnregisterOneOfTwo(t *testing.T) {
	hub := newTestHub(t)
	cancel := startHub(t, hub)
	defer cancel()

	tournamentID := uuid.New()
	c1 := newTestClient(hub, tournamentID, uuid.New())
	c2 := newTestClient(hub, tournamentID, uuid.New())

	hub.register <- c1
	hub.register <- c2
	time.Sleep(10 * time.Millisecond)

	hub.unregister <- c1
	time.Sleep(10 * time.Millisecond)

	stats := hub.GetStats()
	assert.Equal(t, 1, stats["tournaments"])
	assert.Equal(t, 1, stats["total_clients"])
}

func TestHub_UnregisterLastInTournament(t *testing.T) {
	hub := newTestHub(t)
	cancel := startHub(t, hub)
	defer cancel()

	tournamentID := uuid.New()
	client := newTestClient(hub, tournamentID, uuid.New())

	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	hub.unregister <- client
	time.Sleep(10 * time.Millisecond)

	stats := hub.GetStats()
	assert.Equal(t, 0, stats["tournaments"])
	assert.Equal(t, 0, stats["total_clients"])
}

func TestHub_UnregisterUnregistered(t *testing.T) {
	hub := newTestHub(t)
	cancel := startHub(t, hub)
	defer cancel()

	client := newTestClient(hub, uuid.New(), uuid.New())

	// Unregistering a client that was never registered should not panic
	hub.unregister <- client
	time.Sleep(10 * time.Millisecond)

	stats := hub.GetStats()
	assert.Equal(t, 0, stats["tournaments"])
}

func TestHub_BroadcastToRegisteredClients(t *testing.T) {
	hub := newTestHub(t)
	cancel := startHub(t, hub)
	defer cancel()

	tournamentID := uuid.New()
	c1 := newTestClient(hub, tournamentID, uuid.New())
	c2 := newTestClient(hub, tournamentID, uuid.New())

	hub.register <- c1
	hub.register <- c2
	time.Sleep(10 * time.Millisecond)

	hub.broadcast <- &Message{
		TournamentID: tournamentID,
		Type:         MessageTypeMatchUpdate,
		Payload:      map[string]string{"status": "completed"},
	}
	time.Sleep(10 * time.Millisecond)

	// Both clients should receive the message
	require.Len(t, c1.send, 1)
	require.Len(t, c2.send, 1)

	data := <-c1.send
	var msg Message
	require.NoError(t, json.Unmarshal(data, &msg))
	assert.Equal(t, MessageTypeMatchUpdate, msg.Type)
}

func TestHub_BroadcastNoClients(t *testing.T) {
	hub := newTestHub(t)
	cancel := startHub(t, hub)
	defer cancel()

	// Broadcasting to a tournament with no clients should not panic
	hub.broadcast <- &Message{
		TournamentID: uuid.New(),
		Type:         MessageTypeLeaderboardUpdate,
		Payload:      nil,
	}
	time.Sleep(10 * time.Millisecond)

	stats := hub.GetStats()
	assert.Equal(t, 0, stats["tournaments"])
}

func TestHub_GetStats_Empty(t *testing.T) {
	hub := newTestHub(t)

	stats := hub.GetStats()
	assert.Equal(t, 0, stats["tournaments"])
	assert.Equal(t, 0, stats["total_clients"])
}

func TestHub_Broadcast_PublicMethod(t *testing.T) {
	hub := newTestHub(t)
	cancel := startHub(t, hub)
	defer cancel()

	tournamentID := uuid.New()
	client := newTestClient(hub, tournamentID, uuid.New())

	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	hub.Broadcast(tournamentID, "test_type", map[string]string{"key": "value"})
	time.Sleep(10 * time.Millisecond)

	require.Len(t, client.send, 1)
	data := <-client.send
	var msg Message
	require.NoError(t, json.Unmarshal(data, &msg))
	assert.Equal(t, MessageType("test_type"), msg.Type)
}

func TestHub_Shutdown(t *testing.T) {
	hub := newTestHub(t)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		hub.Run(ctx)
		close(done)
	}()
	time.Sleep(10 * time.Millisecond)

	tournamentID := uuid.New()
	client := newTestClient(hub, tournamentID, uuid.New())
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	cancel()

	select {
	case <-done:
		// Hub shut down properly
	case <-time.After(2 * time.Second):
		t.Fatal("Hub.Run did not return after context cancellation")
	}

	stats := hub.GetStats()
	assert.Equal(t, 0, stats["tournaments"])
	assert.Equal(t, 0, stats["total_clients"])
}
