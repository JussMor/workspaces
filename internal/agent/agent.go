package agent

import (
	"context"
	"errors"

	"github.com/jussmor/workspaces/internal/sandbox"
	"github.com/jussmor/workspaces/internal/types"
)

// Engine is the Agent Engine — LAYER 02 of FORGE.
//
// It receives a Task from the Coordinator, plans and executes subtasks
// using LLM reasoning, and delegates low-level operations to Tools.
//
// TODO(forge): implement per docs/platform-plan.jsx Week 3-4
type Engine struct {
	// Sandbox is the execution environment the agent uses to run commands.
	Sandbox sandbox.Driver

	// Tools provides the agent's action set (read, write, git, search, …).
	Tools *Tools
}

// Run executes a Task to completion, driving subtask planning and execution.
// Returns when the task is done or context is cancelled.
// TODO(forge): implement per docs/platform-plan.jsx Week 3-4
func (e *Engine) Run(_ context.Context, _ types.Task) error {
	return errors.New("agent engine: not implemented")
}
