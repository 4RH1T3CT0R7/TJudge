package events

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRedisClient(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	return client, mr
}

// redisCacheAdapter оборачивает *redis.Client под интерфейсы publisher/subscriber.
type redisCacheAdapter struct {
	client *redis.Client
}

func (a *redisCacheAdapter) Publish(ctx context.Context, channel string, message interface{}) error {
	return a.client.Publish(ctx, channel, message).Err()
}

func (a *redisCacheAdapter) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return a.client.Subscribe(ctx, channels...)
}

func TestRedisEventPublisher_Handle(t *testing.T) {
	client, _ := newTestRedisClient(t)
	log := newTestLogger(t)
	adapter := &redisCacheAdapter{client: client}

	pub := NewRedisEventPublisher(adapter, log)

	// Подписываемся на канал, чтобы перехватить опубликованные сообщения.
	ctx := context.Background()
	pubsub := client.Subscribe(ctx, defaultChannel)
	defer pubsub.Close()

	// Ждём готовности подписки.
	_, err := pubsub.Receive(ctx)
	require.NoError(t, err)

	event := MatchResultProcessed{
		TournamentID: uuid.New(),
		MatchID:      uuid.New(),
		Program1ID:   uuid.New(),
		Program2ID:   uuid.New(),
		NewRating1:   1520,
		NewRating2:   1480,
		Winner:       1,
	}

	err = pub.Handle(ctx, event)
	require.NoError(t, err)

	// Читаем опубликованное сообщение.
	msg, err := pubsub.ReceiveMessage(ctx)
	require.NoError(t, err)

	var env envelope
	err = json.Unmarshal([]byte(msg.Payload), &env)
	require.NoError(t, err)

	assert.Equal(t, "MatchResultProcessed", env.Type)

	var received MatchResultProcessed
	err = json.Unmarshal(env.Data, &received)
	require.NoError(t, err)

	assert.Equal(t, event.TournamentID, received.TournamentID)
	assert.Equal(t, event.MatchID, received.MatchID)
	assert.Equal(t, event.NewRating1, received.NewRating1)
	assert.Equal(t, event.NewRating2, received.NewRating2)
	assert.Equal(t, event.Winner, received.Winner)
}

func TestRedisEventSubscriber_ReceivesAndRepublishes(t *testing.T) {
	client, _ := newTestRedisClient(t)
	log := newTestLogger(t)
	adapter := &redisCacheAdapter{client: client}

	// Создаём шину, записывающую опубликованные события.
	var mu sync.Mutex
	var receivedEvents []any
	recordingBus := &recordingBus{
		onPublish: func(event any) {
			mu.Lock()
			receivedEvents = append(receivedEvents, event)
			mu.Unlock()
		},
	}

	sub := NewRedisEventSubscriber(adapter, recordingBus, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go sub.Start(ctx)

	// Даём подписчику время подключиться.
	time.Sleep(100 * time.Millisecond)

	// Публикуем событие напрямую через Redis.
	event := MatchResultProcessed{
		TournamentID: uuid.New(),
		MatchID:      uuid.New(),
		Program1ID:   uuid.New(),
		Program2ID:   uuid.New(),
		NewRating1:   1550,
		NewRating2:   1450,
		Winner:       2,
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	env := envelope{Type: "MatchResultProcessed", Data: data}
	payload, err := json.Marshal(env)
	require.NoError(t, err)

	err = client.Publish(ctx, defaultChannel, payload).Err()
	require.NoError(t, err)

	// Ждём получения и повторной публикации события.
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(receivedEvents) == 1
	}, 2*time.Second, 50*time.Millisecond)

	mu.Lock()
	received, ok := receivedEvents[0].(MatchResultProcessed)
	mu.Unlock()
	require.True(t, ok, "expected MatchResultProcessed, got %T", receivedEvents[0])

	assert.Equal(t, event.TournamentID, received.TournamentID)
	assert.Equal(t, event.NewRating1, received.NewRating1)
	assert.Equal(t, event.Winner, received.Winner)
}

