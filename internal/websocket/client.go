package websocket

import (
	"encoding/json"
	"time"

	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	// Время ожидания записи в WebSocket
	writeWait = 10 * time.Second

	// Время ожидания pong от клиента
	pongWait = 60 * time.Second

	// Интервал отправки ping клиенту
	pingPeriod = (pongWait * 9) / 10

	// Максимальный размер сообщения от клиента
	maxMessageSize = 512
)

// Client представляет WebSocket клиента
type Client struct {
	hub          *Hub
	conn         *websocket.Conn
	send         chan []byte
	tournamentID uuid.UUID
	userID       uuid.UUID
	log          *logger.Logger
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

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)

			// Добавляем queued сообщения в текущий фрейм
			n := len(c.send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte{'\n'})
				_, _ = w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
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

	select {
	case c.send <- data:
	default:
		c.log.Info("Client send buffer full")
	}
}
