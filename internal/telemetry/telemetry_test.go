package telemetry

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPWithMetricsCorrelatesSafely(t *testing.T) {
	metrics := NewMetrics()
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	handler := HTTPWithMetrics(metricsHandler(http.StatusTeapot, "ok"), metrics)
	req := httptest.NewRequest(http.MethodGet, "/v1/search?query=do-not-log-this", nil)
	req.Header.Set("X-Request-ID", "g5-redaction-test")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusTeapot)
	}
	metricsRecorder := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(metricsRecorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	metricsText := metricsRecorder.Body.String()
	if !strings.Contains(metricsText, "question_brain_http_requests_total 1") || !strings.Contains(metricsText, "question_brain_http_errors_total 1") {
		t.Fatalf("metrics did not record request/error: %s", metricsText)
	}
	logText := logs.String()
	for _, expected := range []string{"g5-redaction-test", "v1/search", "trace_id", "span_id"} {
		if !strings.Contains(logText, expected) {
			t.Fatalf("log missing %q: %s", expected, logText)
		}
	}
	if strings.Contains(logText, "do-not-log-this") {
		t.Fatalf("query leaked into structured log: %s", logText)
	}
}

func metricsHandler(status int, body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
}
