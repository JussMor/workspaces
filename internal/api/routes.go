package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	ghpkg "github.com/jussmor/workspaces/internal/github"
)

// registerRoutes binds all HTTP routes to the router.
func (s *Server) registerRoutes(r *chi.Mux) {
	// Health — always returns 200 (readiness probe).
	r.Get("/health", s.handleHealth)

	// Tasks — LAYER 04/06, Week 7-10.
	r.Post("/tasks", notImplemented("coordinator", "7-10"))
	r.Get("/tasks", notImplemented("coordinator", "7-10"))
	r.Get("/tasks/{id}", notImplemented("coordinator", "7-10"))

	// Sandboxes — LAYER 01/06, Week 1-2.
	r.Post("/sandboxes", s.handleCreateSandbox)
	r.Get("/sandboxes", s.handleListSandboxes)
	r.Get("/sandboxes/{id}", s.handleGetSandbox)
	r.Post("/sandboxes/{id}/exec", s.handleExecSandbox)
	r.Delete("/sandboxes/{id}", s.handleDeleteSandbox)

	// Agent — LAYER 02/06, Week 3-4.
	r.Post("/agent/run", s.handleAgentRun)

	// GitHub webhooks — LAYER 05, Week 9.
	r.Post("/webhooks/github", ghpkg.HandleWebhook)

	// WebSocket streams — LAYER 06, Week 10.
	r.Get("/tasks/{id}/logs/ws", notImplemented("api", "10"))
	r.Get("/notifications/ws", notImplemented("api", "10"))
}

// handleHealth responds to GET /health with a 200 JSON status payload.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "forge",
		"version": "0.0.1",
	})
}

// notImplemented returns an http.HandlerFunc that writes a 501 stub response.
func notImplemented(layer, week string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "not implemented",
			"layer": layer,
			"week":  week,
		})
	}
}
