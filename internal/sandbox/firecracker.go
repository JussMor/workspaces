package sandbox

import (
	"context"
	"errors"
)

// FirecrackerDriver implements Driver using Firecracker microVMs.
//
// TODO(forge): implement per docs/platform-plan.jsx Week 1-2 (prod path)
// Manages microVM lifecycle on Hetzner bare metal. Selected at startup
// when SANDBOX_DRIVER=firecracker. Zero callsite changes from DockerDriver.
type FirecrackerDriver struct{}

// compile-time interface assertion
var _ Driver = (*FirecrackerDriver)(nil)

// Create boots a new Firecracker microVM with the given configuration and
// returns its instance ID.
// TODO(forge): implement per docs/platform-plan.jsx Week 1-2
func (f *FirecrackerDriver) Create(_ context.Context, _ Config) (string, error) {
	return "", errors.New("firecracker driver: not implemented")
}

// Exec runs a shell command inside the microVM identified by id.
// TODO(forge): implement per docs/platform-plan.jsx Week 1-2
func (f *FirecrackerDriver) Exec(_ context.Context, _, _ string) (ExecResult, error) {
	return ExecResult{}, errors.New("firecracker driver: not implemented")
}

// WriteFile writes content to path inside the microVM.
// TODO(forge): implement per docs/platform-plan.jsx Week 1-2
func (f *FirecrackerDriver) WriteFile(_ context.Context, _, _, _ string) error {
	return errors.New("firecracker driver: not implemented")
}

// ReadFile reads the contents of path from inside the microVM.
// TODO(forge): implement per docs/platform-plan.jsx Week 1-2
func (f *FirecrackerDriver) ReadFile(_ context.Context, _, _ string) (string, error) {
	return "", errors.New("firecracker driver: not implemented")
}

// Destroy halts and removes the microVM identified by id.
// TODO(forge): implement per docs/platform-plan.jsx Week 1-2
func (f *FirecrackerDriver) Destroy(_ context.Context, _ string) error {
	return errors.New("firecracker driver: not implemented")
}

// Status returns the current lifecycle state of the microVM.
// TODO(forge): implement per docs/platform-plan.jsx Week 1-2
func (f *FirecrackerDriver) Status(_ context.Context, _ string) (Status, error) {
	return "", errors.New("firecracker driver: not implemented")
}

// IP returns the IP address of the microVM's network interface (tap device).
// TODO(forge): implement per docs/platform-plan.jsx Week 1-2
func (f *FirecrackerDriver) IP(_ context.Context, _ string) (string, error) {
	return "", errors.New("firecracker driver: not implemented")
}
