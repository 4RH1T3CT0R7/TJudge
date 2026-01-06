package websocket

import "github.com/google/uuid"

// NoopBroadcaster пустая реализация Broadcaster (для тестов или когда WS отключен)
type NoopBroadcaster struct{}

// NewNoopBroadcaster создаёт новый NoopBroadcaster
func NewNoopBroadcaster() *NoopBroadcaster {
	return &NoopBroadcaster{}
}

// Broadcast ничего не делает
func (n *NoopBroadcaster) Broadcast(tournamentID uuid.UUID, messageType MessageType, payload interface{}) {
	// No-op
}
