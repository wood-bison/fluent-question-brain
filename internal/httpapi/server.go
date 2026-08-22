package httpapi

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"time"
)

type Server struct {
	databaseURL string
}

func New(databaseURL string) *Server {
	return &Server{databaseURL: databaseURL}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.index)
	mux.HandleFunc("GET /health/live", s.live)
	mux.HandleFunc("GET /health/ready", s.ready)
	mux.HandleFunc("POST /v1/search", s.searchNotReady)
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
  <title>Question Brain — G1 preview</title>
  <style>
    :root { color-scheme: dark; font-family: Inter, ui-sans-serif, system-ui, -apple-system, sans-serif; }
    * { box-sizing: border-box; }
    body { margin: 0; min-height: 100vh; color: #f4f7fb; background: #0b1020; }
    main { width: min(1120px, calc(100% - 48px)); margin: 0 auto; padding: 72px 0 80px; }
    .eyebrow { color: #8da5ff; letter-spacing: .18em; text-transform: uppercase; font-size: 12px; font-weight: 700; }
    h1 { margin: 14px 0 16px; font-size: clamp(42px, 8vw, 84px); line-height: .95; letter-spacing: -.06em; max-width: 760px; }
    .lede { color: #aeb9cf; font-size: 20px; line-height: 1.5; max-width: 720px; }
    .grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; margin-top: 44px; }
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
    @media (max-width: 760px) { main { width: min(100% - 32px, 640px); padding-top: 40px; } .grid { grid-template-columns: 1fr; } .lede { font-size: 17px; } }
  </style>
</head>
<body>
  <main>
    <div class="eyebrow">Fluent Engineering Lab · Question Brain</div>
    <h1>Content graph, built to be trusted.</h1>
    <p class="lede">A performance-first Go service for canonical question revisions, graph placement, exact search, and measured semantic retrieval.</p>
    <section class="grid" aria-label="System status">
      <article class="card"><div class="label">Service</div><div class="value ok">Running</div><code>Go · :8080</code></article>
      <article class="card"><div class="label">Storage</div><div class="value ok">Postgres + pgvector</div><code>vector 0.8.6 · pg18</code></article>
      <article class="card"><div class="label">Current gate</div><div class="value pending">G1 in progress</div><code>one-card round-trip proven</code></article>
    </section>
    <div class="links" aria-label="Diagnostics and API links">
      <a href="/health/live">Live health ↗</a>
      <a href="/health/ready">Readiness ↗</a>
    </div>
    <footer>Search stays explicitly gated until duplicate evidence is recorded. This preview is an operational surface, not a fake search demo.</footer>
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

func (s *Server) searchNotReady(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{
		"error":   "search_contract_not_implemented",
		"stage":   "G1",
		"message": "Close the one-card round-trip before exposing search as a production dependency.",
	})
}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Request-ID") == "" {
			w.Header().Set("X-Request-ID", "local-contract")
		}
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
