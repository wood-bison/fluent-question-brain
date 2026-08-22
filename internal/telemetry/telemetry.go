package telemetry

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/wood-bison/fluent-question-brain/internal/httpapi"

// Init configures trace export to Jaeger through OTLP/gRPC. An empty endpoint
// deliberately disables export for local one-off binaries while Compose sets
// the service endpoint to jaeger:4317.
func Init(ctx context.Context, serviceName, endpoint string) (func(context.Context) error, error) {
	if endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}
	resource, err := sdkresource.New(ctx,
		sdkresource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion("g5"),
			attribute.String("deployment.environment", "local-compose"),
		),
	)
	if err != nil {
		return nil, err
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(1.0))),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return provider.Shutdown, nil
}

// Metrics is intentionally small and dependency-free. The public surface is
// Prometheus text, while OpenTelemetry remains the trace/export contract.
// Counters avoid route labels so untrusted paths cannot create cardinality
// explosions in production.
type Metrics struct {
	requests        atomic.Uint64
	errors          atomic.Uint64
	requestDuration atomic.Uint64 // nanoseconds
	responseBytes   atomic.Uint64
	inFlight        atomic.Int64
}

func NewMetrics() *Metrics { return &Metrics{} }

func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		fmt.Fprintf(w, "# HELP question_brain_http_requests_total Total HTTP requests.\n# TYPE question_brain_http_requests_total counter\nquestion_brain_http_requests_total %d\n", m.requests.Load())
		fmt.Fprintf(w, "# HELP question_brain_http_errors_total HTTP responses with status >= 400.\n# TYPE question_brain_http_errors_total counter\nquestion_brain_http_errors_total %d\n", m.errors.Load())
		fmt.Fprintf(w, "# HELP question_brain_http_request_duration_seconds_sum Cumulative HTTP handler duration.\n# TYPE question_brain_http_request_duration_seconds_sum counter\nquestion_brain_http_request_duration_seconds_sum %.9f\n", float64(m.requestDuration.Load())/float64(time.Second))
		fmt.Fprintf(w, "# HELP question_brain_http_request_duration_seconds_count HTTP handler duration observations.\n# TYPE question_brain_http_request_duration_seconds_count counter\nquestion_brain_http_request_duration_seconds_count %d\n", m.requests.Load())
		fmt.Fprintf(w, "# HELP question_brain_http_response_bytes_total Total response bytes.\n# TYPE question_brain_http_response_bytes_total counter\nquestion_brain_http_response_bytes_total %d\n", m.responseBytes.Load())
		fmt.Fprintf(w, "# HELP question_brain_http_in_flight Current in-flight requests.\n# TYPE question_brain_http_in_flight gauge\nquestion_brain_http_in_flight %d\n", m.inFlight.Load())
	})
}

var defaultMetrics = NewMetrics()

func MetricsHandler() http.Handler { return defaultMetrics.Handler() }

func HTTP(next http.Handler) http.Handler { return HTTPWithMetrics(next, defaultMetrics) }

func HTTPWithMetrics(next http.Handler, metrics *Metrics) http.Handler {
	if metrics == nil {
		metrics = defaultMetrics
	}
	tracer := otel.Tracer(tracerName)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		metrics.inFlight.Add(1)
		defer metrics.inFlight.Add(-1)
		if r.Header.Get("X-Request-ID") == "" {
			r.Header.Set("X-Request-ID", "local-contract")
		}
		w.Header().Set("X-Request-ID", r.Header.Get("X-Request-ID"))
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		ctx, span := tracer.Start(ctx, r.Method+" "+r.URL.Path,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("http.request.method", r.Method),
				attribute.String("url.path", r.URL.Path),
			),
		)
		defer span.End()
		r = r.WithContext(ctx)
		wrapped := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(wrapped, r)
		status := wrapped.status
		if status == 0 {
			status = http.StatusOK
		}
		elapsed := time.Since(started)
		metrics.requests.Add(1)
		metrics.requestDuration.Add(uint64(elapsed))
		metrics.responseBytes.Add(uint64(wrapped.bytes))
		if status >= http.StatusBadRequest {
			metrics.errors.Add(1)
		}
		span.SetAttributes(
			attribute.Int("http.response.status_code", status),
			attribute.Int64("http.response.body.size", int64(wrapped.bytes)),
		)
		// Never log query strings or bodies: they can contain prompts, tokens, or
		// learner content. Trace and request IDs are enough to correlate a safe
		// operational record with Jaeger.
		slog.Default().Info("http request complete",
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration_ms", elapsed.Milliseconds(),
			"request_id", r.Header.Get("X-Request-ID"),
			"trace_id", span.SpanContext().TraceID().String(),
			"span_id", span.SpanContext().SpanID().String(),
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	count, err := w.ResponseWriter.Write(body)
	w.bytes += count
	return count, err
}

// Preserve the interfaces used by streaming and server-sent responses.
func (w *statusWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		if w.status == 0 {
			w.WriteHeader(http.StatusOK)
		}
		flusher.Flush()
	}
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *statusWriter) ReadFrom(reader io.Reader) (int64, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if optimized, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		count, err := optimized.ReadFrom(reader)
		w.bytes += int(count)
		return count, err
	}
	buffer := bytes.Buffer{}
	_, _ = buffer.ReadFrom(reader)
	count, err := w.Write(buffer.Bytes())
	return int64(count), err
}

func Shutdown(ctx context.Context, shutdown func(context.Context) error) error {
	if shutdown == nil {
		return nil
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return shutdown(shutdownCtx)
}
