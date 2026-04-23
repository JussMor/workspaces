package agent

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jussmor/workspaces/internal/sandbox"
	"github.com/jussmor/workspaces/internal/types"
)

const (
	// MaxIterations is the hard cap on ReAct loop iterations per Run.
	MaxIterations = 50

	// maxToolRetries is how many times a transient tool failure is retried.
	maxToolRetries = 3
)

// Engine is the Agent Engine — LAYER 02 of FORGE.
//
// It receives a Task from the Coordinator, drives a ReAct (Reason + Act) loop
// using an Ollama-powered LLM, and delegates low-level operations to tools
// that run inside a sandbox.
type Engine struct {
	Sandbox    sandbox.Driver
	Ollama     *OllamaClient
	Tools      *ToolRegistry
	Accounting *Accounting
}

// NewEngine constructs an Engine with all dependencies wired in.
func NewEngine(drv sandbox.Driver, ollama *OllamaClient) *Engine {
	return &Engine{
		Sandbox:    drv,
		Ollama:     ollama,
		Tools:      NewToolRegistry(),
		Accounting: NewAccounting(),
	}
}

// Run executes a Task inside a sandbox using the ReAct loop.
//
// Flow: think → tool call → observe → repeat — up to MaxIterations.
// Returns when the LLM produces a final answer (no tool calls) or the
// iteration limit is reached.
func (e *Engine) Run(ctx context.Context, task types.Task, role Role, sandboxID string) (string, error) {
	model := ModelForRole(role)
	toolDefs := e.Tools.DefsForRole(role)

	slog.Info("agent.Run starting",
		"task_id", task.ID,
		"role", role,
		"model", model,
		"sandbox", sandboxID,
		"tools", len(toolDefs),
	)

	// Build initial conversation.
	messages := []ChatMessage{
		{Role: "system", Content: roleSystemPrompt(role)},
		{Role: "user", Content: buildUserPrompt(task)},
	}

	stats := RunStats{
		Role:      role,
		Model:     model,
		StartedAt: time.Now(),
	}

	var finalAnswer string

	for i := 0; i < MaxIterations; i++ {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("agent: context cancelled at iteration %d: %w", i, err)
		}

		stats.Iterations = i + 1

		slog.Info("agent.ReAct", "iteration", i+1, "messages", len(messages))

		resp, err := e.Ollama.Chat(ctx, ChatRequest{
			Model:    model,
			Messages: messages,
			Tools:    toolDefs,
			Options:  &ModelOptions{Temperature: 0.1, NumCtx: 8192},
		})
		if err != nil {
			return "", fmt.Errorf("agent: ollama chat error at iteration %d: %w", i, err)
		}

		// Accumulate token counts.
		stats.PromptTokens += resp.PromptEvalCount
		stats.CompletionTokens += resp.EvalCount
		stats.TotalTokens += resp.PromptEvalCount + resp.EvalCount

		// Append assistant message to conversation history.
		messages = append(messages, resp.Message)

		// No tool calls → this is the final answer.
		if len(resp.Message.ToolCalls) == 0 {
			finalAnswer = resp.Message.Content
			slog.Info("agent.ReAct complete",
				"iteration", i+1,
				"reason", "final_answer",
				"answer_len", len(finalAnswer),
			)
			break
		}

		// Process each tool call.
		for _, tc := range resp.Message.ToolCalls {
			stats.ToolCalls++

			toolName := tc.Function.Name
			toolArgs := tc.Function.Arguments

			slog.Info("agent.tool_call",
				"tool", toolName,
				"args", toolArgs,
			)

			result, err := e.executeToolWithRetry(ctx, sandboxID, toolName, toolArgs)
			if err != nil {
				slog.Warn("agent.tool_error", "tool", toolName, "err", err)
				result = fmt.Sprintf("ERROR: %v", err)
			}

			// Feed tool result back as a "tool" message.
			messages = append(messages, ChatMessage{
				Role:    "tool",
				Content: fmt.Sprintf("[%s] %s", toolName, result),
			})
		}
	}

	if finalAnswer == "" && stats.Iterations >= MaxIterations {
		finalAnswer = "agent reached maximum iterations without a final answer"
		slog.Warn("agent.max_iterations", "task_id", task.ID, "iterations", MaxIterations)
	}

	stats.FinishedAt = time.Now()
	stats.WallTime = stats.FinishedAt.Sub(stats.StartedAt)
	e.Accounting.Record(stats)

	slog.Info("agent.Run finished",
		"task_id", task.ID,
		"iterations", stats.Iterations,
		"tool_calls", stats.ToolCalls,
		"prompt_tokens", stats.PromptTokens,
		"completion_tokens", stats.CompletionTokens,
		"wall_time", stats.WallTime,
	)

	return finalAnswer, nil
}

// executeToolWithRetry calls a tool, retrying up to maxToolRetries on error.
func (e *Engine) executeToolWithRetry(ctx context.Context, sandboxID, toolName string, args map[string]any) (string, error) {
	tool, ok := e.Tools.Get(toolName)
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}

	var lastErr error
	for attempt := 1; attempt <= maxToolRetries; attempt++ {
		result, err := tool.Execute(ctx, sandboxID, e.Sandbox, args)
		if err == nil {
			return result, nil
		}
		lastErr = err
		slog.Warn("agent.tool_retry",
			"tool", toolName,
			"attempt", attempt,
			"err", err,
		)
		if attempt < maxToolRetries {
			// Brief back-off before retry.
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(attempt) * 500 * time.Millisecond):
			}
		}
	}
	return "", fmt.Errorf("tool %s failed after %d attempts: %w", toolName, maxToolRetries, lastErr)
}

// buildUserPrompt constructs the initial user message from a Task.
func buildUserPrompt(task types.Task) string {
	prompt := fmt.Sprintf("## Task: %s\n\n%s", task.Title, task.Description)
	if len(task.Subtasks) > 0 {
		prompt += "\n\n### Subtasks:\n"
		for i, st := range task.Subtasks {
			prompt += fmt.Sprintf("%d. [%s] %s\n", i+1, st.Status, st.Description)
		}
	}
	prompt += "\n\nWork through this step by step. Use the tools to read files, write code, run commands, and commit your changes. When you are done, provide a summary of what you accomplished."
	return prompt
}

// RunSingle is a convenience that creates a sandbox, runs the agent, and
// destroys the sandbox. Used for demos and single-shot tasks.
func (e *Engine) RunSingle(ctx context.Context, task types.Task, role Role, sandboxCfg sandbox.Config) (string, error) {
	sandboxID, err := e.Sandbox.Create(ctx, sandboxCfg)
	if err != nil {
		return "", fmt.Errorf("agent: create sandbox: %w", err)
	}
	defer func() {
		if derr := e.Sandbox.Destroy(context.Background(), sandboxID); derr != nil {
			slog.Warn("agent: sandbox cleanup failed", "sandbox", sandboxID, "err", derr)
		}
	}()

	return e.Run(ctx, task, role, sandboxID)
}
