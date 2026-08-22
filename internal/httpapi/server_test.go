package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLiveEndpoint(t *testing.T) {
	recorder := httptest.NewRecorder()
	New("postgres://user:pass@localhost:5432/db").Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/live", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestPreviewRoot(t *testing.T) {
	recorder := httptest.NewRecorder()
	New("postgres://user:pass@localhost:5432/db").Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("content type = %q", got)
	}
	if !strings.Contains(recorder.Body.String(), "G5 hardened") {
		t.Fatal("preview does not expose current gate")
	}
	if !strings.Contains(recorder.Body.String(), "Jaeger") {
		t.Fatal("preview does not expose observability sink")
	}
}

func TestSearchRequiresService(t *testing.T) {
	recorder := httptest.NewRecorder()
	New("postgres://user:pass@localhost:5432/db").Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/search", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", recorder.Code)
	}
}

func TestCatalogRequiresService(t *testing.T) {
	recorder := httptest.NewRecorder()
	New("postgres://user:pass@localhost:5432/db").Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/catalog", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", recorder.Code)
	}
}

func TestReleaseRequiresService(t *testing.T) {
	recorder := httptest.NewRecorder()
	New("postgres://user:pass@localhost:5432/db").Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/release", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", recorder.Code)
	}
}
