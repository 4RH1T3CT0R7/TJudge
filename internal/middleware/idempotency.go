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

// IdempotencyStore хранит ответы по Idempotency-Key поверх Redis-кэша
//
// SetNX тут ключевой: если два запроса с одним ключом пришли разом, ключ
// захватит только один и пойдёт в handler, остальные получат 409 или уже
// сохранённый ответ прошлого удачного вызова
type IdempotencyStore interface {
	Get(ctx context.Context, key string) (string, error)
	SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
}

// снапшот ответа для сохранения
type idempotencyEntry struct {
	Status int                 `json:"status"`
	Header map[string][]string `json:"header"`
	Body   string              `json:"body"`
}

// обёртка над ResponseWriter, перехватывает ответ
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

// правила Idempotency-Key (RFC-draft):
// только POST/PATCH, у остальных методов семантика и так идемпотентна;
// первый запрос выполняется и ответ кладётся в store на idempotencyTTL,
// повтор с тем же ключом отдаёт сохранённый ответ, конкурент ловит 409;
// ключ скоупится по userID+method+path чтобы нельзя было переиграть чужой,
// при не-2xx или панике in-flight маркер снимается - можно сразу ретраить
//
// TODO: 24ч захардкожено, надо бы вынести TTL в конфиг
const idempotencyTTL = 24 * time.Hour

// страховочный TTL маркера "запрос выполняется": обычно маркер снимается явно,
// а этот TTL спасает только если процесс упал между SetNX и концом handler'а
const inFlightTTL = 2 * time.Minute

const idempotencyKeyMax = 128
const idempotencyKeyPrefix = "idempotency:"

// Idempotency возвращает middleware дедубликации по store
// store == nil - no-op (удобно в тестах)
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

			// скоупим ключ по юзеру и маршруту: иначе угадавший чужой ключ
			// мог бы получить чужой ответ или заблокировать чужое создание
			scope := "anon"
			if userID, ok := GetUserID(r.Context()); ok {
				scope = userID.String()
			}
			cacheKey := idempotencyKeyPrefix + scope + ":" + r.Method + ":" + r.URL.Path + ":" + rawKey

			// 1. есть ли уже сохранённый ответ
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

			// 2. пробуем захватить in-flight маркер через SetNX
			ok, err := store.SetNX(r.Context(), cacheKey, "in-flight", inFlightTTL)
			if err != nil {
				log.Warn("idempotency store error, bypassing", zap.Error(err))
				next.ServeHTTP(w, r)
				return
			}
			if !ok {
				// ключ уже кем-то захвачен, второму создавать нельзя
				// 409 говорит клиенту повторить позже
				http.Error(w, `{"error":"duplicate request with same Idempotency-Key in progress"}`, http.StatusConflict)
				return
			}

			// 3. первый запрос - выполняем handler и сохраняем snapshot
			// если ответ не сохранили (не-2xx, ошибка сериализации, паника),
			// снимаем маркер - иначе честный ретрай ловил бы 409 до конца TTL
			stored := false
			defer func() {
				if !stored {
					_ = store.Del(r.Context(), cacheKey)
				}
			}()

			rec := &idempotencyRecorder{
				ResponseWriter: w,
				buf:            &bytes.Buffer{},
				status:         http.StatusOK,
			}
			next.ServeHTTP(rec, r)

			// храним только успешные ответы (2xx), ошибку клиент починит и повторит
			if rec.status >= 200 && rec.status < 300 {
				headerSnapshot := map[string][]string{}
				for k, v := range w.Header() {
					// пропускаем чувствительные заголовки
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
					if store.Set(r.Context(), cacheKey, string(payload), idempotencyTTL) == nil {
						stored = true
					}
				}
			}
		})
	}
}
