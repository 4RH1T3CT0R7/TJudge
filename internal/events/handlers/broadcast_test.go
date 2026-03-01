package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mock Broadcaster ---

type mockBroadcaster struct {
	mock.Mock
}

func (m *mockBroadcaster) Broadcast(tournamentID uuid.UUID, messageType string, payload interface{}) {
	m.Called(tournamentID, messageType, payload)
}

// --- BroadcastHandler tests ---

func TestBroadcastHandler_TournamentStarted(t *testing.T) {
	bc := &mockBroadcaster{}
	h := NewBroadcastHandler(bc, newTestLogger(t))

	id := uuid.New()
	now := time.Now()
	bc.On("Broadcast", id, "tournament_update", mock.MatchedBy(func(p interface{}) bool {
		m, ok := p.(map[string]interface{})
		return ok && m["status"] == domain.TournamentActive
	})).Return()

	err := h.Handle(context.Background(), events.TournamentStarted{
		TournamentID: id,
		Status:       domain.TournamentActive,
		StartTime:    &now,
	})
	assert.NoError(t, err)
	bc.AssertExpectations(t)
}

func TestBroadcastHandler_TournamentCompleted(t *testing.T) {
	bc := &mockBroadcaster{}
	h := NewBroadcastHandler(bc, newTestLogger(t))

	id := uuid.New()
	now := time.Now()
	bc.On("Broadcast", id, "tournament_update", mock.MatchedBy(func(p interface{}) bool {
		m, ok := p.(map[string]interface{})
		return ok && m["status"] == domain.TournamentCompleted
	})).Return()

	err := h.Handle(context.Background(), events.TournamentCompleted{
		TournamentID: id,
		Status:       domain.TournamentCompleted,
		EndTime:      &now,
	})
	assert.NoError(t, err)
	bc.AssertExpectations(t)
}

func TestBroadcastHandler_MatchesCreated(t *testing.T) {
	bc := &mockBroadcaster{}
	h := NewBroadcastHandler(bc, newTestLogger(t))

	id := uuid.New()
	pid := uuid.New()
	bc.On("Broadcast", id, "matches_created", mock.MatchedBy(func(p interface{}) bool {
		m, ok := p.(map[string]interface{})
		return ok && m["matches_count"] == 10
	})).Return()

	err := h.Handle(context.Background(), events.MatchesCreated{
		TournamentID: id,
		ProgramID:    pid,
		MatchCount:   10,
	})
	assert.NoError(t, err)
	bc.AssertExpectations(t)
}

func TestBroadcastHandler_MatchResultProcessed(t *testing.T) {
	bc := &mockBroadcaster{}
	h := NewBroadcastHandler(bc, newTestLogger(t))

	id := uuid.New()
	mid := uuid.New()
	bc.On("Broadcast", id, "match_result", mock.MatchedBy(func(p interface{}) bool {
		m, ok := p.(map[string]interface{})
		return ok && m["winner"] == 1 && m["new_rating1"] == 1520
	})).Return()

	err := h.Handle(context.Background(), events.MatchResultProcessed{
		TournamentID: id,
		MatchID:      mid,
		Program1ID:   uuid.New(),
		Program2ID:   uuid.New(),
		NewRating1:   1520,
		NewRating2:   1480,
		Winner:       1,
	})
	assert.NoError(t, err)
	bc.AssertExpectations(t)
}

func TestBroadcastHandler_UnexpectedEvent(t *testing.T) {
	bc := &mockBroadcaster{}
	h := NewBroadcastHandler(bc, newTestLogger(t))

	err := h.Handle(context.Background(), "not an event")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected event type")
}
