package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"go.uber.org/zap"
)

// IdempotencyStore хранит ответы по Idempotency-Key; реализуется Redis-cache'ом.
//
// SetNX (SET if Not eXists) критичен: гарантирует, что параллельные запросы
// с одним ключом не создадут дублирующиеся ресурсы - только один "выиграет"
// и пойдёт к handler'у, остальные получат 409 или кэшированный ответ
// предыдущего (successful) вызова.
type IdempotencyStore interface {
	Get(ctx context.Context, key string) (string, error)
	SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
}

// idempotencyEntry - сохраняемый снапшот ответа.
type idempotencyEntry struct {
	Status int                 `json:"status"`
	Header map[string][]string `json:"header"`
	Body   string              `json:"body"`
}

// idempotencyRecorder - wrapper ResponseWriter для захвата ответа.
type idempotencyRecorder struct {
	http.ResponseWriter
	buf    *bytes.Buffer
	status int
}

func (r *idempotencyRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *idempotencyRecorder) Write(p []byte) (int, error) {
	r.buf.Write(p)
	return r.ResponseWriter.Write(p)
}

// Idempotency middleware реализует RFC-draft Idempotency-Key.
//
// Правила:
//   - Применяется к POST/PATCH (не к GET/PUT/DELETE, где семантика уже идемпотентна).
//   - Клиент посылает заголовок Idempotency-Key (не более 128 байт).
//   - Для первого запроса handler исполняется обычно, ответ сохраняется в store
//     под ключом на idempotencyTTL (24ч по умолчанию).
//   - Повторный запрос с тем же ключом возвращает сохранённый ответ без
//     пере-исполнения handler'а.
//   - Конкурентный запрос с тем же ключом получает 409 Conflict - защита от
//     двойного создания при параллельных ретраях.
const idempotencyTTL = 24 * time.Hour
const idempotencyKeyMax = 128
const idempotencyKeyPrefix = "idempotency:"

// Idempotency возвращает middleware, который использует store для дедубликации.
// Если store == nil, middleware ведёт себя как no-op (удобно для тестов).
func Idempotency(store IdempotencyStore, log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if store == nil {
				next.ServeHTTP(w, r)
				return
			}
			if r.Method != http.MethodPost && r.Method != http.MethodPatch {
				next.ServeHTTP(w, r)
				return
			}

			rawKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
			if rawKey == "" {
				next.ServeHTTP(w, r)
				return
			}
			if len(rawKey) > idempotencyKeyMax {
				http.Error(w, `{"error":"Idempotency-Key too long"}`, http.StatusBadRequest)
				return
			}

			cacheKey := idempotencyKeyPrefix + rawKey

			// 1. Проверяем, есть ли уже сохранённый ответ.
			if saved, err := store.Get(r.Context(), cacheKey); err == nil && saved != "" && saved != "in-flight" {
				var entry idempotencyEntry
				if err := json.Unmarshal([]byte(saved), &entry); err == nil {
					for k, vs := range entry.Header {
						for _, v := range vs {
							w.Header().Add(k, v)
						}
					}
					w.Header().Set("Idempotency-Status", "replayed")
					w.WriteHeader(entry.Status)
					_, _ = io.WriteString(w, entry.Body)
					return
				}
			}

			// 2. Концурентный запрос: пробуем захватить "in-flight" маркер через SetNX.
			ok, err := store.SetNX(r.Context(), cacheKey, "in-flight", idempotencyTTL)
			if err != nil {
				log.Warn("idempotency store error, bypassing", zap.Error(err))
				next.ServeHTTP(w, r)
				return
			}
			if !ok {
				// Чужой запрос захватил ключ - второй не должен пытаться создать.
				// 409 Conflict сообщает клиенту: повторите попытку позже.
				http.Error(w, `{"error":"duplicate request with same Idempotency-Key in progress"}`, http.StatusConflict)
				return
			}

			// 3. Первый запрос - исполняем handler и сохраняем snapshot.
			rec := &idempotencyRecorder{
				ResponseWriter: w,
				buf:            &bytes.Buffer{},
				status:         http.StatusOK,
			}
			next.ServeHTTP(rec, r)

			// Сохраняем только успешные ответы (2xx); ошибки клиент может фикснуть и повторить.
			if rec.status >= 200 && rec.status < 300 {
				headerSnapshot := map[string][]string{}
				for k, v := range w.Header() {
					// Пропускаем hop-by-hop / чувствительные заголовки.
					if strings.EqualFold(k, "Set-Cookie") || strings.EqualFold(k, "Authorization") {
						continue
					}
					headerSnapshot[k] = append([]string{}, v...)
				}
				entry := idempotencyEntry{
					Status: rec.status,
					Header: headerSnapshot,
					Body:   rec.buf.String(),
				}
				if payload, err := json.Marshal(entry); err == nil {
					_ = store.Set(r.Context(), cacheKey, string(payload), idempotencyTTL)
				}
			}
		})
	}
}
