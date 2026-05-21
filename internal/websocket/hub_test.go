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

// newTestClient создаёт синтетического клиента для тестов (без реального WebSocket-соединения)
func newTestClient(hub *Hub, tournamentID, userID uuid.UUID) *Client {
	log, _ := logger.New("error", "json")
	return &Client{
		hub:          hub,
		conn:         nil, // без реального соединения
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
	// Даём горутине хаба время запуститься
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

	// Отмена регистрации никогда не зарегистрированного клиента не должна паниковать
	hub.unregister <- client

	// Даём время на обработку и проверяем, что статистика по-прежнему нулевая
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

	// Ждём, пока оба клиента получат сообщение
	require.Eventually(t, func() bool {
		return len(c1.send) == 1 && len(c2.send) == 1
	}, time.Second, time.Millisecond)

	// Оба клиента должны получить сообщение
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

	// Broadcast в турнир без клиентов не должен паниковать
	hub.broadcast <- &Message{
		TournamentID: uuid.New(),
		Type:         MessageTypeLeaderboardUpdate,
		Payload:      nil,
	}

	// Даём время на обработку
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
		// Хаб корректно завершился
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

	// Создаём "медленного" клиента с send-буфером размера 1
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

	// Заполняем send-буфер "медленного" клиента
	slowClient.send <- []byte("filler")

	// Рассылаем сообщение; буфер "медленного" клиента полон, поэтому он должен быть отключён
	hub.broadcast <- &Message{
		TournamentID: tournamentID,
		Type:         MessageTypeMatchUpdate,
		Payload:      map[string]string{"status": "completed"},
	}

	// "Медленный" клиент должен быть удалён из хаба
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

	// Первый unregister
	hub.unregister <- client
	waitForStats(t, hub, "total_clients", 0)

	// Повторный unregister не должен паниковать
	hub.unregister <- client

	// Даём время на обработку повторного unregister
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

	// Рассылаем в турнир B (где clientA НЕ зарегистрирован)
	hub.broadcast <- &Message{
		TournamentID: tournamentB,
		Type:         MessageTypeLeaderboardUpdate,
		Payload:      map[string]string{"rank": "1"},
	}

	// Даём время на обработку broadcast
	time.Sleep(10 * time.Millisecond)

	// Клиент A НЕ должен был получить ничего
	select {
	case <-clientA.send:
		t.Fatal("client should not receive a message broadcast to a different tournament")
	default:
		// Ок, сообщение не пришло
	}
}

func TestHub_RegisterClosedClient_Skips(t *testing.T) {
	hub := newTestHub(t)
	cancel := startHub(t, hub)
	defer cancel()

	tournamentID := uuid.New()
	client := newTestClient(hub, tournamentID, uuid.New())

	// Закрываем клиента до регистрации (идемпотентно через sync.Once)
	client.CloseSend()

	hub.register <- client
	// Даём время на обработку register
	time.Sleep(10 * time.Millisecond)

	stats := hub.GetStats()
	assert.Equal(t, 0, stats["total_clients"])
	assert.Equal(t, 0, stats["tournaments"])
}

func TestHub_Broadcast_ChannelFull_EventuallyDelivered(t *testing.T) {
	hub := newTestHub(t)
	cancel := startHub(t, hub)
	defer cancel()

	tournamentID := uuid.New()
	client := newTestClient(hub, tournamentID, uuid.New())

	hub.register <- client
	waitForStats(t, hub, "total_clients", 1)

	// Заполняем broadcast-канал хаба до ёмкости (256)
	for i := range 256 {
		hub.broadcast <- &Message{
			TournamentID: tournamentID,
			Type:         MessageTypeMatchUpdate,
			Payload:      map[string]int{"i": i},
		}
	}

	// Следующий Broadcast через публичный метод должен пойти по ветке с таймаутом,
	// но всё равно в итоге доставить сообщение (поскольку хаб обрабатывает)
	hub.Broadcast(tournamentID, string(MessageTypeLeaderboardUpdate), nil)

	// Сливаем client send-канал и проверяем, что сообщения доставлены
	require.Eventually(t, func() bool {
		return len(client.send) > 0
	}, 2*time.Second, 10*time.Millisecond)
}

func TestHub_ShutdownMultipleClients(t *testing.T) {
	hub := newTestHub(t)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		hub.Run(ctx)
		close(done)
	}()
	time.Sleep(5 * time.Millisecond)

	tid1 := uuid.New()
	tid2 := uuid.New()
	c1 := newTestClient(hub, tid1, uuid.New())
	c2 := newTestClient(hub, tid1, uuid.New())
	c3 := newTestClient(hub, tid2, uuid.New())

	hub.register <- c1
	hub.register <- c2
	hub.register <- c3
	waitForStats(t, hub, "total_clients", 3)

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Hub.Run did not return after context cancellation")
	}

	stats := hub.GetStats()
	assert.Equal(t, 0, stats["tournaments"])
	assert.Equal(t, 0, stats["total_clients"])

	// Все клиенты должны быть закрыты
	assert.True(t, c1.IsClosed())
	assert.True(t, c2.IsClosed())
	assert.True(t, c3.IsClosed())
}

