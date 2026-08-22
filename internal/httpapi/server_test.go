package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLiveEndpoint(t *testing.T) {
	recorder := httptest.NewRecorder()
	New("postgres://user:pass@localhost:5432/db").Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/live", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestSearchIsExplicitlyGated(t *testing.T) {
	recorder := httptest.NewRecorder()
	New("postgres://user:pass@localhost:5432/db").Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/search", nil))
	if recorder.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", recorder.Code)
	}
}
