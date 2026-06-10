package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Стабы ---

type stubRecoveryOutbox struct{ retried int64 }

func (s *stubRecoveryOutbox) RetryErrors(_ context.Context) (int64, error) { return s.retried, nil }

type stubRecoveryPrograms struct{ programs []*domain.Program }

func (s *stubRecoveryPrograms) GetStuckCompiling(_ context.Context, _ time.Duration, _ int) ([]*domain.Program, error) {
	return s.programs, nil
}

type stubRecoveryCompileQueue struct{ enqueued []uuid.UUID }

func (s *stubRecoveryCompileQueue) Enqueue(_ context.Context, id uuid.UUID) error {
	s.enqueued = append(s.enqueued, id)
	return nil
}

type stubRecoveryMatches struct {
	stuck []*domain.Match
	reset []uuid.UUID
}

func (s *stubRecoveryMatches) GetStuckRunning(_ context.Context, _ time.Duration, _ int) ([]*domain.Match, error) {
	return s.stuck, nil
}

func (s *stubRecoveryMatches) ResetToPending(_ context.Context, id uuid.UUID) error {
	s.reset = append(s.reset, id)
	return nil
}

type stubRecoveryQueue struct {
	enqueued []uuid.UUID
	cleared  int64
}

func (s *stubRecoveryQueue) Enqueue(_ context.Context, m *domain.Match) error {
	s.enqueued = append(s.enqueued, m.ID)
	return nil
}

func (s *stubRecoveryQueue) ClearDeadLetter(_ context.Context) (int64, error) { return s.cleared, nil }

func recoveryResponse(t *testing.T, h http.HandlerFunc, path string) map[string]int64 {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, nil)
	rec := httptest.NewRecorder()
	h(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var envelope struct {
		Data map[string]int64 `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	return envelope.Data
}

// --- Тесты ---

func TestRecovery_RetryOutboxErrors(t *testing.T) {
	log, _ := logger.New("error", "json")
	h := NewSystemRecoveryHandler(&stubRecoveryOutbox{retried: 3}, nil, nil, nil, nil, log)

	data := recoveryResponse(t, h.RetryOutboxErrors, "/system/recovery/outbox-retry")
	assert.Equal(t, int64(3), data["retried"])
}

func TestRecovery_RequeueCompiling(t *testing.T) {
	log, _ := logger.New("error", "json")
	programs := []*domain.Program{{ID: uuid.New()}, {ID: uuid.New()}}
	cq := &stubRecoveryCompileQueue{}
	h := NewSystemRecoveryHandler(nil, &stubRecoveryPrograms{programs: programs}, cq, nil, nil, log)

	data := recoveryResponse(t, h.RequeueCompiling, "/system/recovery/requeue-compiling")
	assert.Equal(t, int64(2), data["requeued"])
	assert.Len(t, cq.enqueued, 2)
}

func TestRecovery_ResetStuckMatches(t *testing.T) {
	log, _ := logger.New("error", "json")
	stuck := []*domain.Match{
		{ID: uuid.New(), Status: domain.MatchRunning},
		{ID: uuid.New(), Status: domain.MatchRunning},
	}
	mr := &stubRecoveryMatches{stuck: stuck}
	qm := &stubRecoveryQueue{}
	h := NewSystemRecoveryHandler(nil, nil, nil, mr, qm, log)

	data := recoveryResponse(t, h.ResetStuckMatches, "/system/recovery/reset-stuck-matches")
	assert.Equal(t, int64(2), data["reset"])
	assert.Len(t, mr.reset, 2)
	// Матчи возвращены в очередь со статусом pending.
	assert.Len(t, qm.enqueued, 2)
	for _, m := range stuck {
		assert.Equal(t, domain.MatchPending, m.Status)
	}
}

func TestRecovery_ClearDeadLetter(t *testing.T) {
	log, _ := logger.New("error", "json")
	h := NewSystemRecoveryHandler(nil, nil, nil, nil, &stubRecoveryQueue{cleared: 7}, log)

	data := recoveryResponse(t, h.ClearDeadLetter, "/system/recovery/clear-dead-letter")
	assert.Equal(t, int64(7), data["cleared"])
}
