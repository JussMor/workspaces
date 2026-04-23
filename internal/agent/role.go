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

// roleModel maps each Role to the default Ollama model tag.
var roleModel = map[Role]string{
	RolePlanner:  "gemma4",
	RoleCoder:    "gemma4",
	RoleReviewer: "gemma4",
	RoleTester:   "gemma4",
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
		return `You are an expert software engineer. You write clean, idiomatic code and
use the provided tools to read files, write files, run commands, and manage git
inside a sandbox environment. Think step-by-step: read existing code first,
plan your change, implement it, run tests, commit. Always commit your work.`

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
var baseToolNames = []string{
	"run_command",
	"read_file",
	"write_file",
	"list_files",
	"git_status",
	"git_diff",
	"git_commit",
	"search_code",
	"install_deps",
	"run_tests",
	"create_branch",
}

// roleExtraTools lists additional tools per role beyond the base set.
var roleExtraTools = map[Role][]string{
	RolePRAgent: {
		"create_pr",
		"push_branch",
		"set_pr_description",
	},
	RoleReviewer: {
		"post_review_comment",
		"approve_pr",
		"request_changes",
	},
	RoleTester: {
		"create_test_file",
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