func TestHub_ConcurrentRegisterBroadcast(t *testing.T) {
	hub := newTestHub(t)
	cancel := startHub(t, hub)
	defer cancel()

	tournamentID := uuid.New()
	const numGoroutines = 20

	var wg sync.WaitGroup

	// Конкурентно регистрируем клиентов
	clients := make([]*Client, numGoroutines)
	for i := range numGoroutines {
		clients[i] = newTestClient(hub, tournamentID, uuid.New())
	}

	// Половина горутин делает register, половина - broadcast
	for i := range numGoroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			hub.register <- clients[idx]
		}(i)
	}

	for range numGoroutines {
		wg.Go(func() {
			hub.Broadcast(tournamentID, string(MessageTypeMatchUpdate), map[string]string{"data": "test"})
		})
	}

	wg.Wait()

	// Ждём завершения всех регистраций
	waitForStats(t, hub, "total_clients", numGoroutines)

	stats := hub.GetStats()
	assert.Equal(t, 1, stats["tournaments"])
	assert.Equal(t, numGoroutines, stats["total_clients"])
}

func TestHub_Broadcast_ChannelFull_DroppedAfterTimeout(t *testing.T) {
	hub := newTestHub(t)
	// Хаб НЕ запускаем - broadcast-канал никогда не будет опустошён.

	tournamentID := uuid.New()

	// Заполняем broadcast-канал до ёмкости.
	for i := range 256 {
		hub.broadcast <- &Message{
			TournamentID: tournamentID,
			Type:         MessageTypeMatchUpdate,
			Payload:      i,
		}
	}

	// Этот вызов должен пойти по ветке с таймаутом и отбросить сообщение через ~1s.
	done := make(chan struct{})
	go func() {
		hub.Broadcast(tournamentID, string(MessageTypeLeaderboardUpdate), nil)
		close(done)
	}()

	select {
	case <-done:
		// Broadcast вернулся после таймаута.
	case <-time.After(3 * time.Second):
		t.Fatal("Broadcast did not return within timeout")
	}

	// В канале всё ещё должно быть ровно 256 элементов (отброшенное сообщение не добавлено).
	assert.Equal(t, 256, len(hub.broadcast))
}

func TestNoopBroadcaster(t *testing.T) {
	b := NewNoopBroadcaster()

	// Не должно паниковать.
	assert.NotPanics(t, func() {
		b.Broadcast(uuid.New(), "test", map[string]string{"key": "value"})
	})
}

func TestHub_BroadcastMessage_MarshalError(t *testing.T) {
	hub := newTestHub(t)
	cancel := startHub(t, hub)
	defer cancel()

	tournamentID := uuid.New()
	client := newTestClient(hub, tournamentID, uuid.New())

	hub.register <- client
	waitForStats(t, hub, "total_clients", 1)

	// Отправляем сообщение с payload, который не сериализуется.
	hub.broadcast <- &Message{
		TournamentID: tournamentID,
		Type:         MessageTypeMatchUpdate,
		Payload:      make(chan int), // каналы не сериализуются
	}

	// Даём хабу время на обработку.
	time.Sleep(50 * time.Millisecond)

	// Клиент должен остаться подключённым (не отключён из-за ошибки marshal).
	assert.Equal(t, 0, len(client.send), "no message should be sent to client")
	stats := hub.GetStats()
	assert.Equal(t, 1, stats["total_clients"])
}
