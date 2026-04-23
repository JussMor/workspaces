package sandbox

import (
	"context"
	"fmt"
)

// Driver is the sandbox execution abstraction.
//
// Implementations: DockerDriver (dev, Week 1-2) and FirecrackerDriver
// (prod, future). Swap between them via SANDBOX_DRIVER env var — no
// callsite changes required.
type Driver interface {
	Create(ctx context.Context, cfg Config) (string, error)
	Exec(ctx context.Context, id, cmd string) (ExecResult, error)
	WriteFile(ctx context.Context, id, path, content string) error
	ReadFile(ctx context.Context, id, path string) (string, error)
	Destroy(ctx context.Context, id string) error
	Status(ctx context.Context, id string) (Status, error)
	IP(ctx context.Context, id string) (string, error)
}

// Config holds the configuration for creating a sandbox instance.
type Config struct {
	Image string
	Env   map[string]string
	CPUs  float64
	MemMB int
}

// ExecResult captures output from a command run inside a sandbox.
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Status describes the lifecycle state of a sandbox.
type Status string

const (
	StatusRunning  Status = "running"
	StatusSleeping Status = "sleeping"
	StatusDead     Status = "dead"
)

// NewDriver returns the Driver implementation selected by kind.
// Supported values: "docker", "firecracker".
// Selecting via SANDBOX_DRIVER env var keeps all callsites stable.
func NewDriver(kind string) (Driver, error) {
	switch kind {
	case "docker":
		return &DockerDriver{}, nil
	case "firecracker":
		return &FirecrackerDriver{}, nil
	default:
		return nil, fmt.Errorf("unknown sandbox driver: %q", kind)
	}
}
