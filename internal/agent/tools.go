package agent

import (
	"context"
	"errors"
)

// Tools exposes the action set available to the Agent Engine.
//
// Each method corresponds to a capability the agent can invoke during task
// execution. All methods are stubs until Week 3-4 implementation.
//
// TODO(forge): implement per docs/platform-plan.jsx Week 3-4
type Tools struct{}

// RunCommand executes a shell command in the agent's sandbox.
// TODO(forge): implement per docs/platform-plan.jsx Week 3-4
func (t *Tools) RunCommand(_ context.Context, _ string) (string, error) {
	return "", errors.New("tools: not implemented")
}

// ReadFile reads a file from the sandbox filesystem.
// TODO(forge): implement per docs/platform-plan.jsx Week 3-4
func (t *Tools) ReadFile(_ context.Context, _ string) (string, error) {
	return "", errors.New("tools: not implemented")
}

// WriteFile writes content to a file in the sandbox filesystem.
// TODO(forge): implement per docs/platform-plan.jsx Week 3-4
func (t *Tools) WriteFile(_ context.Context, _, _ string) error {
	return errors.New("tools: not implemented")
}

// GitStatus returns the current git status output from the sandbox.
// TODO(forge): implement per docs/platform-plan.jsx Week 3-4
func (t *Tools) GitStatus(_ context.Context) (string, error) {
	return "", errors.New("tools: not implemented")
}

// GitDiff returns the git diff output from the sandbox.
// TODO(forge): implement per docs/platform-plan.jsx Week 3-4
func (t *Tools) GitDiff(_ context.Context) (string, error) {
	return "", errors.New("tools: not implemented")
}

// GitCommit commits staged changes with the provided message.
// TODO(forge): implement per docs/platform-plan.jsx Week 3-4
func (t *Tools) GitCommit(_ context.Context, _ string) error {
	return errors.New("tools: not implemented")
}

// SearchCode performs a code search using ripgrep in the sandbox.
// TODO(forge): implement per docs/platform-plan.jsx Week 3-4
func (t *Tools) SearchCode(_ context.Context, _ string) (string, error) {
	return "", errors.New("tools: not implemented")
}
