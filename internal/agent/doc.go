// Package agent is LAYER 02 of FORGE — the Agent Engine.
//
// Week 3-4 of the platform build. Drives autonomous code editing using
// an LLM (Ollama, local). The Engine receives a Task, drives a ReAct
// (Reason + Act) loop with configurable tools, and reports results back
// to the Coordinator.
//
// Key components:
//
//   - Engine: owns the ReAct loop (think → tool call → observe → repeat)
//   - OllamaClient: HTTP client for the local Ollama LLM API
//   - ToolRegistry: complete set of callable tools (base + role-specific)
//   - Role: determines which LLM model and tool subset an agent uses
//   - Accounting: per-run token and wall-time tracking
//
// The agent is stateless between tasks; all persistent state lives in the
// Context Engine (internal/context).
package agent
