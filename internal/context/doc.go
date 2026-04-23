// Package context is LAYER 03 of FORGE — the Context Engine.
//
// Week 5-6 of the platform build. Maintains the evolving state of a task's
// execution context: repo metadata, subtask progress, test/lint results,
// PR state, and accumulated decisions. The Manager accepts incremental Delta
// updates from the Agent and the Coordinator, and broadcasts change Events
// to any subscriber (e.g. the dashboard WebSocket stream).
//
// Note: this package re-exports a Context type and Manager; callers should
// avoid importing Go's standard "context" package under the same alias.
package context
