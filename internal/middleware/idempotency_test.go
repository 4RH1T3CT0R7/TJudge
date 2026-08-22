package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// stubStore - in-memory IdempotencyStore для тестов.
type stubStore struct {
	mu   sync.Mutex
	data map[string]string
}

func newStubStore() *stubStore {
	return &stubStore{data: map[string]string{}}
}

func (s *stubStore) Get(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data[key], nil
}

func (s *stubStore) SetNX(_ context.Context, key string, value any, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[key]; exists {
		return false, nil
	}
	s.data[key] = toString(value)
	return true, nil
}

func (s *stubStore) Set(_ context.Context, key string, value any, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = toString(value)
	return nil
}

func (s *stubStore) Del(_ context.Context, keys ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, k := range keys {
		delete(s.data, k)
	}
	return nil
}

func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return string(x)
	default:
		return "?"
	}
}

func newIdempLog(t *testing.T) *logger.Logger {
	t.Helper()
	log, _ := logger.New("error", "json")
	return log
}

func TestIdempotency_FirstRequestPasses(t *testing.T) {
	store := newStubStore()
	var called int32
	handler := Idempotency(store, newIdempLog(t))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&called, 1)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"123"}`))
	}))

	req := httptest.NewRequest(http.MethodPost, "/tournaments", strings.NewReader(""))
	req.Header.Set("Idempotency-Key", "first-request-1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, int32(1), atomic.LoadInt32(&called))
	body, _ := io.ReadAll(rec.Body)
	assert.Equal(t, `{"id":"123"}`, string(body))
}

func TestIdempotency_RepeatReturnsCachedResponse(t *testing.T) {
	store := newStubStore()
	var called int32
	handler := Idempotency(store, newIdempLog(t))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&called, 1)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"abc"}`))
	}))

	// Первый запрос
	req1 := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(""))
	req1.Header.Set("Idempotency-Key", "same-key")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	// Повторный запрос с тем же ключом
	req2 := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(""))
	req2.Header.Set("Idempotency-Key", "same-key")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	assert.Equal(t, int32(1), atomic.LoadInt32(&called), "handler должен вызваться ровно один раз")
	assert.Equal(t, http.StatusCreated, rec2.Code)
	body, _ := io.ReadAll(rec2.Body)
	assert.Equal(t, `{"id":"abc"}`, string(body))
	assert.Equal(t, "replayed", rec2.Header().Get("Idempotency-Status"))
}

func TestIdempotency_NoKeyPassesThrough(t *testing.T) {
	store := newStubStore()
	var called int32
	handler := Idempotency(store, newIdempLog(t))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&called, 1)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	// Без Idempotency-Key
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, int32(1), atomic.LoadInt32(&called))
}

func TestIdempotency_TooLongKey(t *testing.T) {
	store := newStubStore()
	handler := Idempotency(store, newIdempLog(t))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler должен быть заблокирован")
	}))

	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set("Idempotency-Key", strings.Repeat("x", 200))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestIdempotency_FailedResponseNotCached(t *testing.T) {
	store := newStubStore()
	var called int32
	handler := Idempotency(store, newIdempLog(t))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&called, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`oops`))
	}))

	// Первый запрос - 500, не кэшируется, in-flight маркер снимается.
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set("Idempotency-Key", "retry-after-error")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req)
	assert.Equal(t, http.StatusInternalServerError, rec1.Code)

	// Честный ретрай с тем же ключом должен исполниться заново,
	// а не упереться в осиротевший in-flight маркер (409 на сутки).
	req2 := httptest.NewRequest(http.MethodPost, "/x", nil)
	req2.Header.Set("Idempotency-Key", "retry-after-error")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusInternalServerError, rec2.Code)
	assert.Equal(t, int32(2), atomic.LoadInt32(&called), "ретрай после ошибки должен пере-исполнить handler")
}

func TestIdempotency_KeyScopedByUser(t *testing.T) {
	store := newStubStore()
	var called int32
	handler := Idempotency(store, newIdempLog(t))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&called, 1)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"owned"}`))
	}))

	user1 := uuid.New()
	user2 := uuid.New()

	// Пользователь 1 создаёт ресурс со своим ключом.
	req1 := httptest.NewRequest(http.MethodPost, "/programs", nil)
	req1.Header.Set("Idempotency-Key", "shared-key")
	req1 = req1.WithContext(context.WithValue(req1.Context(), UserIDKey, user1))
	handler.ServeHTTP(httptest.NewRecorder(), req1)

	// Пользователь 2 с тем же ключом НЕ должен получить replay чужого ответа -
	// его запрос исполняется независимо.
	req2 := httptest.NewRequest(http.MethodPost, "/programs", nil)
	req2.Header.Set("Idempotency-Key", "shared-key")
	req2 = req2.WithContext(context.WithValue(req2.Context(), UserIDKey, user2))
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	assert.Equal(t, int32(2), atomic.LoadInt32(&called), "ключ должен скоупиться по пользователю")
	assert.Empty(t, rec2.Header().Get("Idempotency-Status"))

	// А повтор того же пользователя - реплеится.
	req3 := httptest.NewRequest(http.MethodPost, "/programs", nil)
	req3.Header.Set("Idempotency-Key", "shared-key")
	req3 = req3.WithContext(context.WithValue(req3.Context(), UserIDKey, user1))
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)

	assert.Equal(t, int32(2), atomic.LoadInt32(&called))
	assert.Equal(t, "replayed", rec3.Header().Get("Idempotency-Status"))
}

func TestIdempotency_PanicReleasesInFlight(t *testing.T) {
	store := newStubStore()
	var called int32
	handler := Idempotency(store, newIdempLog(t))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&called, 1) == 1 {
			panic("handler exploded")
		}
		w.WriteHeader(http.StatusCreated)
	}))

	req1 := httptest.NewRequest(http.MethodPost, "/x", nil)
	req1.Header.Set("Idempotency-Key", "panic-key")
	assert.Panics(t, func() { handler.ServeHTTP(httptest.NewRecorder(), req1) })

	// После паники маркер снят - ретрай исполняется и завершается успешно.
	req2 := httptest.NewRequest(http.MethodPost, "/x", nil)
	req2.Header.Set("Idempotency-Key", "panic-key")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusCreated, rec2.Code)
	assert.Equal(t, int32(2), atomic.LoadInt32(&called))
}

func TestIdempotency_IgnoresGET(t *testing.T) {
	store := newStubStore()
	var called int32
	handler := Idempotency(store, newIdempLog(t))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&called, 1)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Idempotency-Key", "not-applicable")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	assert.Equal(t, int32(1), atomic.LoadInt32(&called))
}
