package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wood-bison/fluent-question-brain/internal/search"
	"github.com/wood-bison/fluent-question-brain/internal/store"
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

type duplicateReviewStub struct {
	httpSearchStub
	decision   store.DuplicateDecision
	candidates []store.DuplicateReviewCandidate
}

type capabilityBindingReviewStub struct {
	httpSearchStub
	proposal store.CapabilityBindingProposal
	revoked  struct {
		proposalID string
		actor      string
		rationale  string
	}
}

func (s *capabilityBindingReviewStub) ListCapabilityBindingProposals(context.Context, string, string) ([]store.CapabilityBindingProposal, error) {
	if s.proposal.ID == "" {
		return []store.CapabilityBindingProposal{}, nil
	}
	return []store.CapabilityBindingProposal{s.proposal}, nil
}

func (s *capabilityBindingReviewStub) DecideCapabilityBindingProposal(_ context.Context, proposalID, decision, actor, rationale string) (store.CapabilityBindingProposal, error) {
	s.proposal.ID = proposalID
	s.proposal.Status = decision
	s.proposal.DecidedBy = actor
	s.proposal.Rationale = rationale
	return s.proposal, nil
}

func (s *capabilityBindingReviewStub) RevokeCapabilityBindingProposal(_ context.Context, proposalID, actor, rationale string) (store.CapabilityBindingProposal, error) {
	s.revoked.proposalID = proposalID
	s.revoked.actor = actor
	s.revoked.rationale = rationale
	s.proposal.ID = proposalID
	s.proposal.Status = "rejected"
	s.proposal.DecidedBy = actor
	s.proposal.Rationale = rationale
	return s.proposal, nil
}

type qualityStub struct {
	httpSearchStub
	mu      sync.Mutex
	calls   int
	started chan struct{}
	release chan struct{}
}

func (s *qualityStub) Quality(_ context.Context, _ search.QualityRequest) (search.QualityResponse, error) {
	s.mu.Lock()
	s.calls++
	if s.calls == 1 && s.started != nil {
		close(s.started)
	}
	s.mu.Unlock()
	if s.release != nil {
		<-s.release
	}
	return search.QualityResponse{ContractVersion: "question-brain.quality.v1", WorkspaceKey: "fluent-interview"}, nil
}

func (s *qualityStub) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *duplicateReviewStub) ListDuplicateReviewCandidates(context.Context, string, string) ([]store.DuplicateReviewCandidate, error) {
	return s.candidates, nil
}

func (s *duplicateReviewStub) RecordDuplicateDecision(_ context.Context, decision store.DuplicateDecision) error {
	s.decision = decision
	return nil
}

type capabilityAliasReviewStub struct {
	httpSearchStub
	proposal store.CapabilityAliasSupersessionProposal
	decision struct {
		proposalID string
		decision   string
		actor      string
		rationale  string
	}
}

func (s *capabilityAliasReviewStub) ListCapabilityAliasSupersessionProposals(context.Context, string, string) ([]store.CapabilityAliasSupersessionProposal, error) {
	if s.proposal.ID == "" {
		return []store.CapabilityAliasSupersessionProposal{}, nil
	}
	return []store.CapabilityAliasSupersessionProposal{s.proposal}, nil
}

func (s *capabilityAliasReviewStub) CreateCapabilityAliasSupersessionProposal(_ context.Context, request store.CapabilityAliasSupersessionProposalRequest, actor string) (store.CapabilityAliasSupersessionProposal, error) {
	s.proposal = store.CapabilityAliasSupersessionProposal{
		ID: "proposal-1", WorkspaceKey: request.WorkspaceKey, Action: request.Action,
		SourceKey: request.SourceKey, CanonicalKey: request.CanonicalKey,
		Reason: request.Reason, Source: request.Source, Status: "proposed",
	}
	if actor == "" {
		s.proposal.Source = "question-brain-editorial"
	}
	return s.proposal, nil
}

