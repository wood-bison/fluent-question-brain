package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wood-bison/fluent-question-brain/internal/search"
)

type httpSearchStub struct {
	catalogRequest search.CatalogRequest
}

func (s *httpSearchStub) Search(context.Context, search.Request) ([]search.Result, error) {
	return nil, nil
}

func (s *httpSearchStub) GetQuestion(context.Context, string, string, string) (search.Question, error) {
	return search.Question{}, nil
}

func (s *httpSearchStub) Catalog(_ context.Context, request search.CatalogRequest) (search.CatalogResponse, error) {
	s.catalogRequest = request
	return search.CatalogResponse{Questions: []search.CatalogItem{}}, nil
}

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

func TestQuestionBrowserIsReadOnlySurface(t *testing.T) {
	recorder := httptest.NewRecorder()
	New("postgres://user:pass@localhost:5432/db").Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/browse", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("content type = %q", got)
	}
	body := recorder.Body.String()
	for _, marker := range []string{"Банк вопросов", "/v1/catalog", "/v1/search", "Все компании"} {
		if !strings.Contains(body, marker) {
			t.Fatalf("browser is missing %q", marker)
		}
	}
}

func TestCatalogPassesDimensionFilters(t *testing.T) {
	stub := &httpSearchStub{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/catalog?workspace=fluent-interview&locale=ru&topic_key=topic.http&track=Backend&level=Senior&company=Acme&offset=20&limit=40", nil)
	New("postgres://user:pass@localhost:5432/db", stub).Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	got := stub.catalogRequest
	if got.WorkspaceKey != "fluent-interview" || got.Locale != "ru" || got.TopicKey != "topic.http" {
		t.Fatalf("identity filters = %#v", got)
	}
	if got.Track != "Backend" || got.Level != "Senior" || got.Company != "Acme" {
		t.Fatalf("dimension filters = %#v", got)
	}
	if got.Offset != 20 || got.Limit != 40 {
		t.Fatalf("paging = %#v", got)
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

func TestQualityRequiresService(t *testing.T) {
	recorder := httptest.NewRecorder()
	New("postgres://user:pass@localhost:5432/db").Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/quality", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", recorder.Code)
	}
}
