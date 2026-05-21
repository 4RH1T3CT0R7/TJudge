package websocket

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClient_CloseSendIdempotent гарантирует, что повторные close()
// безопасны.
func TestClient_CloseSendIdempotent(t *testing.T) {
	log, _ := logger.New("error", "json")
	hub := NewHub(log)
	client := NewClient(hub, nil, uuid.New(), uuid.New(), log)

	assert.False(t, client.IsClosed())

	// Первый close закрывает канал
	client.CloseSend()
	assert.True(t, client.IsClosed())
	// Канал должен быть закрыт, чтение возвращает ok=false
	_, ok := <-client.send
	assert.False(t, ok)

	// Повторные close() должны быть no-op без panic
	client.CloseSend()
	client.CloseSend()
	assert.True(t, client.IsClosed())
}

// TestClient_ReadLimiterInitialized защищает от регрессии: при NewClient
// readLimiter должен быть создан и разрешать первые burst-сообщений.
func TestClient_ReadLimiterInitialized(t *testing.T) {
	log, _ := logger.New("error", "json")
	hub := NewHub(log)
	client := NewClient(hub, nil, uuid.New(), uuid.New(), log)

	// Все первые clientMessageBurst Allow() должны пройти.
	for i := range clientMessageBurst {
		assert.True(t, client.readLimiter.Allow(), "burst msg %d must be allowed", i)
	}
	// Следующий Allow после burst должен быть false (rate limit).
	assert.False(t, client.readLimiter.Allow(), "after burst, next message must be limited")
}

// TestClient_CloseSendConcurrent - регрессия на race condition.
// Много горутин одновременно вызывают CloseSend(); только одна должна
// реально закрыть канал, остальные - no-op. Запускается с -race.
func TestClient_CloseSendConcurrent(t *testing.T) {
	log, _ := logger.New("error", "json")
	hub := NewHub(log)
	client := NewClient(hub, nil, uuid.New(), uuid.New(), log)

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			client.CloseSend()
		}()
	}
	wg.Wait()
	assert.True(t, client.IsClosed())
}

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

	// Не должно паниковать
	client.handleMessage([]byte("not valid json"))

	select {
	case <-client.send:
		t.Fatal("should not receive anything on invalid JSON")
	default:
		// Ок
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
		// Ок
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

	// Создаём клиент с send-буфером размера 1
	client := &Client{
		hub:          hub,
		conn:         nil,
		send:         make(chan []byte, 1),
		tournamentID: tournamentID,
		userID:       uuid.New(),
		log:          log,
	}

	// Заполняем send-буфер
	client.send <- []byte("filler")

	// sendPong не должен паниковать при полном буфере
	assert.NotPanics(t, func() {
		client.sendPong()
	})

	// В буфере должно остаться только filler-сообщение
	require.Len(t, client.send, 1)
	data := <-client.send
	assert.Equal(t, []byte("filler"), data)
}

func TestClient_sendPong_ClosedChannel_NoPanic(t *testing.T) {
	hub := newTestHub(t)
	tournamentID := uuid.New()
	log, _ := logger.New("error", "json")

	client := &Client{
		hub:          hub,
		conn:         nil,
		send:         make(chan []byte, 1),
		tournamentID: tournamentID,
		userID:       uuid.New(),
		log:          log,
	}

	// Закрываем канал, чтобы сработала ветка recover
	close(client.send)

	assert.NotPanics(t, func() {
		client.sendPong()
	})
}

func TestClient_handleMessage_EmptyPayload(t *testing.T) {
	hub := newTestHub(t)
	tournamentID := uuid.New()
	client := newTestClient(hub, tournamentID, uuid.New())

	// Валидное JSON-сообщение без поля payload
	msg := Message{
		TournamentID: tournamentID,
		Type:         MessageTypePing,
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	// Должен корректно обработать и отправить pong в ответ
	assert.NotPanics(t, func() {
		client.handleMessage(data)
	})

	// Поскольку тип ping, pong всё равно должен прийти
	select {
	case pongData := <-client.send:
		var pong Message
		require.NoError(t, json.Unmarshal(pongData, &pong))
		assert.Equal(t, MessageTypePong, pong.Type)
		assert.Equal(t, tournamentID, pong.TournamentID)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for pong response")
	}

	// Также проверяем с неизвестным типом и nil payload - не должно паниковать, сообщение не отправляется
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
		// Ок
	}
}
