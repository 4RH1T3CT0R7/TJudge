package websocket

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Hub управляет WebSocket подключениями
type Hub struct {
	// Клиенты по турнирам
	tournaments map[uuid.UUID]map[*Client]bool

	// Канал для регистрации клиентов
	register chan *Client

	// Канал для отмены регистрации клиентов
	unregister chan *Client

	// Канал для broadcast сообщений
	broadcast chan *Message

	// Mutex для защиты tournaments map
	mu sync.RWMutex

	log *logger.Logger
}

// Message представляет WebSocket сообщение
type Message struct {
	TournamentID uuid.UUID   `json:"tournament_id"`
	Type         MessageType `json:"type"`
	Payload      interface{} `json:"payload"`
}

// MessageType тип сообщения
type MessageType string

const (
	// MessageTypeTournamentUpdate обновление турнира
	MessageTypeTournamentUpdate MessageType = "tournament_update"
	// MessageTypeMatchUpdate обновление матча
	MessageTypeMatchUpdate MessageType = "match_update"
	// MessageTypeLeaderboardUpdate обновление таблицы лидеров
	MessageTypeLeaderboardUpdate MessageType = "leaderboard_update"
	// MessageTypeError ошибка
	MessageTypeError MessageType = "error"
	// MessageTypePing ping
	MessageTypePing MessageType = "ping"
	// MessageTypePong pong
	MessageTypePong MessageType = "pong"
)

// NewHub создаёт новый WebSocket hub
func NewHub(log *logger.Logger) *Hub {
	return &Hub{
		tournaments: make(map[uuid.UUID]map[*Client]bool),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		broadcast:   make(chan *Message, 256),
		log:         log,
	}
}

// Run запускает hub в отдельной горутине
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h.log.Info("WebSocket hub shutting down")
			h.shutdown()
			return

		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.broadcastMessage(message)
		}
	}
}

// registerClient регистрирует клиента
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.tournaments[client.tournamentID] == nil {
		h.tournaments[client.tournamentID] = make(map[*Client]bool)
	}

	h.tournaments[client.tournamentID][client] = true

	h.log.Info("Client registered",
		zap.String("tournament_id", client.tournamentID.String()),
		zap.String("user_id", client.userID.String()),
	)
}

// unregisterClient отменяет регистрацию клиента
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.tournaments[client.tournamentID]; ok {
		if _, exists := clients[client]; exists {
			delete(clients, client)
			close(client.send)

			// Удаляем пустую map турнира
			if len(clients) == 0 {
				delete(h.tournaments, client.tournamentID)
			}

			h.log.Info("Client unregistered",
				zap.String("tournament_id", client.tournamentID.String()),
				zap.String("user_id", client.userID.String()),
			)
		}
	}
}

// broadcastMessage отправляет сообщение всем клиентам турнира
func (h *Hub) broadcastMessage(message *Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients, ok := h.tournaments[message.TournamentID]
	if !ok {
		return
	}

	// Сериализуем сообщение один раз
	data, err := json.Marshal(message)
	if err != nil {
		h.log.LogError("Failed to marshal message", err)
		return
	}

	// Отправляем всем клиентам
	for client := range clients {
		select {
		case client.send <- data:
		default:
			// Канал заблокирован, отключаем клиента
			h.log.Info("Client send buffer full, disconnecting",
				zap.String("tournament_id", client.tournamentID.String()),
				zap.String("user_id", client.userID.String()),
			)
			close(client.send)
			delete(clients, client)
		}
	}

	h.log.Debug("Broadcast message sent",
		zap.String("tournament_id", message.TournamentID.String()),
		zap.String("type", string(message.Type)),
		zap.Int("clients", len(clients)),
	)
}

// Broadcast отправляет сообщение в канал broadcast
func (h *Hub) Broadcast(tournamentID uuid.UUID, messageType string, payload interface{}) {
	message := &Message{
		TournamentID: tournamentID,
		Type:         MessageType(messageType),
		Payload:      payload,
	}

	select {
	case h.broadcast <- message:
	default:
		h.log.Error("Broadcast channel full, message dropped",
			zap.String("tournament_id", tournamentID.String()),
			zap.String("type", messageType),
		)
	}
}

// shutdown корректно завершает работу hub
func (h *Hub) shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Закрываем все подключения
	for tournamentID, clients := range h.tournaments {
		for client := range clients {
			close(client.send)
			delete(clients, client)
		}
		delete(h.tournaments, tournamentID)
	}

	h.log.Info("WebSocket hub shutdown complete")
}

// GetStats возвращает статистику hub
func (h *Hub) GetStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	totalClients := 0
	for _, clients := range h.tournaments {
		totalClients += len(clients)
	}

	return map[string]interface{}{
		"tournaments":   len(h.tournaments),
		"total_clients": totalClients,
	}
}
