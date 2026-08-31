package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/metrics"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

// Metrics пишет prometheus-метрики по каждому http-запросу: счётчик, длительность,
// in-flight. лейбл пути берётся из chi RoutePattern ("/tournaments/{id}"), а не из
// сырого пути — иначе id в урле раздули бы кардинальнось метрик. паттерн chi
// проставляет по ходу роутинга, так что читается он уже после обработки
func Metrics() func(http.Handler) http.Handler {
	m := metrics.New()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m.HTTPRequestsInFlight.Inc()
			defer m.HTTPRequestsInFlight.Dec()

			start := time.Now()
			ww := chiMiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			pattern := chi.RouteContext(r.Context()).RoutePattern()
			if pattern == "" {
				pattern = "unmatched"
			}

			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}

			m.RecordHTTPRequest(r.Method, pattern, strconv.Itoa(status), time.Since(start))
		})
	}
}