func (s *capabilityAliasReviewStub) DecideCapabilityAliasSupersessionProposal(_ context.Context, proposalID, decision, actor, rationale string) (store.CapabilityAliasSupersessionProposal, error) {
	s.decision = struct {
		proposalID string
		decision   string
		actor      string
		rationale  string
	}{proposalID, decision, actor, rationale}
	s.proposal.Status = decision
	s.proposal.DecidedBy = actor
	s.proposal.Reason = rationale
	return s.proposal, nil
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

func TestQualityCoalescesConcurrentReadsAndCachesSnapshot(t *testing.T) {
	stub := &qualityStub{started: make(chan struct{}), release: make(chan struct{})}
	handler := New("postgres://user:pass@localhost:5432/db", stub).Handler()
	const requests = 8
	responses := make(chan int, requests)
	var group sync.WaitGroup
	group.Add(requests)
	for i := 0; i < requests; i++ {
		go func() {
			defer group.Done()
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/quality", nil))
			responses <- recorder.Code
		}()
	}
	select {
	case <-stub.started:
	case <-time.After(time.Second):
		t.Fatal("quality read did not start")
	}
	close(stub.release)
	group.Wait()
	close(responses)
	for status := range responses {
		if status != http.StatusOK {
			t.Fatalf("quality status = %d, want 200", status)
		}
	}
	if got := stub.callCount(); got != 1 {
		t.Fatalf("quality calls = %d, want one coalesced read", got)
	}
}

func TestDuplicateDecisionRequiresInternalToken(t *testing.T) {
	t.Setenv("QUESTION_BRAIN_INTERNAL_TOKEN", "review-token")
	stub := &duplicateReviewStub{}
	recorder := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"left_stable_key":"question.a","right_stable_key":"question.b","decision":"not_duplicate","rationale":"Different prompts"}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/duplicates/decision", body)
	New("postgres://user:pass@localhost:5432/db", stub).Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func TestDuplicateDecisionRequiresRationale(t *testing.T) {
	t.Setenv("QUESTION_BRAIN_INTERNAL_TOKEN", "review-token")
	stub := &duplicateReviewStub{}
	recorder := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"left_stable_key":"question.a","right_stable_key":"question.b","decision":"not_duplicate"}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/duplicates/decision", body)
	request.Header.Set("X-Question-Brain-Token", "review-token")
	New("postgres://user:pass@localhost:5432/db", stub).Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
	if stub.decision.Decision != "" {
		t.Fatal("invalid decision reached the write service")
	}
}

