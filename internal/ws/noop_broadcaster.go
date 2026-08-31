package ws

import "github.com/google/uuid"

// NoopBroadcaster - пустой broadcaster, для тестов или когда вебсокет выключен
type NoopBroadcaster struct{}

func NewNoopBroadcaster() *NoopBroadcaster {
	return &NoopBroadcaster{}
}

func (n *NoopBroadcaster) Broadcast(tournamentID uuid.UUID, messageType string, payload any) {
	// ничего не делает
}
