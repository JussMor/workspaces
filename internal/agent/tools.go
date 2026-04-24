package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jussmor/workspaces/internal/sandbox"
)

// Tool is a callable action the agent can invoke during task execution.
// Each tool knows how to describe itself (for the LLM) and how to execute.
type Tool struct {
	Name        string
	Description string
	Parameters  ToolFuncParams
	Execute     func(ctx context.Context, sandboxID string, drv sandbox.Driver, args map[string]any) (string, error)
}

// ToolRegistry holds the complete set of tools keyed by name.
type ToolRegistry struct {
	tools map[string]Tool
}

// NewToolRegistry builds a registry with all base + role-specific tools.
func NewToolRegistry() *ToolRegistry {
	r := &ToolRegistry{tools: make(map[string]Tool)}

	// Core tools — the minimal set agents actually need.
	// Everything else (git, tests, package install, search, etc.) is done
	// via bash. This mirrors the just-bash / "let the agent write shell"
	// philosophy and removes the need for dozens of bespoke tool wrappers.
	r.register(toolBash())
	r.register(toolReadFile())
	r.register(toolWriteFile())

	// PR agent tools — higher-level GitHub integrations that cannot be
	// expressed as shell commands (they go through the GitHub API client).
	r.register(toolCreatePR())
	r.register(toolSetPRDescription())

	// Reviewer tools — also GitHub-API-backed.
	r.register(toolPostReviewComment())
	r.register(toolApprovePR())
	r.register(toolRequestChanges())

	return r
}

func (r *ToolRegistry) register(t Tool) {
	r.tools[t.Name] = t
}

