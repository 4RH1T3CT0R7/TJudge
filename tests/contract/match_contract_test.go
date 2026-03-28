//go:build contract

package contract

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/db"
	"github.com/bmstu-itstech/tjudge/internal/infrastructure/queue"
	"github.com/bmstu-itstech/tjudge/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestContract_Match_List_200_Public verifies that the match list endpoint is
// accessible without authentication (OptionalAuth) and returns a 200 with the
// standard envelope.
func TestContract_Match_List_200_Public(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	tournamentID := uuid.New()
	matches := []*domain.Match{
		{
			ID:           uuid.New(),
			TournamentID: tournamentID,
			Program1ID:   uuid.New(),
			Program2ID:   uuid.New(),
			GameType:     "prisoners_dilemma",
			Status:       domain.MatchCompleted,
			Priority:     domain.PriorityMedium,
			RoundNumber:  1,
			CreatedAt:    time.Now().UTC(),
		},
	}

	h.MatchRepo.EXPECT().
		List(mock.Anything, mock.AnythingOfType("domain.MatchFilter")).
		Return(matches, nil).
		Once()

	resp := h.GET("/api/v1/matches").Do()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	body := ReadBody(t, resp)
	AssertEnvelope(t, body)
}

// TestContract_Match_List_200_WithAuth verifies that the match list endpoint
// works when a user is authenticated via OptionalAuth. The middleware injects
// user identity into the context, but the endpoint remains public.
func TestContract_Match_List_200_WithAuth(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	matches := []*domain.Match{
		{
			ID:           uuid.New(),
			TournamentID: uuid.New(),
			Program1ID:   uuid.New(),
			Program2ID:   uuid.New(),
			GameType:     "tug_of_war",
			Status:       domain.MatchPending,
			Priority:     domain.PriorityHigh,
			RoundNumber:  1,
			CreatedAt:    time.Now().UTC(),
		},
	}

	h.MatchRepo.EXPECT().
		List(mock.Anything, mock.AnythingOfType("domain.MatchFilter")).
		Return(matches, nil).
		Once()

	resp := h.GET("/api/v1/matches").
		WithAuth(h.UserToken()).
		Do()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	body := ReadBody(t, resp)
	AssertEnvelope(t, body)
}

// TestContract_Match_Get_200 verifies that a single match can be fetched by ID
// without authentication.
func TestContract_Match_Get_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	matchID := uuid.New()
	match := &domain.Match{
		ID:           matchID,
		TournamentID: uuid.New(),
		Program1ID:   uuid.New(),
		Program2ID:   uuid.New(),
		GameType:     "travelers_dilemma",
		Status:       domain.MatchCompleted,
		Priority:     domain.PriorityLow,
		RoundNumber:  2,
		CreatedAt:    time.Now().UTC(),
	}

	// Cache miss forces a DB lookup.
	h.MatchCache.EXPECT().
		GetMatch(mock.Anything, matchID).
		Return(nil, fmt.Errorf("cache miss")).
		Once()

	h.MatchRepo.EXPECT().
		GetByID(mock.Anything, matchID).
		Return(match, nil).
		Once()

	resp := h.GET("/api/v1/matches/" + matchID.String()).Do()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	body := ReadBody(t, resp)
	AssertEnvelope(t, body)
}

// TestContract_Match_Get_404 verifies that requesting a non-existent match
// returns 404 with the standard error envelope.
func TestContract_Match_Get_404(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	matchID := uuid.New()

	h.MatchCache.EXPECT().
		GetMatch(mock.Anything, matchID).
		Return(nil, fmt.Errorf("cache miss")).
		Once()

	h.MatchRepo.EXPECT().
		GetByID(mock.Anything, matchID).
		Return(nil, errors.ErrNotFound).
		Once()

	resp := h.GET("/api/v1/matches/" + matchID.String()).Do()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	AssertJSON(t, resp)
	body := ReadBody(t, resp)
	AssertErrorResponse(t, body)
}

// TestContract_Match_GetStatistics_200 verifies that match statistics can be
// retrieved without authentication.
func TestContract_Match_GetStatistics_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	stats := &db.MatchStatistics{
		Total:     42,
		Pending:   5,
		Running:   3,
		Completed: 30,
		Failed:    4,
	}

	// No tournament_id query param => nil pointer for tournamentID.
	h.MatchRepo.EXPECT().
		GetStatistics(mock.Anything, (*uuid.UUID)(nil)).
		Return(stats, nil).
		Once()

	resp := h.GET("/api/v1/matches/statistics").Do()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	body := ReadBody(t, resp)
	AssertEnvelope(t, body)
}

// TestContract_Match_QueueStats_200 verifies that an admin can access queue
// statistics.
func TestContract_Match_QueueStats_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	queueStats := &queue.QueueStats{
		High:   10,
		Medium: 25,
		Low:    5,
		Total:  40,
	}

	h.QueueManager.EXPECT().
		GetStats(mock.Anything).
		Return(queueStats, nil).
		Once()

	resp := h.GET("/api/v1/matches/queue/stats").
		WithAuth(h.AdminToken()).
		Do()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	body := ReadBody(t, resp)
	AssertEnvelope(t, body)
}

// TestContract_Match_QueueStats_403 verifies that a regular user is rejected
// from the admin-only queue stats endpoint.
func TestContract_Match_QueueStats_403(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	resp := h.GET("/api/v1/matches/queue/stats").
		WithAuth(h.UserToken()).
		Do()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	AssertJSON(t, resp)
	body := ReadBody(t, resp)
	AssertErrorResponse(t, body)
}

// TestContract_Match_QueueStats_401 verifies that an unauthenticated request
// to queue stats is rejected with 401.
func TestContract_Match_QueueStats_401(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	resp := h.GET("/api/v1/matches/queue/stats").Do()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	AssertJSON(t, resp)
	body := ReadBody(t, resp)
	AssertErrorResponse(t, body)
}

// TestContract_Match_ClearQueue_200 verifies that an admin can clear all match
// queues and receives a success message.
func TestContract_Match_ClearQueue_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	h.QueueManager.EXPECT().
		Clear(mock.Anything).
		Return(nil).
		Once()

	resp := h.POST("/api/v1/matches/queue/clear").
		WithAuth(h.AdminToken()).
		Do()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	body := ReadBody(t, resp)
	AssertEnvelope(t, body)
}

// TestContract_Match_PurgeInvalid_200 verifies that an admin can purge invalid
// matches from the queue. The validator callback is a closure the handler
// builds internally; the mock accepts any function argument.
func TestContract_Match_PurgeInvalid_200(t *testing.T) {
	t.Parallel()
	h := NewTestHarness(t)

	h.QueueManager.EXPECT().
		PurgeInvalidMatches(mock.Anything, mock.AnythingOfType("func(string) bool")).
		Return(int64(5), nil).
		Once()

	resp := h.POST("/api/v1/matches/queue/purge").
		WithAuth(h.AdminToken()).
		Do()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertJSON(t, resp)
	body := ReadBody(t, resp)
	AssertEnvelope(t, body)
}
