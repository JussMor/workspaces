package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jussmor/workspaces/internal/sandbox"
)

// ─── Request / response shapes ───────────────────────────────────────────────

type createSandboxRequest struct {
	ProjectID string `json:"project_id"`
	AgentRole string `json:"agent_role"`
	Image     string `json:"image"`
}

type execRequest struct {
	Cmd []string `json:"cmd"`
}

type execResponse struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

type sandboxResponse struct {
	ID         string    `json:"id"`
	IP         string    `json:"ip"`
	Status     string    `json:"status"`
	ProjectID  string    `json:"project_id,omitempty"`
	AgentRole  string    `json:"agent_role,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	LastActive time.Time `json:"last_active"`
	MemUsage   int64     `json:"mem_usage"`
	Transport  string    `json:"transport"`
}

type listSandboxesResponse struct {
	Sandboxes []sandboxResponse `json:"sandboxes"`
}

// ─── Handlers ────────────────────────────────────────────────────────────────

// handleCreateSandbox handles POST /sandboxes.
func (s *Server) handleCreateSandbox(w http.ResponseWriter, r *http.Request) {
	var req createSandboxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	cfg := sandbox.Config{
		Image: req.Image,
	}

	containerID, err := s.Sandbox.Create(r.Context(), cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create sandbox: "+err.Error())
		return
	}

	ip, err := s.Sandbox.IP(r.Context(), containerID)
	if err != nil {
		// Non-fatal: container is running but we couldn't get the IP yet.
		ip = ""
	}

	now := time.Now().UTC()
	rec := sandbox.SandboxRecord{
		ID:         containerID,
		IP:         ip,
		Status:     sandbox.StatusRunning,
		ProjectID:  req.ProjectID,
		AgentRole:  req.AgentRole,
		CreatedAt:  now,
		LastActive: now,
		MemUsage:   0,
		Transport:  "none",
	}

	if err := s.Registry.Insert(rec); err != nil {
		// Registry failure — clean up the container.
		_ = s.Sandbox.Destroy(r.Context(), containerID)
		writeError(w, http.StatusInternalServerError, "registry insert: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, recordToResponse(rec))
}

// handleListSandboxes handles GET /sandboxes.
func (s *Server) handleListSandboxes(w http.ResponseWriter, r *http.Request) {
	f := sandbox.ListFilter{
		Status:    r.URL.Query().Get("status"),
		ProjectID: r.URL.Query().Get("project_id"),
	}

	records, err := s.Registry.List(f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list sandboxes: "+err.Error())
		return
	}

	resp := listSandboxesResponse{Sandboxes: make([]sandboxResponse, 0, len(records))}
	for _, rec := range records {
		resp.Sandboxes = append(resp.Sandboxes, recordToResponse(rec))
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleGetSandbox handles GET /sandboxes/{id}.
func (s *Server) handleGetSandbox(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rec, err := s.Registry.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "sandbox not found: "+id)
		return
	}
	writeJSON(w, http.StatusOK, recordToResponse(rec))
}

// handleExecSandbox handles POST /sandboxes/{id}/exec.
func (s *Server) handleExecSandbox(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req execRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if len(req.Cmd) == 0 {
		writeError(w, http.StatusBadRequest, "cmd must not be empty")
		return
	}

	// Verify sandbox exists in registry.
	if _, err := s.Registry.Get(id); err != nil {
		writeError(w, http.StatusNotFound, "sandbox not found: "+id)
		return
	}

	// Build command string from array.
	cmdStr := buildCmd(req.Cmd)

	result, err := s.Sandbox.Exec(r.Context(), id, cmdStr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "exec: "+err.Error())
		return
	}

	// Update last_active.
	_ = s.Registry.TouchLastActive(id)

	writeJSON(w, http.StatusOK, execResponse{
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
		ExitCode: result.ExitCode,
	})
}

// handleDeleteSandbox handles DELETE /sandboxes/{id}.
func (s *Server) handleDeleteSandbox(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := s.Sandbox.Destroy(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "destroy sandbox: "+err.Error())
		return
	}
	if err := s.Registry.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, "registry delete: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func recordToResponse(rec sandbox.SandboxRecord) sandboxResponse {
	return sandboxResponse{
		ID:         rec.ID,
		IP:         rec.IP,
		Status:     string(rec.Status),
		ProjectID:  rec.ProjectID,
		AgentRole:  rec.AgentRole,
		CreatedAt:  rec.CreatedAt,
		LastActive: rec.LastActive,
		MemUsage:   rec.MemUsage,
		Transport:  rec.Transport,
	}
}

// buildCmd joins a command slice into a shell-safe string.
// If there is only a single element it is used as-is (allows passing a
// pre-built shell string like "echo hello").
func buildCmd(cmd []string) string {
	if len(cmd) == 1 {
		return cmd[0]
	}
	// For multi-element slices pass through exec directly by joining with
	// proper escaping. A simple join is fine for the sandbox use case.
	out := ""
	for i, part := range cmd {
		if i > 0 {
			out += " "
		}
		out += shellQuote(part)
	}
	return out
}

// shellQuote wraps a string in single quotes, escaping any embedded single
// quotes by ending the quote, inserting a literal single quote, and re-opening.
func shellQuote(s string) string {
	safe := ""
	for _, r := range s {
		if r == '\'' {
			safe += "'\\''"
		} else {
			safe += string(r)
		}
	}
	return "'" + safe + "'"
}

// writeJSON encodes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
