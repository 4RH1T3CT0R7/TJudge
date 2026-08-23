package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
)

// CacheControl - middleware для GET, ставит Cache-Control и считает ETag по body
//
// разгружает бэкенд на редко меняющихся ресурсах (список игр, турниры):
// клиент с валидным If-None-Match получает 304 без генерации тела
//
//	r.With(middleware.CacheControl(60)).Get("/games", handler)
//
// maxAgeSeconds - значение max-age, 0 = no-store
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
			// гоняем handler через recorder, ловим body и статус
			next.ServeHTTP(rec, r)

			// ETag и Cache-Control только для 2xx, иначе закэшируем ошибку
			// не-2xx просто пробрасываем как есть
			if rec.status < 200 || rec.status >= 300 {
				w.WriteHeader(rec.status)
				_, _ = w.Write(rec.buf.Bytes())
				return
			}

			sum := sha256.Sum256(rec.buf.Bytes())
			etag := `"` + hex.EncodeToString(sum[:16]) + `"` // 16 байт = 128 бит

			w.Header().Set("ETag", etag)
			w.Header().Set("Cache-Control", directive)

			// If-None-Match совпал - отдаём 304 без тела
			if match := r.Header.Get("If-None-Match"); match != "" && match == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}

			// пишем реальный статус и тело
			w.WriteHeader(rec.status)
			_, _ = w.Write(rec.buf.Bytes())
		})
	}
}

// копит body и status, чтобы потом посчитать хэш
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
