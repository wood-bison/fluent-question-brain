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
	mux.HandleFunc("GET /health/live", s.live)
	mux.HandleFunc("GET /health/ready", s.ready)
	mux.HandleFunc("POST /v1/search", s.searchNotReady)
	return requestID(mux)
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
