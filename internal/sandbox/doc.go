// Package sandbox is LAYER 01 of FORGE — the execution foundation.
//
// Week 1-2 of the platform build. Provides a Driver interface with
// interchangeable implementations: DockerDriver for local dev,
// FirecrackerDriver for production on Hetzner bare metal.
//
// The SANDBOX_DRIVER environment variable selects the implementation at
// startup (default: "docker"). All callsites depend only on the Driver
// interface — swapping Docker → Firecracker requires no callsite changes.
package sandbox
