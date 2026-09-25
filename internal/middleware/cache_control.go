package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
)

// CacheControl - middleware для GET, ставит Cache-Control: no-cache и считает ETag по body
//
// браузер хранит ответ, но перед каждым использованием ревалидирует: с валидным
// If-None-Match приходит 304 без тела. max-age тут не годится - после правки
// игры в админке перезапрос отдавался бы из браузерного кэша со старыми данными
//
//	r.With(middleware.CacheControl()).Get("/games", handler)
func CacheControl() func(http.Handler) http.Handler {
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
			// handler гоняется через recorder, снимаются body и статус
			next.ServeHTTP(rec, r)

			// ETag и Cache-Control только для 2xx, иначе в кэш попадёт ошибка
			// не-2xx просто пробрасывается как есть
			if rec.status < 200 || rec.status >= 300 {
				w.WriteHeader(rec.status)
				_, _ = w.Write(rec.buf.Bytes())
				return
			}

			sum := sha256.Sum256(rec.buf.Bytes())
			etag := `"` + hex.EncodeToString(sum[:16]) + `"` // 16 байт = 128 бит

			w.Header().Set("ETag", etag)
			w.Header().Set("Cache-Control", "no-cache")

			// If-None-Match совпал - отдаётся 304 без тела
			if match := r.Header.Get("If-None-Match"); match != "" && match == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}

			// пишется реальный статус и тело
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
