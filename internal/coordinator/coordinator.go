package coordinator

import (
	"context"
	"errors"

	forgectx "github.com/jussmor/workspaces/internal/context"
	"github.com/jussmor/workspaces/internal/agent"
	"github.com/jussmor/workspaces/internal/github"
	"github.com/jussmor/workspaces/internal/sandbox"
)

// Coordinator is LAYER 04 — it orchestrates the full FORGE task pipeline.
//
// It wires the Agent Engine, Context Manager, GitHub Client, and Sandbox
// Driver into a coherent task execution flow. All business orchestration
// logic lives here; individual layers remain independently testable.
//
// TODO(forge): implement per docs/platform-plan.jsx Week 7-8
type Coordinator struct {
	Agent   *agent.Engine
	Context *forgectx.Manager
	GitHub  *github.Client
	Sandbox sandbox.Driver
}

// RunPipeline executes a full task pipeline end-to-end:
// plan → agent loop → tests → PR creation → review loop → merge.
// TODO(forge): implement per docs/platform-plan.jsx Week 7-8
func (c *Coordinator) RunPipeline(_ context.Context, taskID string) error {
	return errors.New("coordinator: not implemented")
}

// RunFixLoop drives the repair cycle after failed tests or review feedback.
// Reads the current context, instructs the agent to apply fixes, re-runs
// checks, and repeats until passing or a max-iteration limit is reached.
// TODO(forge): implement per docs/platform-plan.jsx Week 7-8
func (c *Coordinator) RunFixLoop(_ context.Context, _ *forgectx.Manager) error {
	return errors.New("coordinator: not implemented")
}

// Merge triggers the GitHub merge flow for the PR associated with taskID.
// TODO(forge): implement per docs/platform-plan.jsx Week 7-8
func (c *Coordinator) Merge(_ context.Context, _ string) error {
	return errors.New("coordinator: not implemented")
}
