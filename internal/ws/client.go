package ws

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
	// сколько ждать запись в сокет
	writeWait = 10 * time.Second

	// ожидание pong от клиента, 35s - чтобы быстро ловить мёртвые коннекты
	pongWait = 35 * time.Second

	// как часто шлётся пинг (30s, достаточно чтобы заметить отвал за pongWait)
	pingPeriod = 30 * time.Second

	// макс размер входящего сообщения
	maxMessageSize = 512

	// per-client рейт-лимит на входящие. 10/сек с burst 20 - хватает на ping-pong
	// и ui-события, а флуд от скомпрометированного клиента режет
	clientMessageRate  = 10
	clientMessageBurst = 20

	// closePolicyViolation - код close-frame по RFC 6455 §7.4 (1008)
	closePolicyViolation = 1008
)

// Client - вебсокет клиент
type Client struct {
	hub          *Hub
	conn         *websocket.Conn
	send         chan []byte
	tournamentID uuid.UUID
	userID       uuid.UUID
	log          *logger.Logger

	// closed - атомарный флаг что send-канал закрыт. читается без мьютекса из
	// sendPong/WritePump чтобы не писать в закрытый канал. писать только через
	// CloseSend (sync.Once даёт идемпотентность)
	closed    atomic.Bool
	closeOnce sync.Once

	// per-client ведро токенов, защита от флуда входящими
	readLimiter *rate.Limiter
}

// IsClosed - закрыт ли уже send-канал. безопасно читать из разных гроутин
func (c *Client) IsClosed() bool {
	return c.closed.Load()
}

// CloseSend закрывает send-канал, идемпотентно. можно звать откуда угодно и сколько
// угодно раз - sync.Once сделает close ровно один. без этого ловили гонку
// "close of closed channel" между unregisterClient/broadcastMessage/shutdown
func (c *Client) CloseSend() {
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		close(c.send)
	})
}

// NewClient создаёт нового вебсокет клиента
func NewClient(hub *Hub, conn *websocket.Conn, tournamentID, userID uuid.UUID, log *logger.Logger) *Client {
	return &Client{
		hub:          hub,
		conn:         conn,
		send:         make(chan []byte, 256),
		tournamentID: tournamentID,
		userID:       userID,
		log:          log,
		// ведро токенов под рейт-лимит входящих
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

		// рейт-лимит per-client. превысил - закрытие с кодом 1008 (policy violation)
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

		c.handleMessage(message)
	}
}

// WritePump шлёт сообщения клиенту
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
				// hub закрыл канал
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

			// досылается что накопилось в канале отдельными фреймами
			n := len(c.send)
			for range n {
				queued, ok := <-c.send
				if !ok {
					// hub закрыл канал пока сливали
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

func (c *Client) handleMessage(data []byte) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		c.log.Info("Invalid message format",
			zap.Error(err),
			zap.String("user_id", c.userID.String()),
		)
		return
	}

	// пока только ping, остальное игнорится
	switch msg.Type {
	case MessageTypePing:
		c.sendPong()

	default:
		c.log.Info("Unknown message type",
			zap.String("type", string(msg.Type)),
			zap.String("user_id", c.userID.String()),
		)
	}
}

// sendPong шлёт клиенту pong
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

	// fast-path: если канал уже закрыт другой горутиной - лучше не соваться
	if c.IsClosed() {
		return
	}

	// на всякий случай recover - вдруг гонка между IsClosed() и select.
	// sync.Once делает это почти невероятным, но пусть будет panic-safe
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
