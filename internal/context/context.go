package context

import "github.com/jussmor/workspaces/internal/types"

// ProjectContext is the full execution state for a single task.
//
// The Context Engine owns and mutates this structure over the lifetime of a
// task run. Snapshots are readable by any layer; writes go through Manager.
//
// TODO(forge): implement per docs/platform-plan.jsx Week 5-6
type ProjectContext struct {
	// Identity
	RepoURL  string `json:"repo_url"`
	Branch   string `json:"branch"`
	TaskID   string `json:"task_id"`
	PRNumber int    `json:"pr_number,omitempty"`
	PRStatus string `json:"pr_status,omitempty"`

	// Repository state
	LastCommit string   `json:"last_commit"`
	FileTree   []string `json:"file_tree,omitempty"`

	// Work tracking
	Subtasks      []types.Subtask `json:"subtasks,omitempty"`
	CompletedWork []string        `json:"completed_work,omitempty"`

	// Quality signals
	TestResults  []types.TestResult    `json:"test_results,omitempty"`
	LintResults  []string              `json:"lint_results,omitempty"`
	BuildStatus  string                `json:"build_status,omitempty"`

	// Review
	ReviewComments []types.ReviewComment `json:"review_comments,omitempty"`

	// Accumulated knowledge
	Decisions        map[string]string `json:"decisions,omitempty"`
	Patterns         []string          `json:"patterns,omitempty"`
	FailedApproaches []string          `json:"failed_approaches,omitempty"`
}

// Delta describes an incremental change to apply to a ProjectContext.
// Only non-zero fields are applied.
//
// TODO(forge): implement per docs/platform-plan.jsx Week 5-6
type Delta struct {
	CompletedWork   []string          `json:"completed_work,omitempty"`
	NewSubtasks     []types.Subtask   `json:"new_subtasks,omitempty"`
	TestResults     []types.TestResult `json:"test_results,omitempty"`
	LintResults     []string          `json:"lint_results,omitempty"`
	BuildStatus     string            `json:"build_status,omitempty"`
	NewDecisions    map[string]string `json:"new_decisions,omitempty"`
	NewPatterns     []string          `json:"new_patterns,omitempty"`
	FailedApproach  string            `json:"failed_approach,omitempty"`
	PRNumber        int               `json:"pr_number,omitempty"`
	PRStatus        string            `json:"pr_status,omitempty"`
}

// Event is a change notification broadcast to Manager subscribers.
//
// TODO(forge): implement per docs/platform-plan.jsx Week 5-6
type Event struct {
	TaskID string         `json:"task_id"`
	Delta  Delta          `json:"delta"`
}
