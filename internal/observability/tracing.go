// Package observability предоставляет инициализацию OpenTelemetry tracing (P2.6).
//
// Если задана переменная окружения OTEL_EXPORTER_OTLP_ENDPOINT (или любая
// из стандартных OTEL_*), провайдер отправляет spans в OTLP-совместимый
// collector (Jaeger, Tempo, OTEL Collector). Если не задано — возвращает
// noop-провайдер, и все Tracer-вызовы становятся бесплатными.
//
// Интеграция с HTTP:
//
//	r.Use(observability.HTTPMiddleware("tjudge-api"))
//
// Также отдаёт trace-id в response header X-Trace-ID для корреляции
// с логами и client-side Sentry-подобными системами.
package observability

import (
	"context"
	"net/http"
	"os"

	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// InitTracerProvider настраивает глобальный tracer provider.
//
// Возвращает shutdown-функцию, которую надо вызвать в defer на graceful shutdown
// (flush pending spans). Если OTEL-env не задано, возвращает no-op shutdown.
func InitTracerProvider(ctx context.Context, serviceName, serviceVersion string, log *logger.Logger) (func(context.Context) error, error) {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" && os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") == "" {
		log.Info("OpenTelemetry disabled (no OTLP endpoint configured)")
		return noopShutdown, nil
	}

	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return noopShutdown, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRate()))),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	log.Info("OpenTelemetry enabled",
		zap.String("service", serviceName),
		zap.String("version", serviceVersion),
	)

	return tp.Shutdown, nil
}

func noopShutdown(_ context.Context) error { return nil }

// sampleRate читает OTEL_TRACES_SAMPLER_ARG (0..1). По умолчанию — 10%.
func sampleRate() float64 {
	v := os.Getenv("OTEL_TRACES_SAMPLER_ARG")
	switch v {
	case "1", "1.0", "always":
		return 1.0
	case "0":
		return 0
	case "":
		return 0.1
	}
	// Больше не разбираем — Prometheus, kafka, и friends используют ParentBased.
	return 0.1
}

// HTTPMiddleware оборачивает handler в otelhttp middleware с операцией-именем
// и добавляет X-Trace-ID в response header для корреляции с логами.
func HTTPMiddleware(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		wrapped := otelhttp.NewHandler(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Добавляем trace-id в response header если span активен.
				if span := trace.SpanFromContext(r.Context()); span.SpanContext().IsValid() {
					w.Header().Set("X-Trace-ID", span.SpanContext().TraceID().String())
				}
				next.ServeHTTP(w, r)
			}),
			serviceName,
		)
		return wrapped
	}
}
