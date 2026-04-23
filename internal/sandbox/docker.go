package sandbox

import (
	"context"
	"errors"
)

// DockerDriver implements Driver using Docker containers.
//
// TODO(forge): implement per docs/platform-plan.jsx Week 1-2
// Manages container lifecycle via the Docker Engine API.
type DockerDriver struct{}

// compile-time interface assertion
var _ Driver = (*DockerDriver)(nil)

// Create starts a new Docker container with the given configuration and
// returns its container ID.
// TODO(forge): implement per docs/platform-plan.jsx Week 1-2
func (d *DockerDriver) Create(_ context.Context, _ Config) (string, error) {
	return "", errors.New("docker driver: not implemented")
}

// Exec runs a shell command inside the container identified by id.
// TODO(forge): implement per docs/platform-plan.jsx Week 1-2
func (d *DockerDriver) Exec(_ context.Context, _, _ string) (ExecResult, error) {
	return ExecResult{}, errors.New("docker driver: not implemented")
}

// WriteFile writes content to path inside the container.
// TODO(forge): implement per docs/platform-plan.jsx Week 1-2
func (d *DockerDriver) WriteFile(_ context.Context, _, _, _ string) error {
	return errors.New("docker driver: not implemented")
}

// ReadFile reads the contents of path from inside the container.
// TODO(forge): implement per docs/platform-plan.jsx Week 1-2
func (d *DockerDriver) ReadFile(_ context.Context, _, _ string) (string, error) {
	return "", errors.New("docker driver: not implemented")
}

// Destroy stops and removes the container identified by id.
// TODO(forge): implement per docs/platform-plan.jsx Week 1-2
func (d *DockerDriver) Destroy(_ context.Context, _ string) error {
	return errors.New("docker driver: not implemented")
}

// Status returns the current lifecycle state of the container.
// TODO(forge): implement per docs/platform-plan.jsx Week 1-2
func (d *DockerDriver) Status(_ context.Context, _ string) (Status, error) {
	return "", errors.New("docker driver: not implemented")
}

// IP returns the IP address of the container's primary network interface.
// TODO(forge): implement per docs/platform-plan.jsx Week 1-2
func (d *DockerDriver) IP(_ context.Context, _ string) (string, error) {
	return "", errors.New("docker driver: not implemented")
}
