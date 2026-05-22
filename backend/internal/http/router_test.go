package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"crowdrise/backend/internal/config"
	"crowdrise/backend/internal/services"
)

func TestCORSPreflight(t *testing.T) {
	cfg := config.Config{CORSOrigin: "https://example.netlify.app"}
	router := NewRouter(services.New(nil, "test-secret"), cfg)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/projects", nil)
	req.Header.Set("Origin", "https://example.netlify.app")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://example.netlify.app" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
}
