package websocket

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient_Fields(t *testing.T) {
	log, _ := logger.New("error", "json")
	hub := NewHub(log)
	tournamentID := uuid.New()
	userID := uuid.New()

	client := NewClient(hub, nil, tournamentID, userID, log)

	assert.Equal(t, hub, client.hub)
	assert.Nil(t, client.conn)
	assert.NotNil(t, client.send)
	assert.Equal(t, tournamentID, client.tournamentID)
	assert.Equal(t, userID, client.userID)
	assert.Equal(t, log, client.log)
}

func TestClient_Register_SendsToHub(t *testing.T) {
	hub := newTestHub(t)
	tournamentID := uuid.New()
	userID := uuid.New()
	client := newTestClient(hub, tournamentID, userID)

	go client.Register()

	select {
	case received := <-hub.register:
		assert.Equal(t, client, received)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for register")
	}
}

func TestClient_handleMessage_Ping(t *testing.T) {
	hub := newTestHub(t)
	tournamentID := uuid.New()
	client := newTestClient(hub, tournamentID, uuid.New())

	msg := Message{
		TournamentID: tournamentID,
		Type:         MessageTypePing,
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	client.handleMessage(data)

	select {
	case pongData := <-client.send:
		var pong Message
		require.NoError(t, json.Unmarshal(pongData, &pong))
		assert.Equal(t, MessageTypePong, pong.Type)
		assert.Equal(t, tournamentID, pong.TournamentID)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for pong")
	}
}

func TestClient_handleMessage_InvalidJSON(t *testing.T) {
	hub := newTestHub(t)
	client := newTestClient(hub, uuid.New(), uuid.New())

	// Should not panic
	client.handleMessage([]byte("not valid json"))

	select {
	case <-client.send:
		t.Fatal("should not receive anything on invalid JSON")
	default:
		// OK
	}
}

func TestClient_handleMessage_UnknownType(t *testing.T) {
	hub := newTestHub(t)
	tournamentID := uuid.New()
	client := newTestClient(hub, tournamentID, uuid.New())

	msg := Message{
		TournamentID: tournamentID,
		Type:         MessageType("unknown_type"),
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	client.handleMessage(data)

	select {
	case <-client.send:
		t.Fatal("should not receive anything on unknown type")
	default:
		// OK
	}
}

func TestClient_sendPong(t *testing.T) {
	hub := newTestHub(t)
	tournamentID := uuid.New()
	client := newTestClient(hub, tournamentID, uuid.New())

	client.sendPong()

	select {
	case data := <-client.send:
		var msg Message
		require.NoError(t, json.Unmarshal(data, &msg))
		assert.Equal(t, MessageTypePong, msg.Type)
		assert.Equal(t, tournamentID, msg.TournamentID)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for sendPong")
	}
}

func TestClient_sendPong_BufferFull_NoPanic(t *testing.T) {
	hub := newTestHub(t)
	tournamentID := uuid.New()
	log, _ := logger.New("error", "json")

	// Create a client with a send buffer of size 1
	client := &Client{
		hub:          hub,
		conn:         nil,
		send:         make(chan []byte, 1),
		tournamentID: tournamentID,
		userID:       uuid.New(),
		log:          log,
	}

	// Fill the send buffer
	client.send <- []byte("filler")

	// sendPong should not panic when the buffer is full
	assert.NotPanics(t, func() {
		client.sendPong()
	})

	// The buffer should still contain only the filler message
	require.Len(t, client.send, 1)
	data := <-client.send
	assert.Equal(t, []byte("filler"), data)
}

func TestClient_handleMessage_EmptyPayload(t *testing.T) {
	hub := newTestHub(t)
	tournamentID := uuid.New()
	client := newTestClient(hub, tournamentID, uuid.New())

	// Valid JSON message with no payload field
	msg := Message{
		TournamentID: tournamentID,
		Type:         MessageTypePing,
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	// Should handle gracefully and send a pong response
	assert.NotPanics(t, func() {
		client.handleMessage(data)
	})

	// Since the type is ping, we should still get a pong back
	select {
	case pongData := <-client.send:
		var pong Message
		require.NoError(t, json.Unmarshal(pongData, &pong))
		assert.Equal(t, MessageTypePong, pong.Type)
		assert.Equal(t, tournamentID, pong.TournamentID)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for pong response")
	}

	// Also test with an unknown type and nil payload -- should not panic, no message sent
	unknownMsg := Message{
		TournamentID: tournamentID,
		Type:         MessageType("some_type"),
	}
	unknownData, err := json.Marshal(unknownMsg)
	require.NoError(t, err)

	assert.NotPanics(t, func() {
		client.handleMessage(unknownData)
	})

	select {
	case <-client.send:
		t.Fatal("should not receive anything for unknown type with empty payload")
	default:
		// OK
	}
}
