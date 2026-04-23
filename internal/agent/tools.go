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

	// Base tools — available to every role.
	r.register(toolRunCommand())
	r.register(toolReadFile())
	r.register(toolWriteFile())
	r.register(toolListFiles())
	r.register(toolGitStatus())
	r.register(toolGitDiff())
	r.register(toolGitCommit())
	r.register(toolSearchCode())
	r.register(toolInstallDeps())
	r.register(toolRunTests())
	r.register(toolCreateBranch())

	// PR agent tools.
	r.register(toolCreatePR())
	r.register(toolPushBranch())
	r.register(toolSetPRDescription())

	// Reviewer tools.
	r.register(toolPostReviewComment())
	r.register(toolApprovePR())
	r.register(toolRequestChanges())

	// Tester tools.
	r.register(toolCreateTestFile())

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
		return "", err
	}
	out := strings.TrimSpace(res.Stdout)
	if res.Stderr != "" {
		out += "\n[stderr] " + strings.TrimSpace(res.Stderr)
	}
	if res.ExitCode != 0 {
		out += fmt.Sprintf("\n[exit_code=%d]", res.ExitCode)
	}
	return out, nil
}

// ── Base tools ───────────────────────────────────────────────────────────────

func toolRunCommand() Tool {
	return Tool{
		Name:        "run_command",
		Description: "Execute a shell command in the sandbox and return combined stdout/stderr.",
		Parameters: ToolFuncParams{
			Type: "object",
			Properties: map[string]ToolParam{
				"command": {Type: "string", Description: "The shell command to execute"},
			},
			Required: []string{"command"},
		},
		Execute: func(ctx context.Context, sid string, drv sandbox.Driver, args map[string]any) (string, error) {
			cmd := strArg(args, "command")
			if cmd == "" {
				return "", fmt.Errorf("run_command: command is required")
			}
			slog.Debug("tool:run_command", "sandbox", sid, "cmd", cmd)
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

func toolListFiles() Tool {
	return Tool{
		Name:        "list_files",
		Description: "List files and directories in a directory inside the sandbox.",
		Parameters: ToolFuncParams{
			Type: "object",
			Properties: map[string]ToolParam{
				"directory": {Type: "string", Description: "Directory path to list (default: current working dir)"},
			},
			Required: []string{},
		},
		Execute: func(ctx context.Context, sid string, drv sandbox.Driver, args map[string]any) (string, error) {
			dir := strArg(args, "directory")
			if dir == "" {
				dir = "."
			}
			slog.Debug("tool:list_files", "sandbox", sid, "dir", dir)
			return execCmd(ctx, sid, drv, "find "+dir+" -maxdepth 2 -type f -o -type d | head -200")
		},
	}
}

func toolGitStatus() Tool {
	return Tool{
		Name:        "git_status",
		Description: "Show the working tree status (git status).",
		Parameters: ToolFuncParams{
			Type:       "object",
			Properties: map[string]ToolParam{},
			Required:   []string{},
		},
		Execute: func(ctx context.Context, sid string, drv sandbox.Driver, _ map[string]any) (string, error) {
			slog.Debug("tool:git_status", "sandbox", sid)
			return execCmd(ctx, sid, drv, "git status")
		},
	}
}

func toolGitDiff() Tool {
	return Tool{
		Name:        "git_diff",
		Description: "Show the diff of uncommitted changes (git diff).",
		Parameters: ToolFuncParams{
			Type:       "object",
			Properties: map[string]ToolParam{},
			Required:   []string{},
		},
		Execute: func(ctx context.Context, sid string, drv sandbox.Driver, _ map[string]any) (string, error) {
			slog.Debug("tool:git_diff", "sandbox", sid)
			return execCmd(ctx, sid, drv, "git diff")
		},
	}
}

func toolGitCommit() Tool {
	return Tool{
		Name:        "git_commit",
		Description: "Stage all changes and commit with the given message.",
		Parameters: ToolFuncParams{
			Type: "object",
			Properties: map[string]ToolParam{
				"message": {Type: "string", Description: "Commit message"},
			},
			Required: []string{"message"},
		},
		Execute: func(ctx context.Context, sid string, drv sandbox.Driver, args map[string]any) (string, error) {
			msg := strArg(args, "message")
			if msg == "" {
				return "", fmt.Errorf("git_commit: message is required")
			}
			slog.Debug("tool:git_commit", "sandbox", sid, "message", msg)
			// Sanitise single quotes in the commit message.
			safe := strings.ReplaceAll(msg, "'", "'\\''")
			return execCmd(ctx, sid, drv, "git add -A && git commit -m '"+safe+"'")
		},
	}
}

func toolSearchCode() Tool {
	return Tool{
		Name:        "search_code",
		Description: "Search for a pattern in the codebase using grep.",
		Parameters: ToolFuncParams{
			Type: "object",
			Properties: map[string]ToolParam{
				"pattern": {Type: "string", Description: "Search pattern (grep -rn)"},
			},
			Required: []string{"pattern"},
		},
		Execute: func(ctx context.Context, sid string, drv sandbox.Driver, args map[string]any) (string, error) {
			pattern := strArg(args, "pattern")
			if pattern == "" {
				return "", fmt.Errorf("search_code: pattern is required")
			}
			slog.Debug("tool:search_code", "sandbox", sid, "pattern", pattern)
			safe := strings.ReplaceAll(pattern, "'", "'\\''")
			return execCmd(ctx, sid, drv, "grep -rn '"+safe+"' . --include='*.go' --include='*.js' --include='*.ts' --include='*.py' --include='*.rs' | head -100")
		},
	}
}

func toolInstallDeps() Tool {
	return Tool{
		Name:        "install_deps",
		Description: "Detect the project language and install dependencies (go mod tidy, npm install, etc.).",
		Parameters: ToolFuncParams{
			Type:       "object",
			Properties: map[string]ToolParam{},
			Required:   []string{},
		},
		Execute: func(ctx context.Context, sid string, drv sandbox.Driver, _ map[string]any) (string, error) {
			slog.Debug("tool:install_deps", "sandbox", sid)
			// Auto-detect by looking for common manifest files.
			script := `
if [ -f go.mod ]; then
  go mod tidy 2>&1
elif [ -f package.json ]; then
  npm install 2>&1
elif [ -f requirements.txt ]; then
  pip install -r requirements.txt 2>&1
elif [ -f Cargo.toml ]; then
  cargo fetch 2>&1
else
  echo "no recognised dependency manifest found"
fi`
			return execCmd(ctx, sid, drv, script)
		},
	}
}

func toolRunTests() Tool {
	return Tool{
		Name:        "run_tests",
		Description: "Detect the project language and run the test suite.",
		Parameters: ToolFuncParams{
			Type:       "object",
			Properties: map[string]ToolParam{},
			Required:   []string{},
		},
		Execute: func(ctx context.Context, sid string, drv sandbox.Driver, _ map[string]any) (string, error) {
			slog.Debug("tool:run_tests", "sandbox", sid)
			script := `
if [ -f go.mod ]; then
  go test ./... 2>&1
elif [ -f package.json ]; then
  npm test 2>&1
elif [ -f requirements.txt ] || [ -f setup.py ]; then
  python -m pytest 2>&1
elif [ -f Cargo.toml ]; then
  cargo test 2>&1
else
  echo "no recognised test runner found"
fi`
			return execCmd(ctx, sid, drv, script)
		},
	}
}

func toolCreateBranch() Tool {
	return Tool{
		Name:        "create_branch",
		Description: "Create and switch to a new git branch.",
		Parameters: ToolFuncParams{
			Type: "object",
			Properties: map[string]ToolParam{
				"name": {Type: "string", Description: "Branch name to create"},
			},
			Required: []string{"name"},
		},
		Execute: func(ctx context.Context, sid string, drv sandbox.Driver, args map[string]any) (string, error) {
			name := strArg(args, "name")
			if name == "" {
				return "", fmt.Errorf("create_branch: name is required")
			}
			slog.Debug("tool:create_branch", "sandbox", sid, "branch", name)
			safe := strings.ReplaceAll(name, "'", "'\\''")
			return execCmd(ctx, sid, drv, "git checkout -b '"+safe+"'")
		},
	}
}

// ── PR agent tools ───────────────────────────────────────────────────────────

func toolPushBranch() Tool {
	return Tool{
		Name:        "push_branch",
		Description: "Push the current branch to the remote origin.",
		Parameters: ToolFuncParams{
			Type:       "object",
			Properties: map[string]ToolParam{},
			Required:   []string{},
		},
		Execute: func(ctx context.Context, sid string, drv sandbox.Driver, _ map[string]any) (string, error) {
			slog.Debug("tool:push_branch", "sandbox", sid)
			return execCmd(ctx, sid, drv, "git push -u origin HEAD")
		},
	}
}

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

// ── Tester tools ─────────────────────────────────────────────────────────────

func toolCreateTestFile() Tool {
	return Tool{
		Name:        "create_test_file",
		Description: "Create a test file in the sandbox. Convenience wrapper over write_file for test files.",
		Parameters: ToolFuncParams{
			Type: "object",
			Properties: map[string]ToolParam{
				"path":    {Type: "string", Description: "Path for the test file"},
				"content": {Type: "string", Description: "Test file content"},
			},
			Required: []string{"path", "content"},
		},
		Execute: func(ctx context.Context, sid string, drv sandbox.Driver, args map[string]any) (string, error) {
			path := strArg(args, "path")
			content := strArg(args, "content")
			if path == "" {
				return "", fmt.Errorf("create_test_file: path is required")
			}
			slog.Debug("tool:create_test_file", "sandbox", sid, "path", path)
			dir := path[:strings.LastIndex(path, "/")+1]
			if dir != "" && dir != "/" {
				if _, err := drv.Exec(ctx, sid, "mkdir -p "+dir); err != nil {
					return "", fmt.Errorf("create_test_file: mkdir: %w", err)
				}
			}
			if err := drv.WriteFile(ctx, sid, path, content); err != nil {
				return "", err
			}
			return fmt.Sprintf("test file written: %s (%d bytes)", path, len(content)), nil
		},
	}
}
