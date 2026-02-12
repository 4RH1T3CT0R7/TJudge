package websocket

import (
	"context"
	"encoding/json"
	"sync"
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
	time.Sleep(5 * time.Millisecond)
	return cancel
}

func waitForStats(t *testing.T, hub *Hub, key string, expected int) {
	t.Helper()
	require.Eventually(t, func() bool {
		return hub.GetStats()[key] == expected
	}, time.Second, time.Millisecond)
}

func TestHub_RegisterSingleClient(t *testing.T) {
	hub := newTestHub(t)
	cancel := startHub(t, hub)
	defer cancel()

	tournamentID := uuid.New()
	client := newTestClient(hub, tournamentID, uuid.New())

	hub.register <- client
	waitForStats(t, hub, "total_clients", 1)

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
	waitForStats(t, hub, "total_clients", 2)

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
	waitForStats(t, hub, "total_clients", 2)

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
	waitForStats(t, hub, "total_clients", 2)

	hub.unregister <- c1
	waitForStats(t, hub, "total_clients", 1)

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
	waitForStats(t, hub, "total_clients", 1)

	hub.unregister <- client
	waitForStats(t, hub, "total_clients", 0)

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

	// Give time for processing, then verify stats are still zero
	time.Sleep(5 * time.Millisecond)
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
	waitForStats(t, hub, "total_clients", 2)

	hub.broadcast <- &Message{
		TournamentID: tournamentID,
		Type:         MessageTypeMatchUpdate,
		Payload:      map[string]string{"status": "completed"},
	}

	// Wait for both clients to receive the message
	require.Eventually(t, func() bool {
		return len(c1.send) == 1 && len(c2.send) == 1
	}, time.Second, time.Millisecond)

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

	// Give time for processing
	time.Sleep(5 * time.Millisecond)
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
	waitForStats(t, hub, "total_clients", 1)

	hub.Broadcast(tournamentID, "test_type", map[string]string{"key": "value"})

	require.Eventually(t, func() bool {
		return len(client.send) == 1
	}, time.Second, time.Millisecond)

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
	time.Sleep(5 * time.Millisecond)

	tournamentID := uuid.New()
	client := newTestClient(hub, tournamentID, uuid.New())
	hub.register <- client
	waitForStats(t, hub, "total_clients", 1)

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

func TestHub_BroadcastToSlowClient_DisconnectsClient(t *testing.T) {
	hub := newTestHub(t)
	cancel := startHub(t, hub)
	defer cancel()

	tournamentID := uuid.New()

	// Create a slow client with a send buffer of size 1
	log, _ := logger.New("error", "json")
	slowClient := &Client{
		hub:          hub,
		conn:         nil,
		send:         make(chan []byte, 1),
		tournamentID: tournamentID,
		userID:       uuid.New(),
		log:          log,
	}

	hub.register <- slowClient
	waitForStats(t, hub, "total_clients", 1)

	// Fill the slow client's send buffer
	slowClient.send <- []byte("filler")

	// Broadcast a message; the slow client's buffer is full so it should be disconnected
	hub.broadcast <- &Message{
		TournamentID: tournamentID,
		Type:         MessageTypeMatchUpdate,
		Payload:      map[string]string{"status": "completed"},
	}

	// The slow client should be removed from the hub
	waitForStats(t, hub, "total_clients", 0)

	stats := hub.GetStats()
	assert.Equal(t, 0, stats["total_clients"])
}

func TestHub_DoubleUnregister_NoPanic(t *testing.T) {
	hub := newTestHub(t)
	cancel := startHub(t, hub)
	defer cancel()

	tournamentID := uuid.New()
	client := newTestClient(hub, tournamentID, uuid.New())

	hub.register <- client
	waitForStats(t, hub, "total_clients", 1)

	// First unregister
	hub.unregister <- client
	waitForStats(t, hub, "total_clients", 0)

	// Second unregister should not panic
	hub.unregister <- client

	// Give time for the second unregister to be processed
	time.Sleep(10 * time.Millisecond)

	stats := hub.GetStats()
	assert.Equal(t, 0, stats["tournaments"])
	assert.Equal(t, 0, stats["total_clients"])
}

func TestHub_BroadcastOtherTournament_NotReceived(t *testing.T) {
	hub := newTestHub(t)
	cancel := startHub(t, hub)
	defer cancel()

	tournamentA := uuid.New()
	tournamentB := uuid.New()

	clientA := newTestClient(hub, tournamentA, uuid.New())

	hub.register <- clientA
	waitForStats(t, hub, "total_clients", 1)

	// Broadcast to tournament B (where clientA is NOT registered)
	hub.broadcast <- &Message{
		TournamentID: tournamentB,
		Type:         MessageTypeLeaderboardUpdate,
		Payload:      map[string]string{"rank": "1"},
	}

	// Give time for the broadcast to be processed
	time.Sleep(10 * time.Millisecond)

	// Client A should NOT have received anything
	select {
	case <-clientA.send:
		t.Fatal("client should not receive a message broadcast to a different tournament")
	default:
		// OK - no message received
	}
}

func TestHub_ConcurrentRegisterBroadcast(t *testing.T) {
	hub := newTestHub(t)
	cancel := startHub(t, hub)
	defer cancel()

	tournamentID := uuid.New()
	const numGoroutines = 20

	var wg sync.WaitGroup

	// Concurrently register clients
	clients := make([]*Client, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		clients[i] = newTestClient(hub, tournamentID, uuid.New())
	}

	// Half goroutines register, half broadcast
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			hub.register <- clients[idx]
		}(i)
	}

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hub.Broadcast(tournamentID, string(MessageTypeMatchUpdate), map[string]string{"data": "test"})
		}()
	}

	wg.Wait()

	// Wait for all registrations to complete
	waitForStats(t, hub, "total_clients", numGoroutines)

	stats := hub.GetStats()
	assert.Equal(t, 1, stats["tournaments"])
	assert.Equal(t, numGoroutines, stats["total_clients"])
}
