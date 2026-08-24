package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wood-bison/fluent-question-brain/internal/normalize"
	"github.com/wood-bison/fluent-question-brain/internal/search"
	"github.com/wood-bison/fluent-question-brain/internal/store"
)

type cardPromoter interface {
	PromoteCard(context.Context, normalize.Card, string, string, string) (store.StoredRevision, error)
}

type questionRollbacker interface {
	RollbackQuestion(context.Context, string, string, string, string) (store.StoredRevision, error)
}

type releaseReader interface {
	Release(context.Context, search.ReleaseRequest) (search.ReleaseResponse, error)
}

type qualityReader interface {
	Quality(context.Context, search.QualityRequest) (search.QualityResponse, error)
}

type Server struct {
	databaseURL   string
	searchService search.Service
	promoter      cardPromoter
	rollbacker    questionRollbacker
	internalToken string
}

func New(databaseURL string, service ...search.Service) *Server {
	var searchService search.Service
	var promoter cardPromoter
	var rollbacker questionRollbacker
	if len(service) > 0 {
		searchService = service[0]
		promoter, _ = service[0].(cardPromoter)
		rollbacker, _ = service[0].(questionRollbacker)
	}
	return &Server{
		databaseURL:   databaseURL,
		searchService: searchService,
		promoter:      promoter,
		rollbacker:    rollbacker,
		internalToken: strings.TrimSpace(os.Getenv("QUESTION_BRAIN_INTERNAL_TOKEN")),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.index)
	mux.HandleFunc("GET /health/live", s.live)
	mux.HandleFunc("GET /health/ready", s.ready)
	mux.HandleFunc("POST /v1/search", s.search)
	mux.HandleFunc("GET /browse", s.browse)
	mux.HandleFunc("GET /v1/catalog", s.catalog)
	mux.HandleFunc("GET /v1/release", s.release)
	mux.HandleFunc("GET /v1/quality", s.quality)
	mux.HandleFunc("GET /v1/questions/{stableKey}", s.question)
	mux.HandleFunc("POST /v1/questions/{stableKey}/rollback", s.rollback)
	mux.HandleFunc("POST /v1/promote", s.promote)
	return requestID(mux)
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	writeHTML(w, http.StatusOK, `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Question Brain — G5 production preview</title>
  <style>
    :root { color-scheme: dark; font-family: Inter, ui-sans-serif, system-ui, -apple-system, sans-serif; }
    * { box-sizing: border-box; }
    body { margin: 0; min-height: 100vh; color: #f4f7fb; background: #0b1020; }
    main { width: min(1120px, calc(100% - 48px)); margin: 0 auto; padding: 72px 0 80px; }
    .eyebrow { color: #8da5ff; letter-spacing: .18em; text-transform: uppercase; font-size: 12px; font-weight: 700; }
    h1 { margin: 14px 0 16px; font-size: clamp(42px, 8vw, 84px); line-height: .95; letter-spacing: -.06em; max-width: 760px; }
    .lede { color: #aeb9cf; font-size: 20px; line-height: 1.5; max-width: 720px; }
    .grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 14px; margin-top: 44px; }
    .card { padding: 22px; border: 1px solid #25304b; border-radius: 18px; background: linear-gradient(145deg, #141b31, #10162a); box-shadow: 0 18px 60px #0003; }
    .label { color: #8f9bb7; font-size: 12px; letter-spacing: .14em; text-transform: uppercase; }
    .value { margin-top: 10px; font-size: 25px; font-weight: 700; }
    .ok { color: #76e0a7; }
    .pending { color: #f5c76b; }
    .links { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 34px; }
    a { color: #b9c7ff; text-decoration: none; border: 1px solid #3a4c83; border-radius: 999px; padding: 11px 16px; }
    a:hover, a:focus-visible { background: #1a2855; outline: 2px solid #8da5ff; outline-offset: 2px; }
    code { color: #cbd6f7; font: 500 13px ui-monospace, SFMono-Regular, Menlo, monospace; }
    footer { margin-top: 54px; color: #7683a0; font-size: 13px; }
    @media (max-width: 980px) { .grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
    @media (max-width: 760px) { main { width: min(100% - 32px, 640px); padding-top: 40px; } .grid { grid-template-columns: 1fr; } .lede { font-size: 17px; } }
  </style>
</head>
<body>
  <main>
    <div class="eyebrow">Fluent Engineering Lab · Question Brain</div>
    <h1>Content graph, built to be trusted.</h1>
    <p class="lede">A performance-first Go service for canonical question revisions, graph placement, exact search, and measured semantic retrieval.</p>
    <section class="grid" aria-label="System status">
      <article class="card"><div class="label">Service</div><div class="value ok">Running</div><code>Go API · Compose</code></article>
      <article class="card"><div class="label">Storage</div><div class="value ok">Postgres + pgvector</div><code>vector 0.8.6 · pg18</code></article>
      <article class="card"><div class="label">Current gate</div><div class="value ok">G5 hardened</div><code>metrics · drills · rollback ready</code></article>
      <article class="card"><div class="label">Observability</div><div class="value ok">Jaeger</div><code>OTLP/gRPC · trace-ready</code></article>
    </section>
    <div class="links" aria-label="Diagnostics and API links">
      <a href="/health/live">Live health ↗</a>
      <a href="/health/ready">Readiness ↗</a>
      <a href="/v1/release?workspace=fluent-interview">Question release ↗</a>
      <a href="/browse">Question browser ↗</a>
      <a href="/metrics">Metrics ↗</a>
      <a href="http://localhost:56686/" target="_blank" rel="noreferrer">Jaeger UI ↗</a>
    </div>
    <footer>Search returns explainable provenance from the canonical graph. This preview is an operational surface, not a fake search demo.</footer>
  </main>
</body>
</html>`)
}