// Get returns the tool by name and a boolean indicating whether it exists.
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// DefsForRole returns the Ollama-compatible ToolDef slice for the given role.
func (r *ToolRegistry) DefsForRole(role Role) []ToolDef {
	names := ToolNamesForRole(role)
	var defs []ToolDef
	for _, n := range names {
		t, ok := r.tools[n]
		if !ok {
			continue
		}
		defs = append(defs, ToolDef{
			Type: "function",
			Function: ToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return defs
}

// ── helpers ──────────────────────────────────────────────────────────────────

func strArg(args map[string]any, key string) string {
	if v, ok := args[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func execCmd(ctx context.Context, sandboxID string, drv sandbox.Driver, cmd string) (string, error) {
	res, err := drv.Exec(ctx, sandboxID, cmd)
	if err != nil {
		slog.Error("tool:bash exec error", "sandbox", sandboxID, "cmd", cmd, "err", err)
		return "", err
	}
	slog.Info("tool:bash result",
		"sandbox", sandboxID,
		"exit", res.ExitCode,
		"stdout_bytes", len(res.Stdout),
		"stderr_bytes", len(res.Stderr),
		"stdout_preview", truncate(res.Stdout, 400),
		"stderr_preview", truncate(res.Stderr, 400),
	)

	var parts []string
	stdout := strings.TrimRight(res.Stdout, "\n")
	stderr := strings.TrimRight(res.Stderr, "\n")
	if stdout != "" {
		parts = append(parts, "[stdout]\n"+stdout)
	}
	if stderr != "" {
		parts = append(parts, "[stderr]\n"+stderr)
	}
	parts = append(parts, fmt.Sprintf("[exit_code=%d]", res.ExitCode))
	if stdout == "" && stderr == "" && res.ExitCode == 0 {
		// Make "silent success" explicit so the model doesn't think nothing happened.
		parts = append([]string{"(command produced no output)"}, parts...)
	}
	return strings.Join(parts, "\n"), nil
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ── Core tools ───────────────────────────────────────────────────────────────

// toolBash is the swiss-army tool. Any shell command the agent can think of
// can be expressed here: git, build, test, package install, file listing,
// search, network calls, anything. Each invocation is a fresh shell — state
// (cwd, env vars, background processes) is NOT preserved between calls, so
// the agent must use absolute paths or chain commands with `&&`.
func toolBash() Tool {
	return Tool{
		Name: "bash",
		Description: "Run a shell command inside the sandbox. Each call is a fresh shell — " +
			"state is NOT preserved between calls. Use absolute paths or chain with `&&`. " +
			"Use this for everything: git clone/commit/push, running tests, installing packages, " +
			"searching files, creating directories, etc.",
		Parameters: ToolFuncParams{
			Type: "object",
			Properties: map[string]ToolParam{
				"command": {Type: "string", Description: "The shell command to execute (bash -c)"},
			},
			Required: []string{"command"},
		},
		Execute: func(ctx context.Context, sid string, drv sandbox.Driver, args map[string]any) (string, error) {
			cmd := strArg(args, "command")
			if cmd == "" {
				return "", fmt.Errorf("bash: command is required")
			}
			slog.Info("tool:bash", "sandbox", sid, "cmd", cmd)
			return execCmd(ctx, sid, drv, cmd)
		},
	}
}

func toolReadFile() Tool {
	return Tool{
		Name:        "read_file",
		Description: "Read the contents of a file from the sandbox filesystem.",
		Parameters: ToolFuncParams{
			Type: "object",
			Properties: map[string]ToolParam{
				"path": {Type: "string", Description: "Absolute path of the file to read"},
			},
			Required: []string{"path"},
		},
		Execute: func(ctx context.Context, sid string, drv sandbox.Driver, args map[string]any) (string, error) {
			path := strArg(args, "path")
			if path == "" {
				return "", fmt.Errorf("read_file: path is required")
			}
			slog.Debug("tool:read_file", "sandbox", sid, "path", path)
			return drv.ReadFile(ctx, sid, path)
		},
	}
}

func toolWriteFile() Tool {
	return Tool{
		Name:        "write_file",
		Description: "Write content to a file in the sandbox filesystem. Creates parent directories as needed.",
		Parameters: ToolFuncParams{
			Type: "object",
			Properties: map[string]ToolParam{
				"path":    {Type: "string", Description: "Absolute path of the file to write"},
				"content": {Type: "string", Description: "The file content to write"},
			},
			Required: []string{"path", "content"},
		},
		Execute: func(ctx context.Context, sid string, drv sandbox.Driver, args map[string]any) (string, error) {
			path := strArg(args, "path")
			content := strArg(args, "content")
			if path == "" {
				return "", fmt.Errorf("write_file: path is required")
			}
			slog.Debug("tool:write_file", "sandbox", sid, "path", path, "bytes", len(content))
			// Ensure parent dir exists.
			dir := path[:strings.LastIndex(path, "/")+1]
			if dir != "" && dir != "/" {
				if _, err := drv.Exec(ctx, sid, "mkdir -p "+dir); err != nil {
					return "", fmt.Errorf("write_file: mkdir: %w", err)
				}
			}
			if err := drv.WriteFile(ctx, sid, path, content); err != nil {
				return "", err
			}
			return fmt.Sprintf("wrote %d bytes to %s", len(content), path), nil
		},
	}
}

// ── PR agent tools ───────────────────────────────────────────────────────────

func toolCreatePR() Tool {
	return Tool{
		Name:        "create_pr",
		Description: "Create a GitHub pull request. (Delegates to the GitHub layer — stub until Week 9.)",
		Parameters: ToolFuncParams{
			Type: "object",
			Properties: map[string]ToolParam{
				"title": {Type: "string", Description: "PR title"},
				"body":  {Type: "string", Description: "PR body / description"},
			},
			Required: []string{"title", "body"},
		},
		Execute: func(_ context.Context, _ string, _ sandbox.Driver, args map[string]any) (string, error) {
			title := strArg(args, "title")
			body := strArg(args, "body")
			slog.Info("tool:create_pr (stub)", "title", title, "body_len", len(body))
			return "PR creation stubbed — will be wired in Week 9 (GitHub layer)", nil
		},
	}
}

func toolSetPRDescription() Tool {
	return Tool{
		Name:        "set_pr_description",
		Description: "Update the description of an existing pull request. (Stub until Week 9.)",
		Parameters: ToolFuncParams{
			Type: "object",
			Properties: map[string]ToolParam{
				"description": {Type: "string", Description: "New PR description body"},
			},
			Required: []string{"description"},
		},
		Execute: func(_ context.Context, _ string, _ sandbox.Driver, args map[string]any) (string, error) {
			slog.Info("tool:set_pr_description (stub)")
			return "PR description update stubbed — will be wired in Week 9", nil
		},
	}
}

// ── Reviewer tools ───────────────────────────────────────────────────────────

func toolPostReviewComment() Tool {
	return Tool{
		Name:        "post_review_comment",
		Description: "Post an inline review comment on a PR file. (Stub until Week 9.)",
		Parameters: ToolFuncParams{
			Type: "object",
			Properties: map[string]ToolParam{
				"path":    {Type: "string", Description: "File path the comment applies to"},
				"line":    {Type: "string", Description: "Line number"},
				"comment": {Type: "string", Description: "Review comment text"},
			},
			Required: []string{"path", "line", "comment"},
		},
		Execute: func(_ context.Context, _ string, _ sandbox.Driver, args map[string]any) (string, error) {
			slog.Info("tool:post_review_comment (stub)", "path", strArg(args, "path"))
			return "review comment stubbed — will be wired in Week 9", nil
		},
	}
}

func toolApprovePR() Tool {
	return Tool{
		Name:        "approve_pr",
		Description: "Approve the current pull request. (Stub until Week 9.)",
		Parameters: ToolFuncParams{
			Type:       "object",
			Properties: map[string]ToolParam{},
			Required:   []string{},
		},
		Execute: func(_ context.Context, _ string, _ sandbox.Driver, _ map[string]any) (string, error) {
			slog.Info("tool:approve_pr (stub)")
			return "PR approval stubbed — will be wired in Week 9", nil
		},
	}
}

func toolRequestChanges() Tool {
	return Tool{
		Name:        "request_changes",
		Description: "Request changes on the current pull request. (Stub until Week 9.)",
		Parameters: ToolFuncParams{
			Type: "object",
			Properties: map[string]ToolParam{
				"reason": {Type: "string", Description: "Reason for requesting changes"},
			},
			Required: []string{"reason"},
		},
		Execute: func(_ context.Context, _ string, _ sandbox.Driver, args map[string]any) (string, error) {
			slog.Info("tool:request_changes (stub)", "reason", strArg(args, "reason"))
			return "request-changes stubbed — will be wired in Week 9", nil
		},
	}
}
