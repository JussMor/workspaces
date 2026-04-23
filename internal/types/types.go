// Package types defines shared domain types used across all FORGE layers.
package types

import "time"

// Status represents the lifecycle state of a task or sandbox.
type Status string

const (
	StatusPending  Status = "pending"
	StatusRunning  Status = "running"
	StatusDone     Status = "done"
	StatusFailed   Status = "failed"
	StatusCanceled Status = "canceled"
)

// NotificationType classifies events surfaced to the dashboard.
type NotificationType string

const (
	NotificationInfo    NotificationType = "info"
	NotificationSuccess NotificationType = "success"
	NotificationWarning NotificationType = "warning"
	NotificationError   NotificationType = "error"
)

// Task is the top-level unit of work dispatched by the Coordinator.
type Task struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Status      Status      `json:"status"`
	Subtasks    []Subtask   `json:"subtasks,omitempty"`
	PRNumber    int         `json:"pr_number,omitempty"`
	PRStatus    string      `json:"pr_status,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// Subtask is a unit of work within a Task, executed by the Agent Engine.
type Subtask struct {
	ID          string    `json:"id"`
	TaskID      string    `json:"task_id"`
	Description string    `json:"description"`
	Status      Status    `json:"status"`
	Result      ExecResult `json:"result,omitempty"`
}

// ExecResult captures the output of a sandbox command execution.
type ExecResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// Notification is a real-time event pushed to the dashboard via WebSocket.
type Notification struct {
	ID        string           `json:"id"`
	Type      NotificationType `json:"type"`
	Message   string           `json:"message"`
	TaskID    string           `json:"task_id,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
}

// ReviewComment is a code-review comment fetched from GitHub.
type ReviewComment struct {
	ID       int64  `json:"id"`
	Body     string `json:"body"`
	Path     string `json:"path"`
	Line     int    `json:"line"`
	Resolved bool   `json:"resolved"`
}

// TestResult captures the output of a test run in a sandbox.
type TestResult struct {
	Passed  bool   `json:"passed"`
	Output  string `json:"output"`
	Elapsed string `json:"elapsed"`
}

// Config holds runtime configuration for FORGE, sourced from env vars.
type Config struct {
	Port          string `json:"port"`
	SandboxDriver string `json:"sandbox_driver"`
	GitHubToken   string `json:"-"`
	OllamaURL     string `json:"ollama_url"`
}
