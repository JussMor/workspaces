package agent

import (
	"sync"
	"time"
)

// RunStats records token counts and wall-clock time for a single agent run.
type RunStats struct {
	Role            Role          `json:"role"`
	Model           string        `json:"model"`
	Iterations      int           `json:"iterations"`
	PromptTokens    int           `json:"prompt_tokens"`
	CompletionTokens int          `json:"completion_tokens"`
	TotalTokens     int           `json:"total_tokens"`
	ToolCalls       int           `json:"tool_calls"`
	WallTime        time.Duration `json:"wall_time"`
	StartedAt       time.Time     `json:"started_at"`
	FinishedAt      time.Time     `json:"finished_at"`
}

// Accounting tracks token/cost stats across multiple agent runs.
// Thread-safe.
type Accounting struct {
	mu   sync.Mutex
	runs []RunStats
}

// NewAccounting returns an empty Accounting tracker.
func NewAccounting() *Accounting {
	return &Accounting{}
}

// Record appends a completed RunStats entry.
func (a *Accounting) Record(s RunStats) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.runs = append(a.runs, s)
}

// Runs returns a copy of all recorded stats.
func (a *Accounting) Runs() []RunStats {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]RunStats, len(a.runs))
	copy(out, a.runs)
	return out
}

// Summary returns aggregate counts across all runs.
func (a *Accounting) Summary() RunStats {
	a.mu.Lock()
	defer a.mu.Unlock()
	var s RunStats
	for _, r := range a.runs {
		s.Iterations += r.Iterations
		s.PromptTokens += r.PromptTokens
		s.CompletionTokens += r.CompletionTokens
		s.TotalTokens += r.TotalTokens
		s.ToolCalls += r.ToolCalls
		s.WallTime += r.WallTime
	}
	return s
}
