package ws

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// hub держит вебсокет-подключения, разложенные по турнирам
type Hub struct {
	// клиенты по турнирам
	tournaments map[uuid.UUID]map[*Client]bool

	register   chan *Client
	unregister chan *Client
	broadcast  chan *Message

	// защищает tournaments
	mu sync.RWMutex

	log *logger.Logger
}

// Message - вебсокет сообщение
type Message struct {
	TournamentID uuid.UUID   `json:"tournament_id"`
	Type         MessageType `json:"type"`
	Payload      any         `json:"payload"`
}

// MessageType тип сообщения
type MessageType string

const (
	MessageTypeTournamentUpdate  MessageType = "tournament_update"
	MessageTypeMatchUpdate       MessageType = "match_update"
	MessageTypeLeaderboardUpdate MessageType = "leaderboard_update"
	MessageTypeError             MessageType = "error"
	MessageTypePing              MessageType = "ping"
	MessageTypePong              MessageType = "pong"
)

// NewHub создаёт новый вебсокет hub.
// буферы каналов подобраны на глаз, вроде хватает
// TODO: вынести размеры буферов в конфиг?
func NewHub(log *logger.Logger) *Hub {
	return &Hub{
		tournaments: make(map[uuid.UUID]map[*Client]bool),
		register:    make(chan *Client, 64),
		unregister:  make(chan *Client, 64),
		broadcast:   make(chan *Message, 256),
		log:         log,
	}
}

// Run крутит hub в отдельной горутине - только она трогает tournaments map
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

func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// если клиент уже закрыт (unregister прилетел раньше register из-за буферов),
	// мёртвый клиент в map не добавляется
	if client.IsClosed() {
		h.log.Info("Client already closed, skipping registration",
			zap.String("tournament_id", client.tournamentID.String()),
			zap.String("user_id", client.userID.String()),
		)
		return
	}

	if h.tournaments[client.tournamentID] == nil {
		h.tournaments[client.tournamentID] = make(map[*Client]bool)
	}

	h.tournaments[client.tournamentID][client] = true

	h.log.Info("Client registered",
		zap.String("tournament_id", client.tournamentID.String()),
		zap.String("user_id", client.userID.String()),
	)
}

func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.tournaments[client.tournamentID]; ok {
		if _, exists := clients[client]; exists {
			delete(clients, client)

			// пустая map турнира выкидывается
			if len(clients) == 0 {
				delete(h.tournaments, client.tournamentID)
			}

			h.log.Info("Client unregistered",
				zap.String("tournament_id", client.tournamentID.String()),
				zap.String("user_id", client.userID.String()),
			)
		}
	}

	// клиент всегда закрывается, даже если его ещё не было в map (register висит
	// в буфере) - тогда registerClient увидит закрытого и пропустит.
	// CloseSend идемпотентен через sync.Once
	client.CloseSend()
}

func (h *Hub) broadcastMessage(message *Message) {
	h.mu.Lock()
	defer h.mu.Unlock()

	clients, ok := h.tournaments[message.TournamentID]
	if !ok {
		return
	}

	// маршалинг один раз
	data, err := json.Marshal(message)
	if err != nil {
		h.log.LogError("Failed to marshal message", err)
		return
	}

	for client := range clients {
		// уже закрытые пропускаются (защита от буферизованных register/unregister)
		if client.IsClosed() {
			delete(clients, client)
			continue
		}
		select {
		case client.send <- data:
		default:
			// буфер забит - клиент рубится, ждать смысла нет
			h.log.Info("Client send buffer full, disconnecting",
				zap.String("tournament_id", client.tournamentID.String()),
				zap.String("user_id", client.userID.String()),
			)
			client.CloseSend()
			delete(clients, client)
		}
	}

	h.log.Debug("Broadcast message sent",
		zap.String("tournament_id", message.TournamentID.String()),
		zap.String("type", string(message.Type)),
		zap.Int("clients", len(clients)),
	)
}

// Broadcast кладёт сообщение в канал рассылки.
// FIXME: если клиентов на турнир соберётся под сотни, рассылка под mu начнёт подтормаживать
func (h *Hub) Broadcast(tournamentID uuid.UUID, messageType string, payload any) {
	message := &Message{
		TournamentID: tournamentID,
		Type:         MessageType(messageType),
		Payload:      payload,
	}

	select {
	case h.broadcast <- message:
	default:
		// канал полон, ещё попытка с таймаутом, потом дроп
		timer := time.NewTimer(time.Second)
		defer timer.Stop()
		select {
		case h.broadcast <- message:
		case <-timer.C:
			h.log.Error("Broadcast channel full, message dropped after 1s timeout",
				zap.String("tournament_id", tournamentID.String()),
				zap.String("type", messageType),
			)
		}
	}
}

func (h *Hub) shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()

	// закрытие всех подключений (idempotent через sync.Once)
	for tournamentID, clients := range h.tournaments {
		for client := range clients {
			client.CloseSend()
			delete(clients, client)
		}
		delete(h.tournaments, tournamentID)
	}

	h.log.Info("WebSocket hub shutdown complete")
}

// GetStats отдаёт статистику hub
func (h *Hub) GetStats() map[string]any {
	h.mu.RLock()
	defer h.mu.RUnlock()

	totalClients := 0
	for _, clients := range h.tournaments {
		totalClients += len(clients)
	}

	return map[string]any{
		"tournaments":   len(h.tournaments),
		"total_clients": totalClients,
	}
}
