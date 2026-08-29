package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Audit ---

type mockAuditReader struct{ mock.Mock }

func (m *mockAuditReader) List(ctx context.Context, limit int) ([]*models.AuditLogEntry, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.AuditLogEntry), args.Error(1)
}

func newAuditHandlerFixture(t *testing.T) (*AuditHandler, *mockAuditReader) {
	t.Helper()
	log, _ := logger.New("error", "json")
	repo := new(mockAuditReader)
	return NewAuditHandler(repo, log), repo
}

// без параметра — лимит по умолчанию 100, тело с записью декодируется
func TestAuditHandler_List_DefaultLimit(t *testing.T) {
	h, repo := newAuditHandlerFixture(t)
	entry := &models.AuditLogEntry{ID: uuid.New(), Action: "POST /tournaments", CreatedAt: time.Now()}
	repo.On("List", mock.Anything, 100).Return([]*models.AuditLogEntry{entry}, nil)

	req := httptest.NewRequest(http.MethodGet, "/admin/audit", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var envelope struct {
		Data []models.AuditLogEntry `json:"data"`
	}
	assert.NoError(t, json.NewDecoder(rec.Body).Decode(&envelope))
	assert.Len(t, envelope.Data, 1)
	assert.Equal(t, "POST /tournaments", envelope.Data[0].Action)
}

// разбор query-параметра limit: валидный, обрезка сверху и мусор
func TestAuditHandler_List_LimitParsing(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  int
	}{
		{"custom", "?limit=50", 50},
		{"caps at 500", "?limit=99999", 500},
		{"invalid falls back to default", "?limit=garbage", 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, repo := newAuditHandlerFixture(t)
			repo.On("List", mock.Anything, tc.want).Return([]*models.AuditLogEntry{}, nil)

			req := httptest.NewRequest(http.MethodGet, "/admin/audit"+tc.query, nil)
			rec := httptest.NewRecorder()
			h.List(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			repo.AssertCalled(t, "List", mock.Anything, tc.want)
		})
	}
}

func TestAuditHandler_List_RepoError_500(t *testing.T) {
	h, repo := newAuditHandlerFixture(t)
	repo.On("List", mock.Anything, 100).Return(nil, errors.New("db down"))

	req := httptest.NewRequest(http.MethodGet, "/admin/audit", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- Стабы для recovery ---

type stubRecoveryOutbox struct{ retried int64 }

func (s *stubRecoveryOutbox) RetryErrors(_ context.Context) (int64, error) { return s.retried, nil }

type stubRecoveryPrograms struct{ programs []*models.Program }

func (s *stubRecoveryPrograms) GetStuckCompiling(_ context.Context, _ time.Duration, _ int) ([]*models.Program, error) {
	return s.programs, nil
}

type stubRecoveryCompileQueue struct{ enqueued []uuid.UUID }

func (s *stubRecoveryCompileQueue) Enqueue(_ context.Context, id uuid.UUID) error {
	s.enqueued = append(s.enqueued, id)
	return nil
}

type stubRecoveryMatches struct {
	stuck []*models.Match
	reset []uuid.UUID
}

func (s *stubRecoveryMatches) GetStuckRunning(_ context.Context, _ time.Duration, _ int) ([]*models.Match, error) {
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

func (s *stubRecoveryQueue) Enqueue(_ context.Context, m *models.Match) error {
	s.enqueued = append(s.enqueued, m.ID)
	return nil
}

func (s *stubRecoveryQueue) ClearDeadLetter(_ context.Context) (int64, error) { return s.cleared, nil }

// гоняет recovery-ручку и возвращает распакованный data-конверт
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

// --- Тесты recovery ---

func TestRecovery_RetryOutboxErrors(t *testing.T) {
	log, _ := logger.New("error", "json")
	h := NewSystemRecoveryHandler(&stubRecoveryOutbox{retried: 3}, nil, nil, nil, nil, log)

	data := recoveryResponse(t, h.RetryOutboxErrors, "/system/recovery/outbox-retry")
	assert.Equal(t, int64(3), data["retried"])
}

func TestRecovery_RequeueCompiling(t *testing.T) {
	log, _ := logger.New("error", "json")
	programs := []*models.Program{{ID: uuid.New()}, {ID: uuid.New()}}
	cq := &stubRecoveryCompileQueue{}
	h := NewSystemRecoveryHandler(nil, &stubRecoveryPrograms{programs: programs}, cq, nil, nil, log)

	data := recoveryResponse(t, h.RequeueCompiling, "/system/recovery/requeue-compiling")
	assert.Equal(t, int64(2), data["requeued"])
	assert.Len(t, cq.enqueued, 2)
}

func TestRecovery_ResetStuckMatches(t *testing.T) {
	log, _ := logger.New("error", "json")
	stuck := []*models.Match{
		{ID: uuid.New(), Status: models.MatchRunning},
		{ID: uuid.New(), Status: models.MatchRunning},
	}
	mr := &stubRecoveryMatches{stuck: stuck}
	qm := &stubRecoveryQueue{}
	h := NewSystemRecoveryHandler(nil, nil, nil, mr, qm, log)

	data := recoveryResponse(t, h.ResetStuckMatches, "/system/recovery/reset-stuck-matches")
	assert.Equal(t, int64(2), data["reset"])
	assert.Len(t, mr.reset, 2)
	// матчи вернулись в очередь уже со статусом pending
	assert.Len(t, qm.enqueued, 2)
	for _, m := range stuck {
		assert.Equal(t, models.MatchPending, m.Status)
	}
}

func TestRecovery_ClearDeadLetter(t *testing.T) {
	log, _ := logger.New("error", "json")
	h := NewSystemRecoveryHandler(nil, nil, nil, nil, &stubRecoveryQueue{cleared: 7}, log)

	data := recoveryResponse(t, h.ClearDeadLetter, "/system/recovery/clear-dead-letter")
	assert.Equal(t, int64(7), data["cleared"])
}
