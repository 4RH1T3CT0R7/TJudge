package handlers

import (
	"context"
	"fmt"

	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
)

// Broadcaster отправляет real-time сообщения WebSocket-клиентам.
type Broadcaster interface {
	Broadcast(tournamentID uuid.UUID, messageType string, payload any)
}

// BroadcastHandler отправляет WebSocket-уведомления в ответ на доменные события.
type BroadcastHandler struct {
	broadcaster Broadcaster
	log         *logger.Logger
}

// NewBroadcastHandler создаёт handler, рассылающий WebSocket-сообщения.
func NewBroadcastHandler(broadcaster Broadcaster, log *logger.Logger) *BroadcastHandler {
	return &BroadcastHandler{broadcaster: broadcaster, log: log}
}

func (h *BroadcastHandler) Handle(_ context.Context, event any) error {
	switch e := event.(type) {
	case events.TournamentStarted:
		h.broadcaster.Broadcast(e.TournamentID, "tournament_update", map[string]any{
			"status":     e.Status,
			"start_time": e.StartTime,
		})

	case events.TournamentCompleted:
		h.broadcaster.Broadcast(e.TournamentID, "tournament_update", map[string]any{
			"status":   e.Status,
			"end_time": e.EndTime,
		})

	case events.MatchesCreated:
		h.broadcaster.Broadcast(e.TournamentID, "matches_created", map[string]any{
			"program_id":    e.ProgramID.String(),
			"matches_count": e.MatchCount,
		})

	case events.ProgramCompiled:
		h.broadcaster.Broadcast(e.TournamentID, "program_update", map[string]any{
			"program_id":    e.ProgramID.String(),
			"team_id":       e.TeamID.String(),
			"status":        e.Status,
			"error_message": e.ErrorMessage,
		})

	case events.MatchResultProcessed:
		h.broadcaster.Broadcast(e.TournamentID, "match_result", map[string]any{
			"match_id":    e.MatchID.String(),
			"program1_id": e.Program1ID.String(),
			"program2_id": e.Program2ID.String(),
			"new_rating1": e.NewRating1,
			"new_rating2": e.NewRating2,
			"winner":      e.Winner,
		})

	default:
		return fmt.Errorf("BroadcastHandler: unexpected event type %T", event)
	}

	return nil
}
