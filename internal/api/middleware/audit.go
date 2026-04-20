package middleware

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AuditLogSink принимает записи audit-лога. Реализуется репозиторием БД.
type AuditLogSink interface {
	Insert(ctx context.Context, e *domain.AuditLogEntry) error
}

// AuditLogger обёртывает AuditLogSink асинхронным буфером: HTTP-обработчик
// не блокируется ожиданием INSERT в БД.
//
// Overflow-политика: при переполнении буфера запись отбрасывается с логом
// "audit log buffer full" вместо блокировки запроса.
type AuditLogger struct {
	sink    AuditLogSink
	ch      chan *domain.AuditLogEntry
	log     *logger.Logger
	dropped atomic.Int64
}

// NewAuditLogger создаёт logger с буфером заданного размера.
// Запустите Run() в отдельной горутине; Close() для graceful shutdown.
func NewAuditLogger(sink AuditLogSink, bufferSize int, log *logger.Logger) *AuditLogger {
	if bufferSize <= 0 {
		bufferSize = 1024
	}
	return &AuditLogger{
		sink: sink,
		ch:   make(chan *domain.AuditLogEntry, bufferSize),
		log:  log,
	}
}

// Run запускает background-воркер, записывающий записи в sink.
// Завершается когда канал закроется (Close()).
func (a *AuditLogger) Run(ctx context.Context) {
	for entry := range a.ch {
		// Используем отдельный context для insert (request ctx мог отмениться).
		insertCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := a.sink.Insert(insertCtx, entry); err != nil {
			a.log.Error("Failed to write audit log entry",
				zap.Error(err),
				zap.String("action", entry.Action),
				zap.String("actor_id", entry.ActorID.String()),
			)
		}
		cancel()
	}
}

// Close закрывает буфер. Вызовите после Stop() HTTP-сервера.
func (a *AuditLogger) Close() { close(a.ch) }

// Dropped возвращает число записей, отброшенных из-за переполнения буфера.
func (a *AuditLogger) Dropped() int64 { return a.dropped.Load() }

// enqueue неблокирующе помещает запись в канал; при переполнении - drop.
func (a *AuditLogger) enqueue(e *domain.AuditLogEntry) {
	select {
	case a.ch <- e:
	default:
		a.dropped.Add(1)
		a.log.Warn("audit log buffer full, entry dropped",
			zap.String("action", e.Action),
			zap.Int64("total_dropped", a.dropped.Load()),
		)
	}
}

// auditResponseWriter захватывает status code ответа для аудита.
type auditResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *auditResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Audit middleware пишет в audit-лог все state-changing запросы
// (POST/PUT/PATCH/DELETE) от пользователей с ролью admin.
//
// Вызывать ПОСЛЕ auth middleware: читает UserIDKey и RoleKey из контекста.
//
// Audit trail нужен для compliance и incident response.
func Audit(a *AuditLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Только mutation-методы (POST/PUT/PATCH/DELETE); read-запросы не аудитим.
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			// Только админы (остальные действия и так лимитированы rbac).
			role, _ := r.Context().Value(RoleKey).(domain.Role)
			if role != domain.RoleAdmin {
				next.ServeHTTP(w, r)
				return
			}

			userID, ok := r.Context().Value(UserIDKey).(uuid.UUID)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			aw := &auditResponseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(aw, r)

			entry := &domain.AuditLogEntry{
				ID:         uuid.New(),
				ActorID:    userID,
				ActorRole:  string(role),
				Action:     r.Method + " " + r.URL.Path,
				Method:     r.Method,
				Path:       r.URL.Path,
				StatusCode: aw.status,
				IP:         getClientIP(r),
				UserAgent:  r.UserAgent(),
				CreatedAt:  time.Now(),
			}
			a.enqueue(entry)
		})
	}
}
