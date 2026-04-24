package agent

// Role identifies the kind of agent and determines which LLM model and
// tool subset it uses.
type Role string

const (
	RolePlanner  Role = "planner"
	RoleCoder    Role = "coder"
	RoleReviewer Role = "reviewer"
	RoleTester   Role = "tester"
	RolePRAgent  Role = "pr_agent"
)

// roleModel maps each Role to the Ollama model tag.
//
// Code-intensive roles use Qwen3-coder:30b; reasoning/writing roles use gemma4.
var roleModel = map[Role]string{
	RolePlanner:  "gemma4",
	RoleCoder:    "qwen3-coder:30b",
	RoleReviewer: "gemma4",
	RoleTester:   "qwen3-coder:30b",
	RolePRAgent:  "gemma4",
}

// ModelForRole returns the Ollama model tag for the given role.
// Falls back to gemma4 for unknown roles.
func ModelForRole(r Role) string {
	if m, ok := roleModel[r]; ok {
		return m
	}
	return "gemma4"
}

// roleSystemPrompt returns the system prompt for a given role.
func roleSystemPrompt(r Role) string {
	switch r {
	case RolePlanner:
		return `You are a senior software architect and planner. Your job is to break a
high-level task into concrete, actionable subtasks that a coder agent can execute
one at a time. Be specific about files to create or modify, function signatures,
and test expectations. Output a numbered list of subtasks.`

	case RoleCoder:
		return "You are an expert software engineer working inside a persistent Linux sandbox (Debian).\n" +
			"The sandbox SURVIVES between calls — do not re-clone or re-install anything that is already there.\n" +
			"\n" +
			"TOOLS (this is the complete list — no others exist):\n" +
			"  * bash(command)             — run any shell command as user `forge` in /home/forge/workspace.\n" +
			"  * read_file(path)           — read a file's contents.\n" +
			"  * write_file(path, content) — overwrite/create a file with content.\n" +
			"\n" +
			"ENVIRONMENT:\n" +
			"  * User: `forge` (uid 1000). Passwordless sudo is available — prefix commands with `sudo` when root is needed.\n" +
			"  * HOME = /home/forge.  Workspace = /home/forge/workspace (also your cwd on every bash call).\n" +
			"  * Pre-installed: git, gh (GitHub CLI), go, node 20 + npm/pnpm/yarn, python3 + pip, make, build-essential, jq, rg, curl, wget, kubectl, docker CLI, awscli.\n" +
			"  * To install anything else: `sudo apt-get update && sudo apt-get install -y <pkg>` (or pip/npm/go install for language deps).\n" +
			"  * Use `gh` for PRs/pushes/auth when the task involves GitHub (it reads GITHUB_TOKEN from env if set).\n" +
			"\n" +
			"WORKFLOW:\n" +
			"  1. If the task mentions a repo and /home/forge/workspace/.git does not exist, clone it INTO the workspace: `git clone <url> .` (from /home/forge/workspace). Otherwise reuse what's there.\n" +
			"  2. Create a branch, edit files, run tests, commit, push.\n" +
			"  3. If the task requires running a server (web app, API, etc.), start it in the background: `nohup <cmd> &>/tmp/server.log &` then confirm it is up with `curl -s http://localhost:<port>/`.\n" +
			"  4. When you start a server, always report the port in your final answer so the user can access it via the proxy URL.\n" +
			"  5. For one-shot checks use: go test, go build, npm test (not go run / npm run dev as foreground processes).\n" +
			"  6. When the task is fully complete, reply with a plain-text summary (files changed + commands run + test results + server URL if applicable) and STOP calling tools."

	case RoleReviewer:
		return `You are a meticulous code reviewer. Read the diff carefully and check for
bugs, security issues, style problems, and missing tests. Use the provided tools
to inspect code, run tests, and leave review comments. Be specific and actionable.`

	case RoleTester:
		return `You are a test engineer. Your job is to write comprehensive tests for the
code changes made by other agents. Use the tools to read the source, create test
files, and run the test suite. Ensure edge cases are covered.`

	case RolePRAgent:
		return `You are a pull request agent. Your job is to create well-structured PRs
with descriptive titles and bodies. Push the branch, create the PR, and set a
clear description that summarises the changes, links to the task, and lists the
test results.`

	default:
		return `You are a helpful AI assistant with access to a sandbox environment.
Use the provided tools to complete the task.`
	}
}

// baseToolNames lists tools available to every role.
// The set is deliberately minimal: bash for everything imperative, plus
// structured file I/O. The agent composes its own workflow.
var baseToolNames = []string{
	"bash",
	"read_file",
	"write_file",
}

// roleExtraTools lists additional tools per role beyond the base set.
// Higher-level integrations (PR creation, review comments) that cannot be
// expressed as plain shell commands live here.
var roleExtraTools = map[Role][]string{
	RolePRAgent: {
		"create_pr",
		"set_pr_description",
	},
	RoleReviewer: {
		"post_review_comment",
		"approve_pr",
		"request_changes",
	},
}

// ToolNamesForRole returns the full set of tool names available to the role.
func ToolNamesForRole(r Role) []string {
	names := make([]string, len(baseToolNames))
	copy(names, baseToolNames)
	if extras, ok := roleExtraTools[r]; ok {
		names = append(names, extras...)
	}
	return names
}
