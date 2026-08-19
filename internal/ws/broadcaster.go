package ws

import "github.com/google/uuid"

// Broadcaster - кто умеет рассылать обновления клиентам турнира
type Broadcaster interface {
	Broadcast(tournamentID uuid.UUID, messageType string, payload any)
}