func TestDuplicateDecisionReturnsAuditableContract(t *testing.T) {
	t.Setenv("QUESTION_BRAIN_INTERNAL_TOKEN", "review-token")
	stub := &duplicateReviewStub{}
	recorder := httptest.NewRecorder()
	payload, err := json.Marshal(map[string]any{
		"workspace_key": "fluent-interview", "left_stable_key": "question.a", "right_stable_key": "question.b",
		"exact_score": 0.97, "semantic_score": 0.41, "decision": "not_duplicate", "rationale": "Different capability boundaries.",
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/duplicates/decision", bytes.NewReader(payload))
	request.Header.Set("X-Question-Brain-Token", "review-token")
	request.Header.Set("X-Question-Brain-Actor", "sergey")
	New("postgres://user:pass@localhost:5432/db", stub).Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"contract_version":"question-brain.duplicate-review.v1"`) {
		t.Fatal("response does not expose the duplicate review contract")
	}
	if stub.decision.Actor != "sergey" || stub.decision.Rationale == "" || stub.decision.Decision != "not_duplicate" {
		t.Fatalf("unexpected persisted decision: %#v", stub.decision)
	}
}

func TestDuplicateReviewQueueReturnsDurableCandidates(t *testing.T) {
	stub := &duplicateReviewStub{candidates: []store.DuplicateReviewCandidate{{
		ID: "candidate-1", WorkspaceKey: "fluent-interview",
		LeftStableKey: "question.a", LeftRevisionID: "revision-a",
		RightStableKey: "question.b", RightRevisionID: "revision-b",
		Decision: "open",
	}}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/duplicates/review?workspace=fluent-interview&status=proposed", nil)
	New("postgres://user:pass@localhost:5432/db", stub).Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"contract_version":"question-brain.duplicate-review.v1"`) ||
		!strings.Contains(body, `"candidates":[`) || !strings.Contains(body, `"candidate-1"`) {
		t.Fatalf("durable duplicate queue contract missing: %s", body)
	}
}

func TestCapabilityAliasReviewQueueIsExplicitlyEmpty(t *testing.T) {
	stub := &capabilityAliasReviewStub{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/capability-aliases/review?workspace=fluent-interview&status=proposed", nil)
	New("postgres://user:pass@localhost:5432/db", stub).Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"contract_version":"question-brain.capability-alias-supersession-review.v1"`) || !strings.Contains(body, `"proposals":[]`) {
		t.Fatalf("queue contract is not explicit: %s", body)
	}
}

func TestCapabilityAliasReviewDecisionRequiresInternalToken(t *testing.T) {
	t.Setenv("QUESTION_BRAIN_INTERNAL_TOKEN", "review-token")
	stub := &capabilityAliasReviewStub{proposal: store.CapabilityAliasSupersessionProposal{ID: "proposal-1", Status: "proposed"}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/capability-aliases/review/proposal-1/decision", bytes.NewBufferString(`{"decision":"accepted","rationale":"confirmed"}`))
	New("postgres://user:pass@localhost:5432/db", stub).Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func TestCapabilityAliasReviewDecisionReturnsVersionedContract(t *testing.T) {
	t.Setenv("QUESTION_BRAIN_INTERNAL_TOKEN", "review-token")
	stub := &capabilityAliasReviewStub{proposal: store.CapabilityAliasSupersessionProposal{
		ID: "proposal-1", WorkspaceKey: "fluent-interview", Action: "alias",
		SourceKey: "capability.legacy.loop", CanonicalKey: "capability.nodejs.event-loop-ordering", Status: "proposed",
	}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/capability-aliases/review/proposal-1/decision", bytes.NewBufferString(`{"decision":"accepted","rationale":"canonical registry review"}`))
	request.Header.Set("X-Question-Brain-Token", "review-token")
	request.Header.Set("X-Question-Brain-Actor", "sergey")
	New("postgres://user:pass@localhost:5432/db", stub).Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"contract_version":"question-brain.capability-alias-supersession-decision.v1"`) {
		t.Fatal("decision contract version missing")
	}
	if stub.decision.actor != "sergey" || stub.decision.decision != "accepted" || stub.decision.rationale == "" {
		t.Fatalf("unexpected alias decision: %#v", stub.decision)
	}
}

func TestCapabilityBindingRevocationRequiresInternalToken(t *testing.T) {
	t.Setenv("QUESTION_BRAIN_INTERNAL_TOKEN", "review-token")
	stub := &capabilityBindingReviewStub{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/capability-bindings/review/proposal-1/revoke", bytes.NewBufferString(`{"rationale":"invalid path"}`))
	New("postgres://user:pass@localhost:5432/db", stub).Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func TestCapabilityBindingRevocationRequiresRationale(t *testing.T) {
	t.Setenv("QUESTION_BRAIN_INTERNAL_TOKEN", "review-token")
	stub := &capabilityBindingReviewStub{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/capability-bindings/review/proposal-1/revoke", bytes.NewBufferString(`{}`))
	request.Header.Set("X-Question-Brain-Token", "review-token")
	New("postgres://user:pass@localhost:5432/db", stub).Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
	if stub.revoked.proposalID != "" {
		t.Fatal("invalid revocation reached the write service")
	}
}

func TestCapabilityBindingRevocationReturnsVersionedContract(t *testing.T) {
	t.Setenv("QUESTION_BRAIN_INTERNAL_TOKEN", "review-token")
	stub := &capabilityBindingReviewStub{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/capability-bindings/review/proposal-1/revoke", bytes.NewBufferString(`{"rationale":"path/revision mismatch discovered by release compiler"}`))
	request.Header.Set("X-Question-Brain-Token", "review-token")
	request.Header.Set("X-Question-Brain-Actor", "sergey-integrity")
	New("postgres://user:pass@localhost:5432/db", stub).Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"contract_version":"question-brain.capability-binding-revocation.v1"`) ||
		!strings.Contains(recorder.Body.String(), `"release_required":true`) {
		t.Fatalf("revocation contract missing: %s", recorder.Body.String())
	}
	if stub.revoked.actor != "sergey-integrity" || stub.revoked.rationale == "" || stub.revoked.proposalID != "proposal-1" {
		t.Fatalf("unexpected revocation: %#v", stub.revoked)
	}
}
