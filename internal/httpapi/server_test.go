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
	if !strings.Contains(recorder.Body.String(), "G1 in progress") {
		t.Fatal("preview does not expose current gate")
	}
}

func TestSearchIsExplicitlyGated(t *testing.T) {
	recorder := httptest.NewRecorder()
	New("postgres://user:pass@localhost:5432/db").Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/search", nil))
	if recorder.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", recorder.Code)
	}
}
