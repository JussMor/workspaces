// Package agent is LAYER 02 of FORGE — the Agent Engine.
//
// Week 3-4 of the platform build. Drives autonomous code editing using
// an LLM (Ollama, local). The Engine receives a Task, breaks it into
// subtasks, calls the appropriate Tools, and reports results back to the
// Coordinator.
//
// The agent is stateless between tasks; all persistent state lives in the
// Context Engine (internal/context).
package agent