func TestRedisEventSubscriber_UnknownTypeIgnored(t *testing.T) {
	client, _ := newTestRedisClient(t)
	log := newTestLogger(t)
	adapter := &redisCacheAdapter{client: client}

	var mu sync.Mutex
	var receivedEvents []any
	recordingBus := &recordingBus{
		onPublish: func(event any) {
			mu.Lock()
			receivedEvents = append(receivedEvents, event)
			mu.Unlock()
		},
	}

	sub := NewRedisEventSubscriber(adapter, recordingBus, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go sub.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	// Публикуем неизвестный тип события.
	env := envelope{Type: "UnknownEventType", Data: json.RawMessage(`{"foo":"bar"}`)}
	payload, _ := json.Marshal(env)
	err := client.Publish(ctx, defaultChannel, payload).Err()
	require.NoError(t, err)

	// Немного ждём, чтобы убедиться, что событие не перепубликовывается.
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	assert.Empty(t, receivedEvents)
	mu.Unlock()
}

func TestRedisEventSubscriber_Stop(t *testing.T) {
	client, _ := newTestRedisClient(t)
	log := newTestLogger(t)
	adapter := &redisCacheAdapter{client: client}

	sub := NewRedisEventSubscriber(adapter, NoopBus{}, log)

	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		sub.Start(ctx)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	sub.Stop()

	select {
	case <-done:
		// Ок, подписчик остановлен.
	case <-time.After(2 * time.Second):
		t.Fatal("subscriber did not stop within timeout")
	}
}

func TestRedisEndToEnd_PublisherToSubscriber(t *testing.T) {
	client, _ := newTestRedisClient(t)
	log := newTestLogger(t)
	adapter := &redisCacheAdapter{client: client}

	var mu sync.Mutex
	var receivedEvents []any
	recordingBus := &recordingBus{
		onPublish: func(event any) {
			mu.Lock()
			receivedEvents = append(receivedEvents, event)
			mu.Unlock()
		},
	}

	// Настраиваем подписчика.
	sub := NewRedisEventSubscriber(adapter, recordingBus, log)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sub.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	// Настраиваем publisher.
	pub := NewRedisEventPublisher(adapter, log)

	// Публикуем через Handler publisher'а.
	event := MatchResultProcessed{
		TournamentID: uuid.New(),
		MatchID:      uuid.New(),
		Program1ID:   uuid.New(),
		Program2ID:   uuid.New(),
		NewRating1:   1600,
		NewRating2:   1400,
		Winner:       1,
	}

	err := pub.Handle(ctx, event)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(receivedEvents) == 1
	}, 2*time.Second, 50*time.Millisecond)

	mu.Lock()
	received, ok := receivedEvents[0].(MatchResultProcessed)
	mu.Unlock()
	require.True(t, ok)

	assert.Equal(t, event.TournamentID, received.TournamentID)
	assert.Equal(t, event.NewRating1, received.NewRating1)
}

func TestRedisEventSubscriber_InvalidEnvelopeJSON(t *testing.T) {
	client, _ := newTestRedisClient(t)
	log := newTestLogger(t)
	adapter := &redisCacheAdapter{client: client}

	var mu sync.Mutex
	var receivedEvents []any
	recordingBus := &recordingBus{
		onPublish: func(event any) {
			mu.Lock()
			receivedEvents = append(receivedEvents, event)
			mu.Unlock()
		},
	}

	sub := NewRedisEventSubscriber(adapter, recordingBus, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go sub.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	// Публикуем невалидный JSON (не корректный envelope).
	err := client.Publish(ctx, defaultChannel, "not valid json{{{").Err()
	require.NoError(t, err)

	// Немного ждём - ни одно событие не должно быть перепубликовано.
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	assert.Empty(t, receivedEvents)
	mu.Unlock()
}

func TestRedisEventSubscriber_InvalidEventData(t *testing.T) {
	client, _ := newTestRedisClient(t)
	log := newTestLogger(t)
	adapter := &redisCacheAdapter{client: client}

	var mu sync.Mutex
	var receivedEvents []any
	recordingBus := &recordingBus{
		onPublish: func(event any) {
			mu.Lock()
			receivedEvents = append(receivedEvents, event)
			mu.Unlock()
		},
	}

	sub := NewRedisEventSubscriber(adapter, recordingBus, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go sub.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	// Корректный envelope, но невалидные данные для MatchResultProcessed (UUID-поля не распарсятся из числа).
	env := envelope{Type: "MatchResultProcessed", Data: json.RawMessage(`{invalid json`)}
	payload, _ := json.Marshal(env)
	err := client.Publish(ctx, defaultChannel, payload).Err()
	require.NoError(t, err)

	// Немного ждём - ни одно событие не должно быть перепубликовано из-за ошибки unmarshal.
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	assert.Empty(t, receivedEvents)
	mu.Unlock()
}

func TestRedisEventPublisher_Handle_PublishError(t *testing.T) {
	log := newTestLogger(t)

	// Используем publisher, который всегда возвращает ошибку.
	failPub := &failingPublisher{}
	pub := NewRedisEventPublisher(failPub, log)

	event := MatchResultProcessed{
		TournamentID: uuid.New(),
		MatchID:      uuid.New(),
	}

	err := pub.Handle(context.Background(), event)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis publisher: publish")
}

func TestRedisEventSubscriber_DoubleStop(t *testing.T) {
	client, _ := newTestRedisClient(t)
	log := newTestLogger(t)
	adapter := &redisCacheAdapter{client: client}

	sub := NewRedisEventSubscriber(adapter, NoopBus{}, log)

	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		sub.Start(ctx)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)

	// Повторный Stop не должен паниковать.
	sub.Stop()
	assert.NotPanics(t, func() { sub.Stop() })

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("subscriber did not stop within timeout")
	}
}

// failingPublisher всегда возвращает ошибку при вызове Publish.
type failingPublisher struct{}

func (f *failingPublisher) Publish(_ context.Context, _ string, _ interface{}) error {
	return assert.AnError
}

// recordingBus - тестовый Bus, записывающий все опубликованные события.
type recordingBus struct {
	onPublish func(event any)
}

func (b *recordingBus) Publish(_ context.Context, event any) {
	if b.onPublish != nil {
		b.onPublish(event)
	}
}

func (b *recordingBus) Subscribe(Handler, ...any) {}
