package sandbox

import (
	"errors"
	"time"
)

// Record tracks a live sandbox instance managed by the Registry.
type Record struct {
	ID        string
	IP        string
	Status    Status
	AgentRole string
	CreatedAt time.Time
}

// Registry tracks all active sandbox instances across the platform.
//
// TODO(forge): implement per docs/platform-plan.jsx Week 1-2
// Provides an in-memory (and eventually persistent) index of running
// sandboxes so the Coordinator can route work to the right instance.
type Registry struct{}

// Track registers an active sandbox instance in the registry.
// TODO(forge): implement per docs/platform-plan.jsx Week 1-2
func (r *Registry) Track(_ string, _ Driver) error {
	return errors.New("sandbox registry: not implemented")
}

// List returns all currently tracked sandbox records.
// TODO(forge): implement per docs/platform-plan.jsx Week 1-2
func (r *Registry) List() ([]Record, error) {
	return nil, errors.New("sandbox registry: not implemented")
}
