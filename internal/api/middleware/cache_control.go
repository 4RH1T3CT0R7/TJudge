package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
)

// CacheControl — middleware для идемпотентных GET-эндпоинтов,
// который выставляет Cache-Control header и генерирует ETag из body.
//
// P2.2: снижает нагрузку на backend для редко-меняющихся ресурсов
// (справочники игр, список турниров). Клиент с валидным If-None-Match
// получает 304 без заново-генерации тела.
//
// Использование:
//
//	r.With(middleware.CacheControl(60)).Get("/games", handler)
//
// maxAgeSeconds — значение `max-age=`, 0 → no-store.
func CacheControl(maxAgeSeconds int) func(http.Handler) http.Handler {
	directive := "no-store"
	if maxAgeSeconds > 0 {
		directive = "public, max-age=" + strconv.Itoa(maxAgeSeconds) + ", stale-while-revalidate=" + strconv.Itoa(maxAgeSeconds/2)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				next.ServeHTTP(w, r)
				return
			}

			rec := &etagRecorder{
				ResponseWriter: w,
				buf:            &bytes.Buffer{},
				status:         http.StatusOK,
			}
			next.ServeHTTP(rec, r)

			// ETag считаем только для 200/successfully-rendered ответов.
			if rec.status < 200 || rec.status >= 300 {
				_, _ = w.Write(rec.buf.Bytes())
				return
			}

			sum := sha256.Sum256(rec.buf.Bytes())
			etag := `"` + hex.EncodeToString(sum[:16]) + `"` // 16 bytes == 128 bits

			w.Header().Set("ETag", etag)
			w.Header().Set("Cache-Control", directive)

			// Conditional request: If-None-Match совпал → 304 без тела.
			if match := r.Header.Get("If-None-Match"); match != "" && match == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}

			// Записываем реальный статус и тело.
			if rec.headerWritten {
				// Если наш recorder уже писал WriteHeader, оригинальный w ещё не
				// получил заголовок; проставим его сейчас.
				w.WriteHeader(rec.status)
			}
			_, _ = w.Write(rec.buf.Bytes())
		})
	}
}

// etagRecorder захватывает body и status для пост-пост-хэширования.
type etagRecorder struct {
	http.ResponseWriter
	buf           *bytes.Buffer
	status        int
	headerWritten bool
}

func (e *etagRecorder) WriteHeader(code int) {
	e.status = code
	e.headerWritten = true
}

func (e *etagRecorder) Write(p []byte) (int, error) {
	return e.buf.Write(p)
}
