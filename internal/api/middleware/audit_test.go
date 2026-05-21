package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// stubSink накапливает audit entries для проверки в тестах.
type stubSink struct {
	mu      sync.Mutex
	entries []*domain.AuditLogEntry
	err     error
}

func (s *stubSink) Insert(_ context.Context, e *domain.AuditLogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.entries = append(s.entries, e)
	return nil
}

func (s *stubSink) len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

func (s *stubSink) last() *domain.AuditLogEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.entries) == 0 {
		return nil
	}
	return s.entries[len(s.entries)-1]
}

func newAuditFixture(t *testing.T) (*AuditLogger, *stubSink, context.CancelFunc) {
	t.Helper()
	log, _ := logger.New("error", "json")
	sink := &stubSink{}
	al := NewAuditLogger(sink, 64, log)
	ctx, cancel := context.WithCancel(context.Background())
	go al.Run(ctx)
	return al, sink, func() {
		al.Close()
		cancel()
	}
}

func TestAudit_RecordsAdminMutation(t *testing.T) {
	al, sink, cleanup := newAuditFixture(t)
	defer cleanup()

	adminID := uuid.New()
	handler := Audit(al)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tournaments", nil)
	req.RemoteAddr = "10.0.0.5:12345"
	req.Header.Set("User-Agent", "ci-test/1.0")
	ctx := context.WithValue(req.Context(), UserIDKey, adminID)
	ctx = context.WithValue(ctx, RoleKey, domain.RoleAdmin)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusCreated, rr.Code)

	// async - ждём drain
	assert.Eventually(t, func() bool { return sink.len() == 1 }, time.Second, 10*time.Millisecond)
	e := sink.last()
	assert.Equal(t, adminID, e.ActorID)
	assert.Equal(t, string(domain.RoleAdmin), e.ActorRole)
	assert.Equal(t, "POST /api/v1/tournaments", e.Action)
	assert.Equal(t, http.StatusCreated, e.StatusCode)
	assert.Equal(t, "10.0.0.5", e.IP)
	assert.Equal(t, "ci-test/1.0", e.UserAgent)
}

func TestAudit_IgnoresGETRequests(t *testing.T) {
	al, sink, cleanup := newAuditFixture(t)
	defer cleanup()

	handler := Audit(al)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tournaments", nil)
	ctx := context.WithValue(req.Context(), UserIDKey, uuid.New())
	ctx = context.WithValue(ctx, RoleKey, domain.RoleAdmin)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	time.Sleep(50 * time.Millisecond) // интервал drain
	assert.Equal(t, 0, sink.len(), "GET должен игнорироваться")
}

func TestAudit_IgnoresNonAdmin(t *testing.T) {
	al, sink, cleanup := newAuditFixture(t)
	defer cleanup()

	handler := Audit(al)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/teams", nil)
	ctx := context.WithValue(req.Context(), UserIDKey, uuid.New())
	ctx = context.WithValue(ctx, RoleKey, domain.RoleUser)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 0, sink.len(), "не-admin действия не попадают в audit")
}

func TestAudit_BufferOverflow_DropsRatherThanBlocks(t *testing.T) {
	log, _ := logger.New("error", "json")
	// Маленький буфер + sink, который блокирует, должен приводить к drop, а не к deadlock.
	blockingSink := &blockingSink{start: make(chan struct{})}
	al := NewAuditLogger(blockingSink, 2, log)
	ctx := t.Context()
	go al.Run(ctx)
	defer al.Close()

	handler := Audit(al)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for range 10 {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/x", nil)
		req.RemoteAddr = "1.2.3.4:99"
		c := context.WithValue(req.Context(), UserIDKey, uuid.New())
		c = context.WithValue(c, RoleKey, domain.RoleAdmin)
		req = req.WithContext(c)
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
	// Хотя бы 1 запись должна быть отброшена при блокирующем sink и буфере 2.
	// (1 в work, 2 в буфере, остальные drop)
	assert.Greater(t, al.Dropped(), int64(0), "при переполнении буфера дропы должны расти")
}

type blockingSink struct {
	start chan struct{}
}

func (b *blockingSink) Insert(ctx context.Context, _ *domain.AuditLogEntry) error {
	select {
	case <-b.start:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
