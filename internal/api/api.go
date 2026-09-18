package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reconix-cloud/internal/jobs"
	"reconix-cloud/internal/storage"
	"strings"
	"time"
)

type Server struct {
	Manager *jobs.Manager
	Store   storage.Store
	Results storage.ResultStore
	APIKey  string
}

func (s Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", s.health)
	mux.HandleFunc("/api/v1/scans", s.scans)
	mux.HandleFunc("/api/v1/scans/", s.scan)
	mux.HandleFunc("/openapi.json", s.openapi)
	return s.auth(mux)
}
func (s Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Request-ID") == "" {
			w.Header().Set("X-Request-ID", fmt.Sprintf("req_%d", time.Now().UnixNano()))
		} else {
			w.Header().Set("X-Request-ID", r.Header.Get("X-Request-ID"))
		}
		if r.URL.Path == "/api/v1/health" || r.URL.Path == "/openapi.json" || s.APIKey == "" {
			next.ServeHTTP(w, r)
			return
		}
		key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if key == "" || key != s.APIKey {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (s Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
func (s Server) scans(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var request struct{ Target, Profile string }
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&request) != nil || request.Target == "" {
			writeError(w, 400, "INVALID_REQUEST", "target is required")
			return
		}
		if request.Profile == "" {
			request.Profile = "standard"
		}
		if request.Profile != "safe" && request.Profile != "standard" && request.Profile != "deep" {
			writeError(w, 400, "INVALID_PROFILE", "unsupported profile")
			return
		}
		scan, err := s.Manager.Create(r.Context(), request.Target, request.Profile)
		if err != nil {
			writeError(w, 400, "TARGET_NOT_ALLOWED", "target is not permitted")
			return
		}
		writeJSON(w, http.StatusAccepted, scan)
	case http.MethodGet:
		scans, err := s.Store.List(r.Context())
		if err != nil {
			writeError(w, 500, "STORAGE_ERROR", "unable to list scans")
			return
		}
		writeJSON(w, 200, scans)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
func (s Server) scan(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		w.WriteHeader(404)
		return
	}
	id := parts[3]
	if len(parts) == 5 && parts[4] == "results" {
		if r.Method != http.MethodGet {
			w.WriteHeader(405)
			return
		}
		result, err := s.Results.Result(id)
		if err != nil {
			writeError(w, 404, "RESULTS_NOT_FOUND", "results are not available")
			return
		}
		writeJSON(w, 200, result)
		return
	}
	if len(parts) == 5 && parts[4] == "cancel" {
		if r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		if err := s.Manager.Cancel(r.Context(), id); err != nil {
			writeError(w, 409, "INVALID_STATE", "scan cannot be cancelled")
			return
		}
		scan, _ := s.Store.Get(r.Context(), id)
		writeJSON(w, 200, scan)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(405)
		return
	}
	scan, err := s.Store.Get(r.Context(), id)
	if err != nil {
		writeError(w, 404, "SCAN_NOT_FOUND", "scan not found")
		return
	}
	writeJSON(w, 200, scan)
}
func (s Server) openapi(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"openapi": "3.0.3", "info": map[string]string{"title": "Reconix Cloud API", "version": "1.0"}, "paths": map[string]any{"/api/v1/health": map[string]any{"get": map[string]string{"summary": "Health check"}}, "/api/v1/scans": map[string]any{"get": map[string]string{"summary": "List scans"}, "post": map[string]string{"summary": "Queue a scan"}}}})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message, "request_id": w.Header().Get("X-Request-ID")}})
}
