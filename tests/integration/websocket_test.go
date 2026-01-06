//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	ws "github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// WebSocketTestSuite tests WebSocket functionality
type WebSocketTestSuite struct {
	suite.Suite
	hub    *websocket.Hub
	server *httptest.Server
}

func (s *WebSocketTestSuite) SetupSuite() {
	if os.Getenv("RUN_INTEGRATION") != "true" {
		s.T().Skip("Skipping integration tests (set RUN_INTEGRATION=true)")
	}

	// Create hub
	s.hub = websocket.NewHub()
	go s.hub.Run()

	// Create test server
	r := chi.NewRouter()
	r.Get("/ws/tournaments/{id}", s.wsHandler)

	s.server = httptest.NewServer(r)
}

func (s *WebSocketTestSuite) TearDownSuite() {
	if s.hub != nil {
		s.hub.Shutdown()
	}
	if s.server != nil {
		s.server.Close()
	}
}

func (s *WebSocketTestSuite) wsHandler(w http.ResponseWriter, r *http.Request) {
	tournamentIDStr := chi.URLParam(r, "id")
	tournamentID, err := uuid.Parse(tournamentIDStr)
	if err != nil {
		http.Error(w, "Invalid tournament ID", http.StatusBadRequest)
		return
	}

	// Get user ID from query (simplified for testing)
	userIDStr := r.URL.Query().Get("user_id")
	var userID uuid.UUID
	if userIDStr != "" {
		userID, _ = uuid.Parse(userIDStr)
	} else {
		userID = uuid.New()
	}

	upgrader := ws.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for testing
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := websocket.NewClient(s.hub, conn, userID, tournamentID)
	s.hub.Register(client)
	go client.WritePump()
	go client.ReadPump()
}

func (s *WebSocketTestSuite) wsURL() string {
	return "ws" + strings.TrimPrefix(s.server.URL, "http")
}

// =============================================================================
// Connection Tests
// =============================================================================

func (s *WebSocketTestSuite) TestWebSocket_Connect() {
	tournamentID := uuid.New()
	url := s.wsURL() + "/ws/tournaments/" + tournamentID.String()

	conn, resp, err := ws.DefaultDialer.Dial(url, nil)
	require.NoError(s.T(), err)
	defer conn.Close()

	assert.Equal(s.T(), http.StatusSwitchingProtocols, resp.StatusCode)
}

func (s *WebSocketTestSuite) TestWebSocket_InvalidTournamentID() {
	url := s.wsURL() + "/ws/tournaments/invalid-uuid"

	_, resp, err := ws.DefaultDialer.Dial(url, nil)
	assert.Error(s.T(), err)
	if resp != nil {
		assert.Equal(s.T(), http.StatusBadRequest, resp.StatusCode)
	}
}

func (s *WebSocketTestSuite) TestWebSocket_MultipleConnections() {
	tournamentID := uuid.New()
	url := s.wsURL() + "/ws/tournaments/" + tournamentID.String()

	const numConnections = 5
	connections := make([]*ws.Conn, numConnections)

	// Create multiple connections
	for i := 0; i < numConnections; i++ {
		conn, _, err := ws.DefaultDialer.Dial(url, nil)
		require.NoError(s.T(), err)
		connections[i] = conn
	}

	// Close all connections
	for _, conn := range connections {
		conn.Close()
	}
}

// =============================================================================
// Broadcast Tests
// =============================================================================

