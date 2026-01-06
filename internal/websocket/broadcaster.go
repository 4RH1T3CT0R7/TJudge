package websocket

import "github.com/google/uuid"

// Broadcaster интерфейс для broadcast обновлений
type Broadcaster interface {
	Broadcast(tournamentID uuid.UUID, messageType string, payload interface{})
}