func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	parsed, err := url.Parse(s.databaseURL)
	if err != nil || parsed.Hostname() == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "reason": "invalid_database_url"})
		return
	}
	port := parsed.Port()
	if port == "" {
		port = "5432"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 700*time.Millisecond)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(parsed.Hostname(), port))
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "reason": "database_unreachable"})
		return
	}
	_ = conn.Close()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready", "database": "reachable", "migration": "compose-init"})
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	if s.searchService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error":   "search_service_unavailable",
			"message": "The API was started without a canonical search service.",
		})
		return
	}
	var request struct {
		Query    string `json:"query"`
		Locale   string `json:"locale"`
		TopicKey string `json:"topic_key"`
		Level    string `json:"level"`
		Company  string `json:"company"`
		Limit    int    `json:"limit"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_search_request"})
		return
	}
	results, err := s.searchService.Search(r.Context(), search.Request{
		WorkspaceKey: "fluent-interview",
		Query:        request.Query,
		Locale:       request.Locale,
		TopicKey:     request.TopicKey,
		Level:        request.Level,
		Company:      request.Company,
		Limit:        request.Limit,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"query":     request.Query,
		"locale":    request.Locale,
		"topic_key": request.TopicKey,
		"level":     request.Level,
		"company":   request.Company,
		"results":   results,
		"provenance": map[string]any{
			"pipeline":    []string{"exact", "fts", "trigram", "semantic"},
			"explainable": true,
		},
	})
}

func (s *Server) catalog(w http.ResponseWriter, r *http.Request) {
	if s.searchService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "catalog_service_unavailable",
		})
		return
	}
	parseInt := func(key string, defaultValue int) int {
		value, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get(key)))
		if err != nil {
			return defaultValue
		}
		return value
	}
	request := search.CatalogRequest{
		WorkspaceKey:    r.URL.Query().Get("workspace"),
		Locale:          r.URL.Query().Get("locale"),
		TopicKey:        r.URL.Query().Get("topic_key"),
		Track:           r.URL.Query().Get("track"),
		Level:           r.URL.Query().Get("level"),
		Company:         r.URL.Query().Get("company"),
		Offset:          parseInt("offset", 0),
		Limit:           parseInt("limit", 100),
		IncludeFixtures: parseBool(r.URL.Query().Get("include_fixtures")),
	}
	if strings.TrimSpace(request.WorkspaceKey) == "" {
		request.WorkspaceKey = "fluent-interview"
	}
	response, err := s.searchService.Catalog(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) release(w http.ResponseWriter, r *http.Request) {
	reader, ok := s.searchService.(releaseReader)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "release_service_unavailable",
		})
		return
	}
	workspaceKey := strings.TrimSpace(r.URL.Query().Get("workspace"))
	if workspaceKey == "" {
		workspaceKey = "fluent-interview"
	}
	response, err := reader.Release(r.Context(), search.ReleaseRequest{
		WorkspaceKey:    workspaceKey,
		IncludeFixtures: parseBool(r.URL.Query().Get("include_fixtures")),
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) quality(w http.ResponseWriter, r *http.Request) {
	reader, ok := s.searchService.(qualityReader)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "quality_service_unavailable",
		})
		return
	}
	workspaceKey := strings.TrimSpace(r.URL.Query().Get("workspace"))
	if workspaceKey == "" {
		workspaceKey = "fluent-interview"
	}
	response, err := reader.Quality(r.Context(), search.QualityRequest{
		WorkspaceKey:    workspaceKey,
		IncludeFixtures: parseBool(r.URL.Query().Get("include_fixtures")),
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func parseBool(value string) bool {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (s *Server) question(w http.ResponseWriter, r *http.Request) {
	if s.searchService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "question_service_unavailable"})
		return
	}
	question, err := s.searchService.GetQuestion(r.Context(), r.PathValue("stableKey"), "fluent-interview", r.URL.Query().Get("locale"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, question)
}

type promoteRequest struct {
	WorkspaceKey  string              `json:"workspace_key"`
	WorkspaceName string              `json:"workspace_name"`
	SourceRef     string              `json:"source_ref"`
	StableKey     string              `json:"stable_key"`
	Slug          string              `json:"slug"`
	Title         string              `json:"title"`
	Track         string              `json:"track"`
	Topic         string              `json:"topic"`
	Scope         string              `json:"scope"`
	Lang          string              `json:"lang"`
	Priority      string              `json:"priority"`
	Group         string              `json:"group"`
	Level         string              `json:"level"`
	ProgramKey    string              `json:"program_key"`
	PathKey       string              `json:"path_key"`
	DomainKey     string              `json:"domain_key"`
	CapabilityKey string              `json:"capability_key"`
	MappingState  string              `json:"mapping_state"`
	Question      string              `json:"question"`
	Sections      []normalize.Section `json:"sections"`
}

func (s *Server) promote(w http.ResponseWriter, r *http.Request) {
	if s.promoter == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "promote_service_unavailable"})
		return
	}
	if s.internalToken == "" || r.Header.Get("X-Question-Brain-Token") != s.internalToken {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_internal_token"})
		return
	}
	var request promoteRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, 2<<20))
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_promote_request"})
		return
	}
	if strings.TrimSpace(request.WorkspaceKey) == "" {
		request.WorkspaceKey = "fluent-interview"
	}
	if strings.TrimSpace(request.WorkspaceName) == "" {
		request.WorkspaceName = "Fluent Interview"
	}
	if strings.TrimSpace(request.SourceRef) == "" {
		request.SourceRef = "payload://question/" + strings.TrimSpace(request.StableKey)
	}
	canonicalFields := map[string]any{
		"stable_key": request.StableKey,
		"slug":       request.Slug,
		"title":      request.Title,
		"track":      request.Track,
		"topic":      request.Topic,
		"scope":      request.Scope,
		"lang":       request.Lang,
		"priority":   request.Priority,
		"group":      request.Group,
		"level":      request.Level,
		"question":   request.Question,
		"sections":   request.Sections,
	}
	// Optional curriculum fields are omitted when empty so an older CMS client
	// continues to produce the exact legacy payload/hash shape.
	if request.ProgramKey != "" {
		canonicalFields["program_key"] = request.ProgramKey
	}
	if request.PathKey != "" {
		canonicalFields["path_key"] = request.PathKey
	}
	if request.DomainKey != "" {
		canonicalFields["domain_key"] = request.DomainKey
	}
	if request.CapabilityKey != "" {
		canonicalFields["capability_key"] = request.CapabilityKey
	}
	if request.MappingState != "" {
		canonicalFields["mapping_state"] = request.MappingState
	}
	canonical, err := json.Marshal(canonicalFields)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "encode_promote_payload"})
		return
	}
	card, err := normalize.CardFromPayload(request.SourceRef, canonical)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	actor := strings.TrimSpace(r.Header.Get("X-Question-Brain-Actor"))
	if actor == "" {
		actor = "payload-cms"
	}
	stored, err := s.promoter.PromoteCard(r.Context(), card, request.WorkspaceKey, request.WorkspaceName, actor)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "published",
		"action":           stored.Action,
		"question_id":      stored.QuestionID,
		"revision_id":      stored.RevisionID,
		"content_hash":     stored.Hash,
		"revision_created": stored.RevisionCreated,
		"source":           "payload-cms",
	})
}

func (s *Server) rollback(w http.ResponseWriter, r *http.Request) {
	if s.rollbacker == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "rollback_service_unavailable"})
		return
	}
	if s.internalToken == "" || r.Header.Get("X-Question-Brain-Token") != s.internalToken {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_internal_token"})
		return
	}
	var request struct {
		RevisionID string `json:"revision_id"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
	if err := decoder.Decode(&request); err != nil || strings.TrimSpace(request.RevisionID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "revision_id_required"})
		return
	}
	actor := strings.TrimSpace(r.Header.Get("X-Question-Brain-Actor"))
	stored, err := s.rollbacker.RollbackQuestion(r.Context(), "fluent-interview", r.PathValue("stableKey"), request.RevisionID, actor)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "published",
		"action":       stored.Action,
		"question_id":  stored.QuestionID,
		"revision_id":  stored.RevisionID,
		"content_hash": stored.Hash,
		"source":       "question-brain-rollback",
	})
}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Request-ID") == "" {
			r.Header.Set("X-Request-ID", "local-contract")
		}
		w.Header().Set("X-Request-ID", r.Header.Get("X-Request-ID"))
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeHTML(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}