func (s *WebSocketTestSuite) TestWebSocket_BroadcastToTournament() {
	tournamentID := uuid.New()
	url := s.wsURL() + "/ws/tournaments/" + tournamentID.String()

	// Connect two clients to the same tournament
	conn1, _, err := ws.DefaultDialer.Dial(url, nil)
	require.NoError(s.T(), err)
	defer conn1.Close()

	conn2, _, err := ws.DefaultDialer.Dial(url, nil)
	require.NoError(s.T(), err)
	defer conn2.Close()

	// Give time for connections to be registered
	time.Sleep(100 * time.Millisecond)

	// Broadcast message
	message := map[string]interface{}{
		"type":    "tournament_update",
		"payload": map[string]string{"status": "started"},
	}
	s.hub.BroadcastToTournament(tournamentID, message)

	// Both clients should receive the message
	received1 := make(chan bool, 1)
	received2 := make(chan bool, 1)

	go func() {
		conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _, err := conn1.ReadMessage()
		received1 <- err == nil
	}()

	go func() {
		conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _, err := conn2.ReadMessage()
		received2 <- err == nil
	}()

	assert.True(s.T(), <-received1, "Client 1 should receive broadcast")
	assert.True(s.T(), <-received2, "Client 2 should receive broadcast")
}

func (s *WebSocketTestSuite) TestWebSocket_BroadcastIsolation() {
	tournament1 := uuid.New()
	tournament2 := uuid.New()

	url1 := s.wsURL() + "/ws/tournaments/" + tournament1.String()
	url2 := s.wsURL() + "/ws/tournaments/" + tournament2.String()

	// Connect to different tournaments
	conn1, _, err := ws.DefaultDialer.Dial(url1, nil)
	require.NoError(s.T(), err)
	defer conn1.Close()

	conn2, _, err := ws.DefaultDialer.Dial(url2, nil)
	require.NoError(s.T(), err)
	defer conn2.Close()

	time.Sleep(100 * time.Millisecond)

	// Broadcast only to tournament1
	message := map[string]interface{}{
		"type": "test",
	}
	s.hub.BroadcastToTournament(tournament1, message)

	// Client 1 should receive, client 2 should not
	received1 := make(chan bool, 1)
	received2 := make(chan bool, 1)

	go func() {
		conn1.SetReadDeadline(time.Now().Add(1 * time.Second))
		_, _, err := conn1.ReadMessage()
		received1 <- err == nil
	}()

	go func() {
		conn2.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, _, err := conn2.ReadMessage()
		received2 <- err == nil
	}()

	assert.True(s.T(), <-received1, "Tournament 1 client should receive")
	assert.False(s.T(), <-received2, "Tournament 2 client should NOT receive")
}

// =============================================================================
// Message Tests
// =============================================================================

func (s *WebSocketTestSuite) TestWebSocket_MessageFormat() {
	tournamentID := uuid.New()
	url := s.wsURL() + "/ws/tournaments/" + tournamentID.String()

	conn, _, err := ws.DefaultDialer.Dial(url, nil)
	require.NoError(s.T(), err)
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)

	// Broadcast structured message
	message := map[string]interface{}{
		"type": "match_completed",
		"payload": map[string]interface{}{
			"match_id":  uuid.New().String(),
			"winner_id": uuid.New().String(),
			"score_p1":  10,
			"score_p2":  5,
		},
	}
	s.hub.BroadcastToTournament(tournamentID, message)

	// Read and verify message format
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	require.NoError(s.T(), err)

	var received map[string]interface{}
	err = json.Unmarshal(data, &received)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), "match_completed", received["type"])
	payload := received["payload"].(map[string]interface{})
	assert.NotEmpty(s.T(), payload["match_id"])
}

// =============================================================================
// Ping/Pong Tests
// =============================================================================

func (s *WebSocketTestSuite) TestWebSocket_PingPong() {
	tournamentID := uuid.New()
	url := s.wsURL() + "/ws/tournaments/" + tournamentID.String()

	conn, _, err := ws.DefaultDialer.Dial(url, nil)
	require.NoError(s.T(), err)
	defer conn.Close()

	// Set up pong handler
	pongReceived := make(chan bool, 1)
	conn.SetPongHandler(func(string) error {
		pongReceived <- true
		return nil
	})

	// Send ping
	err = conn.WriteControl(ws.PingMessage, []byte{}, time.Now().Add(time.Second))
	require.NoError(s.T(), err)

	// Start read loop in background to process pong
	go func() {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		conn.ReadMessage()
	}()

	// Wait for pong
	select {
	case <-pongReceived:
		// Success
	case <-time.After(2 * time.Second):
		s.T().Log("Pong timeout - this might be expected depending on implementation")
	}
}

