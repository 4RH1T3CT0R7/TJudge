package middleware

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AuditLogSink принимает записи audit-лога, обычно это репозиторий БД
type AuditLogSink interface {
	Insert(ctx context.Context, e *models.AuditLogEntry) error
}

// AuditLogger оборачивает AuditLogSink асинхронным буфером, чтобы
// HTTP-обработчик не ждал INSERT в БД
//
// при переполнени буфера запись дропается (лог "audit log buffer full"),
// а не блокирует запрос: потерять пару строк аудита лучше чем повесить API
type AuditLogger struct {
	sink    AuditLogSink
	ch      chan *models.AuditLogEntry
	log     *logger.Logger
	dropped atomic.Int64
}

// NewAuditLogger создаёт logger с буфером заданного размера
// Run() запускать в горутине, Close() - для graceful shutdown
func NewAuditLogger(sink AuditLogSink, bufferSize int, log *logger.Logger) *AuditLogger {
	if bufferSize <= 0 {
		bufferSize = 1024
	}
	return &AuditLogger{
		sink: sink,
		ch:   make(chan *models.AuditLogEntry, bufferSize),
		log:  log,
	}
}

// Run - фоновый воркер, пишет записи в sink; завершается на закрытии канала
func (a *AuditLogger) Run(ctx context.Context) {
	for entry := range a.ch {
		// отдельный контекст на insert, request ctx мог уже отмениться
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

func (a *AuditLogger) Close() { close(a.ch) }

func (a *AuditLogger) Dropped() int64 { return a.dropped.Load() }

// enqueue кладёт запись в канал не блокируясь, при переполнении - drop
func (a *AuditLogger) enqueue(e *models.AuditLogEntry) {
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

// перехватывает status code ответа
type auditResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *auditResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Audit пишет в audit-лог изменяющие запросы (POST/PUT/PATCH/DELETE)
// от админов - нужно для разбора инцидентов
//
// ставить ПОСЛЕ auth middleware: читает UserIDKey и RoleKey из контекста
func Audit(a *AuditLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// изменяющие методы, read-запросы не аудитим
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			// только админы, остальных и так режет rbac
			role, _ := r.Context().Value(RoleKey).(models.Role)
			if role != models.RoleAdmin {
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

			entry := &models.AuditLogEntry{
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
