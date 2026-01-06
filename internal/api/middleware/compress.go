package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// gzipWriterPool пул gzip writers для переиспользования
var gzipWriterPool = sync.Pool{
	New: func() interface{} {
		return gzip.NewWriter(io.Discard)
	},
}

// gzipResponseWriter обёртка над http.ResponseWriter с gzip сжатием
type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
	wroteHeader bool
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	if !w.wroteHeader {
		w.ResponseWriter.Header().Del("Content-Length")
		w.ResponseWriter.WriteHeader(status)
		w.wroteHeader = true
	}
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.Writer.Write(b)
}

// Compress middleware для gzip сжатия ответов
func Compress() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Проверяем, поддерживает ли клиент gzip
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}

			// Получаем gzip writer из пула
			gz := gzipWriterPool.Get().(*gzip.Writer)
			defer gzipWriterPool.Put(gz)

			gz.Reset(w)
			defer gz.Close()

			// Устанавливаем заголовки для gzip
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Vary", "Accept-Encoding")

			// Оборачиваем ResponseWriter
			gzw := &gzipResponseWriter{
				Writer:         gz,
				ResponseWriter: w,
			}

			next.ServeHTTP(gzw, r)
		})
	}
}

// CompressWithLevel middleware для gzip сжатия с указанным уровнем
func CompressWithLevel(level int) func(http.Handler) http.Handler {
	// Создаём отдельный пул для конкретного уровня сжатия
	writerPool := &sync.Pool{
		New: func() interface{} {
			w, _ := gzip.NewWriterLevel(io.Discard, level)
			return w
		},
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Проверяем, поддерживает ли клиент gzip
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}

			// Получаем gzip writer из пула
			gz := writerPool.Get().(*gzip.Writer)
			defer writerPool.Put(gz)

			gz.Reset(w)
			defer gz.Close()

			// Устанавливаем заголовки для gzip
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Vary", "Accept-Encoding")

			// Оборачиваем ResponseWriter
			gzw := &gzipResponseWriter{
				Writer:         gz,
				ResponseWriter: w,
			}

			next.ServeHTTP(gzw, r)
		})
	}
}
