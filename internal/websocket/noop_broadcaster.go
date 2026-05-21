package websocket

import "github.com/google/uuid"

// NoopBroadcaster пустая реализация Broadcaster (для тестов или когда WS отключен)
type NoopBroadcaster struct{}

// Compile-time проверка, что NoopBroadcaster реализует Broadcaster
var _ Broadcaster = (*NoopBroadcaster)(nil)

// NewNoopBroadcaster создаёт новый NoopBroadcaster
func NewNoopBroadcaster() *NoopBroadcaster {
	return &NoopBroadcaster{}
}

// Broadcast ничего не делает
func (n *NoopBroadcaster) Broadcast(tournamentID uuid.UUID, messageType string, payload any) {
	// Ничего не делаем
}
