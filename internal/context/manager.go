package context

import "errors"

// Manager owns the ProjectContext for a task and mediates all reads/writes.
//
// Callers apply incremental Deltas via Apply; the Manager broadcasts Events
// to all active subscribers (dashboard WebSocket consumers).
//
// TODO(forge): implement per docs/platform-plan.jsx Week 5-6
type Manager struct{}

// Apply applies a Delta to the current ProjectContext.
// The change is broadcast to all subscribers after it is applied.
// TODO(forge): implement per docs/platform-plan.jsx Week 5-6
func (m *Manager) Apply(_ Delta) error {
	return errors.New("context manager: not implemented")
}

// Snapshot returns a read-only copy of the current ProjectContext.
// TODO(forge): implement per docs/platform-plan.jsx Week 5-6
func (m *Manager) Snapshot() ProjectContext {
	return ProjectContext{}
}

// Subscribe returns a channel that receives an Event for each Delta applied.
// The caller must drain the channel or risk blocking the Manager.
// TODO(forge): implement per docs/platform-plan.jsx Week 5-6
func (m *Manager) Subscribe() <-chan Event {
	ch := make(chan Event)
	close(ch) // stub: closed channel signals "no events yet"
	return ch
}
