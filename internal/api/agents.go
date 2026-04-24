package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jussmor/workspaces/internal/agent"
	"github.com/jussmor/workspaces/internal/sandbox"
	"github.com/jussmor/workspaces/internal/types"
)

// ─── Request / response shapes ───────────────────────────────────────────────

type agentRunRequest struct {
	RepoURL   string `json:"repo_url"`
	Task      string `json:"task"`
	Role      string `json:"role"`
	Image     string `json:"image"`
	SandboxID string `json:"sandbox_id,omitempty"` // reuse an existing sandbox
}

type agentRunResponse struct {
	TaskID    string         `json:"task_id"`
	Role      string         `json:"role"`
	Model     string         `json:"model"`
	Answer    string         `json:"answer"`
	SandboxID string         `json:"sandbox_id"` // always returned so the caller can reuse or destroy
	Stats     agent.RunStats `json:"stats"`
}

// ─── Handler ─────────────────────────────────────────────────────────────────

// handleAgentRun handles POST /agent/run.
//
// Behaviour:
//   - If sandbox_id is provided, the agent runs inside that existing sandbox.
//   - If not, a new sandbox is created AND KEPT ALIVE after the run finishes.
//     The caller is responsible for eventually calling DELETE /sandboxes/{id}.
//     This enables multi-agent pipelines (coder → reviewer → tester → pr_agent)
//     to hand off the same sandbox with its built artifacts and cloned repo.
func (s *Server) handleAgentRun(w http.ResponseWriter, r *http.Request) {
	var req agentRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Task == "" {
		writeError(w, http.StatusBadRequest, "task is required")
		return
	}

	role := agent.Role(req.Role)
	if req.Role == "" {
		role = agent.RoleCoder
	}

	img := req.Image
	if img == "" {
		// Empty → driver picks its default (forge-sandbox:latest).
		img = ""
	}

	task := types.Task{
		ID:          "agent-run-" + r.Header.Get("X-Request-Id"),
		Title:       req.Task,
		Description: req.Task,
		Status:      types.StatusRunning,
	}

	if req.RepoURL != "" {
		task.Description = "Repository: " + req.RepoURL + "\n\n" +
			"Your cwd is /home/forge/workspace. If it is empty (no .git), clone the repo INTO it: `git clone " + req.RepoURL + " .`\n" +
			"If /home/forge/workspace/.git already exists, reuse the existing clone.\n\n" +
			"PROXY: If you start a server on port <P>, the user can reach it at http://localhost:7001/sandboxes/SANDBOX_ID/port/<P>/\n\n" +
			"Then complete this task:\n" + req.Task
	}

	// Guard against hung tool calls (for example, long-running server processes).
	runCtx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	// Resolve the sandbox — reuse or create.
	sandboxID := req.SandboxID
	createdNew := false
	if sandboxID == "" {
		id, err := s.Sandbox.Create(runCtx, sandbox.Config{Image: img})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "create sandbox: "+err.Error())
			return
		}
		sandboxID = id
		createdNew = true

		// Record in the registry so it appears in GET /sandboxes and
		// participates in idle-reaping.
		ip, _ := s.Sandbox.IP(runCtx, sandboxID)
		now := time.Now().UTC()
		if rerr := s.Registry.Insert(sandbox.SandboxRecord{
			ID:         sandboxID,
			IP:         ip,
			Status:     sandbox.StatusRunning,
			ProjectID:  req.RepoURL,
			AgentRole:  string(role),
			CreatedAt:  now,
			LastActive: now,
			Transport:  "docker",
		}); rerr != nil {
			// Registry failure is non-fatal but we should log it.
			_ = rerr
		}
	} else {
		// Verify the sandbox exists in the registry.
		if _, gerr := s.Registry.Get(sandboxID); gerr != nil {
			writeError(w, http.StatusNotFound, "sandbox not found: "+sandboxID)
			return
		}
	}

	// Run the agent inside the sandbox. Sandbox is NOT destroyed afterward —
	// the caller must DELETE /sandboxes/{id} explicitly when done.
	answer, err := s.Agent.Run(runCtx, task, role, sandboxID)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			writeError(w, http.StatusGatewayTimeout, "agent run timed out after 10m (likely long-running command)")
			return
		}
		writeError(w, http.StatusInternalServerError, "agent run failed: "+err.Error())
		return
	}

	// Touch last_active so the reaper knows the sandbox is still relevant.
	_ = s.Registry.TouchLastActive(sandboxID)

	// Get the latest run stats.
	runs := s.Agent.Accounting.Runs()
	var stats agent.RunStats
	if len(runs) > 0 {
		stats = runs[len(runs)-1]
	}

	_ = createdNew // reserved for future metrics
	writeJSON(w, http.StatusOK, agentRunResponse{
		TaskID:    task.ID,
		Role:      string(role),
		Model:     agent.ModelForRole(role),
		Answer:    answer,
		SandboxID: sandboxID,
		Stats:     stats,
	})
}