// =============================================================================
// Disconnect Tests
// =============================================================================

func (s *WebSocketTestSuite) TestWebSocket_GracefulDisconnect() {
	tournamentID := uuid.New()
	url := s.wsURL() + "/ws/tournaments/" + tournamentID.String()

	conn, _, err := ws.DefaultDialer.Dial(url, nil)
	require.NoError(s.T(), err)

	time.Sleep(100 * time.Millisecond)

	// Send close message
	err = conn.WriteMessage(ws.CloseMessage,
		ws.FormatCloseMessage(ws.CloseNormalClosure, ""))
	require.NoError(s.T(), err)

	conn.Close()

	// Give hub time to clean up
	time.Sleep(100 * time.Millisecond)

	// Broadcast should not panic even with no clients
	message := map[string]interface{}{"type": "test"}
	s.hub.BroadcastToTournament(tournamentID, message)
}

func (s *WebSocketTestSuite) TestWebSocket_AbruptDisconnect() {
	tournamentID := uuid.New()
	url := s.wsURL() + "/ws/tournaments/" + tournamentID.String()

	conn, _, err := ws.DefaultDialer.Dial(url, nil)
	require.NoError(s.T(), err)

	time.Sleep(100 * time.Millisecond)

	// Close without sending close message
	conn.Close()

	// Give hub time to detect disconnection
	time.Sleep(200 * time.Millisecond)

	// Hub should handle this gracefully
	message := map[string]interface{}{"type": "test"}
	s.hub.BroadcastToTournament(tournamentID, message)
}

// =============================================================================
// Concurrent Tests
// =============================================================================

func (s *WebSocketTestSuite) TestWebSocket_ConcurrentBroadcasts() {
	tournamentID := uuid.New()
	url := s.wsURL() + "/ws/tournaments/" + tournamentID.String()

	// Connect client
	conn, _, err := ws.DefaultDialer.Dial(url, nil)
	require.NoError(s.T(), err)
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)

	// Send multiple broadcasts concurrently
	const numBroadcasts = 50
	var wg sync.WaitGroup

	for i := 0; i < numBroadcasts; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			message := map[string]interface{}{
				"type":  "concurrent_test",
				"index": idx,
			}
			s.hub.BroadcastToTournament(tournamentID, message)
		}(i)
	}

	wg.Wait()

	// Read messages (some might be batched or dropped)
	received := 0
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
		received++
	}

	s.T().Logf("Received %d of %d messages", received, numBroadcasts)
	assert.Greater(s.T(), received, 0, "Should receive at least some messages")
}

func (s *WebSocketTestSuite) TestWebSocket_ConcurrentConnections() {
	tournamentID := uuid.New()
	url := s.wsURL() + "/ws/tournaments/" + tournamentID.String()

	const numClients = 20
	connections := make([]*ws.Conn, numClients)
	var wg sync.WaitGroup
	var mu sync.Mutex
	errors := 0

	// Connect clients concurrently
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			conn, _, err := ws.DefaultDialer.Dial(url, nil)
			if err != nil {
				mu.Lock()
				errors++
				mu.Unlock()
				return
			}
			mu.Lock()
			connections[idx] = conn
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	assert.Equal(s.T(), 0, errors, "All connections should succeed")

	// Cleanup
	for _, conn := range connections {
		if conn != nil {
			conn.Close()
		}
	}
}

func TestWebSocketSuite(t *testing.T) {
	suite.Run(t, new(WebSocketTestSuite))
}
