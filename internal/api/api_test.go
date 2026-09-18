package api

import (
	"net/http/httptest"
	"testing"
)

func TestHealthIsPublic(t *testing.T) {
	s := Server{APIKey: "secret"}
	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	res := httptest.NewRecorder()
	s.Handler().ServeHTTP(res, req)
	if res.Code != 200 {
		t.Fatalf("expected 200, got %d", res.Code)
	}
}

func TestProtectedEndpointRequiresKey(t *testing.T) {
	s := Server{APIKey: "secret"}
	req := httptest.NewRequest("GET", "/api/v1/scans", nil)
	res := httptest.NewRecorder()
	s.Handler().ServeHTTP(res, req)
	if res.Code != 401 {
		t.Fatalf("expected 401, got %d", res.Code)
	}
}
