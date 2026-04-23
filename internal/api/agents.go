package api

import (
	"encoding/json"
	"net/http"

	"github.com/jussmor/workspaces/internal/agent"
	"github.com/jussmor/workspaces/internal/sandbox"
	"github.com/jussmor/workspaces/internal/types"
)

// ─── Request / response shapes ───────────────────────────────────────────────

type agentRunRequest struct {
	RepoURL     string `json:"repo_url"`
	Task        string `json:"task"`
	Role        string `json:"role"`
	Image       string `json:"image"`
}

type agentRunResponse struct {
	TaskID  string           `json:"task_id"`
	Role    string           `json:"role"`
	Model   string           `json:"model"`
	Answer  string           `json:"answer"`
	Stats   agent.RunStats   `json:"stats"`
}

// ─── Handler ─────────────────────────────────────────────────────────────────

// handleAgentRun handles POST /agent/run.
// Spawns a sandbox, clones the repo (if given), runs a single agent to
// completion, and returns the result. The sandbox is destroyed afterward.
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
		img = "alpine:3.19"
	}

	task := types.Task{
		ID:          "agent-run-" + r.Header.Get("X-Request-Id"),
		Title:       req.Task,
		Description: req.Task,
		Status:      types.StatusRunning,
	}

	// Clone step baked into task description if repo provided.
	if req.RepoURL != "" {
		task.Description = "Repository: " + req.RepoURL + "\n\n" +
			"First clone the repository with: git clone " + req.RepoURL + " /workspace && cd /workspace\n\n" +
			"Then complete this task:\n" + req.Task
	}

	answer, err := s.Agent.RunSingle(r.Context(), task, role, sandbox.Config{Image: img})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "agent run failed: "+err.Error())
		return
	}

	// Get the latest run stats.
	runs := s.Agent.Accounting.Runs()
	var stats agent.RunStats
	if len(runs) > 0 {
		stats = runs[len(runs)-1]
	}

	writeJSON(w, http.StatusOK, agentRunResponse{
		TaskID: task.ID,
		Role:   string(role),
		Model:  agent.ModelForRole(role),
		Answer: answer,
		Stats:  stats,
	})
}
