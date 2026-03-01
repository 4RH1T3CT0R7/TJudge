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

// envelope wraps an event with its type name for JSON serialization.
type envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// RedisPublisher is a cache.Cache-compatible interface for publishing.
type redisPublisher interface {
	Publish(ctx context.Context, channel string, message interface{}) error
}

// redisSubscriber is a cache.Cache-compatible interface for subscribing.
type redisSubscriber interface {
	Subscribe(ctx context.Context, channels ...string) *redis.PubSub
}

// eventTypeRegistry maps type name → reflect.Type for deserialization.
var eventTypeRegistry = map[string]reflect.Type{}

func init() {
	// Register all event types that can cross the Redis bridge.
	registerType(MatchResultProcessed{})
	registerType(TournamentStarted{})
	registerType(TournamentCompleted{})
	registerType(MatchesCreated{})
}

func registerType(v any) {
	t := reflect.TypeOf(v)
	eventTypeRegistry[t.Name()] = t
}

// RedisEventPublisher is an event Handler that forwards events to a Redis Pub/Sub channel.
// Attach it to a SyncBus so that events emitted in one process (e.g. worker) are forwarded
// to other processes (e.g. API) that subscribe to the same Redis channel.
type RedisEventPublisher struct {
	pub     redisPublisher
	channel string
	log     *logger.Logger
}

// NewRedisEventPublisher creates a publisher that sends events to the given Redis channel.
func NewRedisEventPublisher(pub redisPublisher, log *logger.Logger) *RedisEventPublisher {
	return &RedisEventPublisher{
		pub:     pub,
		channel: defaultChannel,
		log:     log,
	}
}

// Handle serializes the event and publishes it to Redis.
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

// RedisEventSubscriber listens on a Redis Pub/Sub channel and re-publishes
// received events to a local event Bus (typically in the API process).
type RedisEventSubscriber struct {
	sub     redisSubscriber
	bus     Bus
	channel string
	log     *logger.Logger
	stopCh  chan struct{}
}

// NewRedisEventSubscriber creates a subscriber that listens on Redis and feeds events to the local bus.
func NewRedisEventSubscriber(sub redisSubscriber, bus Bus, log *logger.Logger) *RedisEventSubscriber {
	return &RedisEventSubscriber{
		sub:     sub,
		bus:     bus,
		channel: defaultChannel,
		log:     log,
		stopCh:  make(chan struct{}),
	}
}

// Start begins listening for events on the Redis channel.
// It blocks until Stop is called or the context is cancelled; call it in a goroutine.
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

// Stop signals the subscriber to stop, causing Start to return.
func (s *RedisEventSubscriber) Stop() {
	select {
	case <-s.stopCh:
		// Already stopped.
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
