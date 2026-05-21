package events

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const defaultChannel = "tjudge:events"

// envelope оборачивает событие вместе с именем типа для JSON-сериализации.
type envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// redisPublisher - совместимый с cache.Cache интерфейс для публикации.
type redisPublisher interface {
	Publish(ctx context.Context, channel string, message any) error
}

// redisSubscriber - совместимый с cache.Cache интерфейс для подписки.
type redisSubscriber interface {
	Subscribe(ctx context.Context, channels ...string) *redis.PubSub
}

// eventTypeRegistry сопоставляет имя типа с reflect.Type для десериализации.
var eventTypeRegistry = map[string]reflect.Type{}

func init() {
	// Регистрируем все типы событий, которые проходят через Redis bridge.
	registerType(MatchResultProcessed{})
	registerType(TournamentStarted{})
	registerType(TournamentCompleted{})
	registerType(MatchesCreated{})
}

func registerType(v any) {
	t := reflect.TypeOf(v)
	eventTypeRegistry[t.Name()] = t
}

// RedisEventPublisher - Handler событий, пересылающий их в Redis Pub/Sub канал.
// Подключается к SyncBus, чтобы события, возникшие в одном процессе (например, worker),
// пересылались в другие процессы (например, API), подписанные на тот же канал Redis.
type RedisEventPublisher struct {
	pub     redisPublisher
	channel string
	log     *logger.Logger
}

// NewRedisEventPublisher создаёт publisher, отправляющий события в указанный Redis-канал.
func NewRedisEventPublisher(pub redisPublisher, log *logger.Logger) *RedisEventPublisher {
	return &RedisEventPublisher{
		pub:     pub,
		channel: defaultChannel,
		log:     log,
	}
}

// Handle сериализует событие и публикует его в Redis.
func (p *RedisEventPublisher) Handle(ctx context.Context, event any) error {
	typeName := reflect.TypeOf(event).Name()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("redis publisher: marshal %s: %w", typeName, err)
	}

	env := envelope{Type: typeName, Data: data}
	payload, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("redis publisher: marshal envelope: %w", err)
	}

	if err := p.pub.Publish(ctx, p.channel, payload); err != nil {
		return fmt.Errorf("redis publisher: publish to %s: %w", p.channel, err)
	}

	p.log.Debug("Event published to Redis",
		zap.String("type", typeName),
		zap.String("channel", p.channel),
	)
	return nil
}

// RedisEventSubscriber слушает Redis Pub/Sub канал и перепубликовывает
// полученные события в локальную шину (обычно в процессе API).
type RedisEventSubscriber struct {
	sub     redisSubscriber
	bus     Bus
	channel string
	log     *logger.Logger
	stopCh  chan struct{}
}

// NewRedisEventSubscriber создаёт подписчика, слушающего Redis и передающего события в локальную шину.
func NewRedisEventSubscriber(sub redisSubscriber, bus Bus, log *logger.Logger) *RedisEventSubscriber {
	return &RedisEventSubscriber{
		sub:     sub,
		bus:     bus,
		channel: defaultChannel,
		log:     log,
		stopCh:  make(chan struct{}),
	}
}

// Start начинает слушать события в Redis-канале.
// Блокируется до вызова Stop или отмены контекста; вызывайте в goroutine.
func (s *RedisEventSubscriber) Start(ctx context.Context) {
	pubsub := s.sub.Subscribe(ctx, s.channel)
	ch := pubsub.Channel()

	s.log.Info("Redis event subscriber started",
		zap.String("channel", s.channel),
	)

	for {
		select {
		case <-ctx.Done():
			_ = pubsub.Close()
			s.log.Info("Redis event subscriber stopped")
			return
		case <-s.stopCh:
			_ = pubsub.Close()
			s.log.Info("Redis event subscriber stopped")
			return
		case msg, ok := <-ch:
			if !ok {
				s.log.Warn("Redis event subscriber channel closed")
				return
			}
			s.handleMessage(ctx, msg)
		}
	}
}

// Stop сигнализирует подписчику остановиться, заставляя Start вернуться.
func (s *RedisEventSubscriber) Stop() {
	select {
	case <-s.stopCh:
		// Уже остановлен.
	default:
		close(s.stopCh)
	}
}

func (s *RedisEventSubscriber) handleMessage(ctx context.Context, msg *redis.Message) {
	var env envelope
	if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
		s.log.Error("Redis event subscriber: unmarshal envelope",
			zap.Error(err),
			zap.String("payload", msg.Payload),
		)
		return
	}

	typ, ok := eventTypeRegistry[env.Type]
	if !ok {
		s.log.Warn("Redis event subscriber: unknown event type",
			zap.String("type", env.Type),
		)
		return
	}

	eventPtr := reflect.New(typ).Interface()
	if err := json.Unmarshal(env.Data, eventPtr); err != nil {
		s.log.Error("Redis event subscriber: unmarshal event data",
			zap.Error(err),
			zap.String("type", env.Type),
		)
		return
	}

	event := reflect.ValueOf(eventPtr).Elem().Interface()
	s.bus.Publish(ctx, event)

	s.log.Debug("Event received from Redis and re-published",
		zap.String("type", env.Type),
		zap.String("channel", s.channel),
	)
}
