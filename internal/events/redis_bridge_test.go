package events

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLogger(t *testing.T) *logger.Logger {
	t.Helper()
	log, err := logger.New("error", "json")
	require.NoError(t, err)
	return log
}

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

func (a *redisCacheAdapter) Publish(ctx context.Context, channel string, message any) error {
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

	// подписка на канал, чтобы перехватить опубликованные сообщения
	ctx := context.Background()
	pubsub := client.Subscribe(ctx, defaultChannel)
	defer pubsub.Close()

	// ожидание готовности подписки.
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

	err = pub.Publish(ctx, "MatchResultProcessed", event)
	require.NoError(t, err)

	// чтение опубликованного сообщения.
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

// отменённый контекст запроса не мешает отправить событие
func TestRedisEventPublisher_IgnoresCancel(t *testing.T) {
	client, _ := newTestRedisClient(t)
	pub := NewRedisEventPublisher(&redisCacheAdapter{client: client}, newTestLogger(t))

	pubsub := client.Subscribe(t.Context(), defaultChannel)
	defer pubsub.Close()
	_, err := pubsub.Receive(t.Context())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.NoError(t, pub.Publish(ctx, "TournamentStarted", TournamentStarted{TournamentID: uuid.New()}))

	msg, err := pubsub.ReceiveMessage(t.Context())
	require.NoError(t, err)
	assert.Contains(t, msg.Payload, "TournamentStarted")
}

func TestRedisEventSubscriber_ReceivesAndRepublishes(t *testing.T) {
	client, _ := newTestRedisClient(t)
	log := newTestLogger(t)
	adapter := &redisCacheAdapter{client: client}

	// шина, записывающая опубликованные события
	var mu sync.Mutex
	var receivedEvents []any
	recordingBus := &recordingNotifier{
		onPublish: func(event any) {
			mu.Lock()
			receivedEvents = append(receivedEvents, event)
			mu.Unlock()
		},
	}

	sub := NewRedisEventSubscriber(adapter, recordingBus, log)

	ctx := t.Context()

	go sub.Start(ctx)

	// подписчику даётся время подключиться.
	time.Sleep(100 * time.Millisecond)

	// событие публикуется напрямую через Redis
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

	// ожидание получения и повторной публикации события.
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
	recordingBus := &recordingNotifier{
		onPublish: func(event any) {
			mu.Lock()
			receivedEvents = append(receivedEvents, event)
			mu.Unlock()
		},
	}

	sub := NewRedisEventSubscriber(adapter, recordingBus, log)

	ctx := t.Context()

	go sub.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	// публикация неизвестного типа события
	env := envelope{Type: "UnknownEventType", Data: json.RawMessage(`{"foo":"bar"}`)}
	payload, _ := json.Marshal(env)
	err := client.Publish(ctx, defaultChannel, payload).Err()
	require.NoError(t, err)

	// небольшая пауза, чтобы убедиться, что событие не перепубликовывается.
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	assert.Empty(t, receivedEvents)
	mu.Unlock()
}

func TestRedisEventSubscriber_Stop(t *testing.T) {
	client, _ := newTestRedisClient(t)
	log := newTestLogger(t)
	adapter := &redisCacheAdapter{client: client}

	sub := NewRedisEventSubscriber(adapter, NoopNotifier{}, log)

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
	recordingBus := &recordingNotifier{
		onPublish: func(event any) {
			mu.Lock()
			receivedEvents = append(receivedEvents, event)
			mu.Unlock()
		},
	}

	// настройка подписчика
	sub := NewRedisEventSubscriber(adapter, recordingBus, log)
	ctx := t.Context()
	go sub.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	// настройка publisher'а
	pub := NewRedisEventPublisher(adapter, log)

	// публикация через Handler publisher'а
	event := MatchResultProcessed{
		TournamentID: uuid.New(),
		MatchID:      uuid.New(),
		Program1ID:   uuid.New(),
		Program2ID:   uuid.New(),
		NewRating1:   1600,
		NewRating2:   1400,
		Winner:       1,
	}

	err := pub.Publish(ctx, "MatchResultProcessed", event)
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
	recordingBus := &recordingNotifier{
		onPublish: func(event any) {
			mu.Lock()
			receivedEvents = append(receivedEvents, event)
			mu.Unlock()
		},
	}

	sub := NewRedisEventSubscriber(adapter, recordingBus, log)

	ctx := t.Context()

	go sub.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	// публикация невалидного JSON (не корректный envelope).
	err := client.Publish(ctx, defaultChannel, "not valid json{{{").Err()
	require.NoError(t, err)

	// небольшая пауза - ни одно событие не должно быть перепубликовано.
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
	recordingBus := &recordingNotifier{
		onPublish: func(event any) {
			mu.Lock()
			receivedEvents = append(receivedEvents, event)
			mu.Unlock()
		},
	}

	sub := NewRedisEventSubscriber(adapter, recordingBus, log)

	ctx := t.Context()

	go sub.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	// Корректный envelope, но невалидные данные для MatchResultProcessed (UUID-поля не распарсятся из числа).
	env := envelope{Type: "MatchResultProcessed", Data: json.RawMessage(`{invalid json`)}
	payload, _ := json.Marshal(env)
	err := client.Publish(ctx, defaultChannel, payload).Err()
	require.NoError(t, err)

	// небольшая пауза - ни одно событие не должно быть перепубликовано из-за ошибки unmarshal.
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	assert.Empty(t, receivedEvents)
	mu.Unlock()
}

func TestRedisEventPublisher_Handle_PublishError(t *testing.T) {
	log := newTestLogger(t)

	// publisher, который всегда возвращает ошибку
	failPub := &failingPublisher{}
	pub := NewRedisEventPublisher(failPub, log)

	event := MatchResultProcessed{
		TournamentID: uuid.New(),
		MatchID:      uuid.New(),
	}

	err := pub.Publish(context.Background(), "MatchResultProcessed", event)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis publisher: publish")
}

func TestRedisEventSubscriber_DoubleStop(t *testing.T) {
	client, _ := newTestRedisClient(t)
	log := newTestLogger(t)
	adapter := &redisCacheAdapter{client: client}

	sub := NewRedisEventSubscriber(adapter, NoopNotifier{}, log)

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

// события апи через редис доходят до вебсокета любой реплики, включая свою
func TestRedisBridge_ApiEventsReachBroadcaster(t *testing.T) {
	client, _ := newTestRedisClient(t)
	log := newTestLogger(t)
	adapter := &redisCacheAdapter{client: client}
	ctx := t.Context()

	got := make(chanBroadcaster, 3)
	sub := NewRedisEventSubscriber(adapter, &SyncNotifier{Broadcaster: got, Log: log}, log)
	go sub.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	api := &SyncNotifier{Redis: NewRedisEventPublisher(adapter, log), Log: log}
	tid := uuid.New()
	api.TournamentStarted(ctx, TournamentStarted{Version: 1, TournamentID: tid, Status: models.TournamentActive})
	api.TournamentCompleted(ctx, TournamentCompleted{Version: 1, TournamentID: tid, Status: models.TournamentCompleted})

	for _, want := range []string{"tournament_update", "tournament_update"} {
		select {
		case c := <-got:
			assert.Equal(t, want, c.messageType)
			assert.Equal(t, tid, c.tournamentID)
		case <-time.After(2 * time.Second):
			t.Fatalf("не дошло %s", want)
		}
	}
}

// chanBroadcaster отдаёт рассылки в канал, подписчик зовёт его из своей горутины
type chanBroadcaster chan broadcastCall

func (c chanBroadcaster) Broadcast(tid uuid.UUID, mt string, p any) {
	c <- broadcastCall{tid, mt, p}
}

// failingPublisher всегда возвращает ошибку при вызове Publish.
type failingPublisher struct{}

func (f *failingPublisher) Publish(_ context.Context, _ string, _ any) error {
	return assert.AnError
}

// recordingNotifier - тестовый Notifier, отдаёт полученные события в колбэк.
// остальные методы берутся из NoopNotifier
type recordingNotifier struct {
	NoopNotifier
	onPublish func(event any)
}

func (n *recordingNotifier) MatchResultProcessed(_ context.Context, e MatchResultProcessed) {
	if n.onPublish != nil {
		n.onPublish(e)
	}
}

func (n *recordingNotifier) ProgramCompiled(_ context.Context, e ProgramCompiled) {
	if n.onPublish != nil {
		n.onPublish(e)
	}
}
