package websocket

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

const (
	// Время ожидания записи в WebSocket
	writeWait = 10 * time.Second

	// Время ожидания pong от клиента (35s обеспечивает быстрое
	// обнаружение зависших соединений).
	pongWait = 35 * time.Second

	// Интервал отправки ping клиенту (30s, достаточно агрессивно,
	// чтобы обнаружить disconnect за pongWait после сетевого сбоя).
	pingPeriod = 30 * time.Second

	// Максимальный размер сообщения от клиента
	maxMessageSize = 512

	// Per-client rate limit на входящие сообщения.
	// 10 msg/sec с burst=20 достаточно для ping-pong и UI-событий,
	// блокирует flood из скомпрометированного/злонамеренного клиента.
	clientMessageRate  = 10
	clientMessageBurst = 20

	// closePolicyViolation - код close-frame по RFC 6455 §7.4 (1008).
	closePolicyViolation = 1008
)

// Client представляет WebSocket клиента
type Client struct {
	hub          *Hub
	conn         *websocket.Conn
	send         chan []byte
	tournamentID uuid.UUID
	userID       uuid.UUID
	log          *logger.Logger

	// closed - атомарный флаг, отражающий закрытие send-канала.
	// Читается без mutex из sendPong/WritePump чтобы избежать write-on-closed.
	// Писаться может только через CloseSend() (sync.Once гарантирует идемпотентность).
	closed    atomic.Bool
	closeOnce sync.Once

	// readLimiter - per-client token bucket для входящих сообщений.
	// Защищает от message flooding со стороны клиента.
	readLimiter *rate.Limiter
}

// IsClosed возвращает true если send-канал клиента уже закрыт.
// Безопасно для concurrent-чтения.
func (c *Client) IsClosed() bool {
	return c.closed.Load()
}

// CloseSend идемпотентно закрывает send-канал клиента. Безопасно вызывать
// из любого goroutine и многократно - sync.Once гарантирует ровно одно close().
// Это закрывает race, при котором прежний bool-флаг мог быть гонкой между
// unregisterClient/broadcastMessage/shutdown и приводить к panic "close of closed channel".
func (c *Client) CloseSend() {
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		close(c.send)
	})
}

// NewClient создаёт нового WebSocket клиента
func NewClient(hub *Hub, conn *websocket.Conn, tournamentID, userID uuid.UUID, log *logger.Logger) *Client {
	return &Client{
		hub:          hub,
		conn:         conn,
		send:         make(chan []byte, 256),
		tournamentID: tournamentID,
		userID:       userID,
		log:          log,
		// Token bucket для rate-limit входящих сообщений.
		readLimiter: rate.NewLimiter(rate.Limit(clientMessageRate), clientMessageBurst),
	}
}

// Register регистрирует клиента в hub
func (c *Client) Register() {
	c.hub.register <- c
}

// ReadPump читает сообщения от клиента
func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()

	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.log.LogError("WebSocket read error", err,
					zap.String("tournament_id", c.tournamentID.String()),
					zap.String("user_id", c.userID.String()),
				)
			}
			break
		}

		// Rate-limit per-client. При превышении закрываем с кодом 1008 (policy violation).
		if !c.readLimiter.Allow() {
			c.log.Info("WebSocket client exceeded message rate limit, disconnecting",
				zap.String("tournament_id", c.tournamentID.String()),
				zap.String("user_id", c.userID.String()),
			)
			_ = c.conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(closePolicyViolation, "rate limit exceeded"),
				time.Now().Add(writeWait),
			)
			break
		}

		// Обрабатываем входящее сообщение
		c.handleMessage(message)
	}
}

// WritePump отправляет сообщения клиенту
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub закрыл канал
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

			// Отправляем queued сообщения как отдельные WebSocket фреймы
			n := len(c.send)
			for i := 0; i < n; i++ {
				queued, ok := <-c.send
				if !ok {
					// Hub закрыл канал во время drain
					return
				}
				if err := c.conn.WriteMessage(websocket.TextMessage, queued); err != nil {
					return
				}
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage обрабатывает входящее сообщение от клиента
func (c *Client) handleMessage(data []byte) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		c.log.Info("Invalid message format",
			zap.Error(err),
			zap.String("user_id", c.userID.String()),
		)
		return
	}

	// Обрабатываем разные типы сообщений
	switch msg.Type {
	case MessageTypePing:
		// Отправляем pong
		c.sendPong()

	default:
		c.log.Info("Unknown message type",
			zap.String("type", string(msg.Type)),
			zap.String("user_id", c.userID.String()),
		)
	}
}

// sendPong отправляет pong сообщение клиенту
func (c *Client) sendPong() {
	message := &Message{
		TournamentID: c.tournamentID,
		Type:         MessageTypePong,
		Payload:      map[string]string{"status": "ok"},
	}

	data, err := json.Marshal(message)
	if err != nil {
		c.log.LogError("Failed to marshal pong", err)
		return
	}

	// Fast-path: skip send если канал уже закрыт другой горутиной.
	// Атомарный флаг избавляет от recover()-шаблона.
	if c.IsClosed() {
		return
	}

	// Defensive recover на случай гонки между IsClosed() и select.
	// sync.Once делает такой race крайне маловероятным, но panic-safe сохраняем.
	defer func() {
		if r := recover(); r != nil {
			c.log.Info("sendPong: channel closed, client disconnecting")
		}
	}()

	select {
	case c.send <- data:
	default:
		c.log.Info("Client send buffer full")
	}
}
