package events

import (
	"context"
	"encoding/json"
	"fmt"

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

// RedisEventPublisher отправляет события в Redis Pub/Sub канал, чтобы события одного
// процесса (воркер или реплика API) доходили до всех реплик API, подписанных на канал.
type RedisEventPublisher struct {
	pub     redisPublisher
	channel string
	log     *logger.Logger
}

// NewRedisEventPublisher создаёт publisher, пишущий в общий канал
func NewRedisEventPublisher(pub redisPublisher, log *logger.Logger) *RedisEventPublisher {
	return &RedisEventPublisher{
		pub:     pub,
		channel: defaultChannel,
		log:     log,
	}
}

// Publish сериализует событие и кладёт его в редис. typeName - имя типа, оно уходит
// в envelope и по нему подписчик на той стороне разберёт что пришло.
func (p *RedisEventPublisher) Publish(ctx context.Context, typeName string, event any) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("redis publisher: marshal %s: %w", typeName, err)
	}

	env := envelope{Type: typeName, Data: data}
	payload, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("redis publisher: marshal envelope: %w", err)
	}

	// событие шлётся после записи в БД, так что отмена запроса или остановка
	// воркера не должна его терять; от зависания страхуют таймауты клиента редиса
	if err := p.pub.Publish(context.WithoutCancel(ctx), p.channel, payload); err != nil {
		return fmt.Errorf("redis publisher: publish to %s: %w", p.channel, err)
	}

	p.log.Debug("Event published to Redis",
		zap.String("type", typeName),
		zap.String("channel", p.channel),
	)
	return nil
}

// RedisEventSubscriber слушает Redis-канал и отдаёт полученные события локальному
// нотифаеру (в процессе API - там он только рассылает по вебсокету).
type RedisEventSubscriber struct {
	sub      redisSubscriber
	notifier Notifier
	channel  string
	log      *logger.Logger
	stopCh   chan struct{}
}

// NewRedisEventSubscriber создаёт подписчика, слушающего Redis.
func NewRedisEventSubscriber(sub redisSubscriber, notifier Notifier, log *logger.Logger) *RedisEventSubscriber {
	return &RedisEventSubscriber{
		sub:      sub,
		notifier: notifier,
		channel:  defaultChannel,
		log:      log,
		stopCh:   make(chan struct{}),
	}
}

// Start слушает события в Redis-канале.
// блокируется до Stop или отмены контекста, так что зовите в горутине
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

// Stop велит подписчику остановиться - Start после этого выйдет
func (s *RedisEventSubscriber) Stop() {
	select {
	case <-s.stopCh:
		// уже остановлен
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

	// имена типов обязаны совпадать с тем что кладёт publisher
	var err error
	switch env.Type {
	case "MatchResultProcessed":
		err = dispatch(ctx, env.Data, s.notifier.MatchResultProcessed)
	case "ProgramCompiled":
		err = dispatch(ctx, env.Data, s.notifier.ProgramCompiled)
	case "TournamentStarted":
		err = dispatch(ctx, env.Data, s.notifier.TournamentStarted)
	case "TournamentCompleted":
		err = dispatch(ctx, env.Data, s.notifier.TournamentCompleted)
	default:
		s.log.Warn("Redis event subscriber: unknown event type",
			zap.String("type", env.Type),
		)
		return
	}
	if err != nil {
		s.log.Error("Redis event subscriber: unmarshal event data",
			zap.Error(err),
			zap.String("type", env.Type),
		)
		return
	}

	s.log.Debug("Event received from Redis and re-published",
		zap.String("type", env.Type),
		zap.String("channel", s.channel),
	)
}

// dispatch разбирает data в событие типа T и отдаёт его обработчику
func dispatch[T any](ctx context.Context, data json.RawMessage, handle func(context.Context, T)) error {
	var e T
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	handle(ctx, e)
	return nil
}
