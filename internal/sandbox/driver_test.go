//go:build integration

package sandbox_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jussmor/workspaces/internal/sandbox"
)

// runDriverSuite executes the shared acceptance test suite against any Driver.
// Called by each driver-specific test (TestDockerDriverSuite, etc.).
func runDriverSuite(t *testing.T, drv sandbox.Driver) {
	t.Helper()
	ctx := context.Background()

	// ── Create ────────────────────────────────────────────────────────────────
	t.Run("Create", func(t *testing.T) {
		id, err := drv.Create(ctx, sandbox.Config{Image: "alpine:3.19"})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if id == "" {
			t.Fatal("Create returned empty id")
		}
		t.Cleanup(func() { _ = drv.Destroy(ctx, id) })

		// ── Status ───────────────────────────────────────────────────────────
		status, err := drv.Status(ctx, id)
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if status != sandbox.StatusRunning {
			t.Fatalf("want status=running, got %q", status)
		}

		// ── IP ───────────────────────────────────────────────────────────────
		ip, err := drv.IP(ctx, id)
		if err != nil {
			t.Fatalf("IP: %v", err)
		}
		if !strings.HasPrefix(ip, "10.0.5.") {
			t.Fatalf("want 10.0.5.x IP, got %q", ip)
		}
	})

	// ── Exec ─────────────────────────────────────────────────────────────────
	t.Run("Exec", func(t *testing.T) {
		id, err := drv.Create(ctx, sandbox.Config{Image: "alpine:3.19"})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		t.Cleanup(func() { _ = drv.Destroy(ctx, id) })

		res, err := drv.Exec(ctx, id, "echo hello-forge")
		if err != nil {
			t.Fatalf("Exec: %v", err)
		}
		if res.ExitCode != 0 {
			t.Fatalf("want exit_code=0, got %d (stderr: %s)", res.ExitCode, res.Stderr)
		}
		if !strings.Contains(res.Stdout, "hello-forge") {
			t.Fatalf("stdout %q does not contain 'hello-forge'", res.Stdout)
		}
	})

	// ── Exec: non-zero exit code ──────────────────────────────────────────────
	t.Run("Exec_NonZeroExit", func(t *testing.T) {
		id, err := drv.Create(ctx, sandbox.Config{Image: "alpine:3.19"})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		t.Cleanup(func() { _ = drv.Destroy(ctx, id) })

		res, err := drv.Exec(ctx, id, "exit 42")
		if err != nil {
			t.Fatalf("Exec: %v", err)
		}
		if res.ExitCode != 42 {
			t.Fatalf("want exit_code=42, got %d", res.ExitCode)
		}
	})

	// ── WriteFile / ReadFile ──────────────────────────────────────────────────
	t.Run("WriteReadFile", func(t *testing.T) {
		id, err := drv.Create(ctx, sandbox.Config{Image: "alpine:3.19"})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		t.Cleanup(func() { _ = drv.Destroy(ctx, id) })

		const content = "hello from forge\n"
		if err := drv.WriteFile(ctx, id, "/tmp/forge-test.txt", content); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		got, err := drv.ReadFile(ctx, id, "/tmp/forge-test.txt")
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if got != content {
			t.Fatalf("ReadFile got %q, want %q", got, content)
		}
	})

	// ── Destroy ───────────────────────────────────────────────────────────────
	t.Run("Destroy", func(t *testing.T) {
		id, err := drv.Create(ctx, sandbox.Config{Image: "alpine:3.19"})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}

		if err := drv.Destroy(ctx, id); err != nil {
			t.Fatalf("Destroy: %v", err)
		}

		status, err := drv.Status(ctx, id)
		if err != nil {
			t.Fatalf("Status after Destroy: %v", err)
		}
		if status != sandbox.StatusDead {
			t.Fatalf("want status=dead after destroy, got %q", status)
		}
	})

	// ── Destroy: idempotent ───────────────────────────────────────────────────
	t.Run("Destroy_Idempotent", func(t *testing.T) {
		id, err := drv.Create(ctx, sandbox.Config{Image: "alpine:3.19"})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if err := drv.Destroy(ctx, id); err != nil {
			t.Fatalf("first Destroy: %v", err)
		}
		// Second call must not error.
		if err := drv.Destroy(ctx, id); err != nil {
			t.Fatalf("second (idempotent) Destroy: %v", err)
		}
	})
}

// TestDockerDriverSuite runs the full acceptance suite against DockerDriver.
func TestDockerDriverSuite(t *testing.T) {
	drv, err := sandbox.NewDriver("docker")
	if err != nil {
		t.Fatalf("NewDriver(docker): %v", err)
	}
	runDriverSuite(t, drv)
}

// TestDockerDriverParallelChurn spawns 20 sandboxes concurrently, execs a
// command in each, then destroys them all — must complete in under 60 s with
// zero leaks (every create paired with a successful destroy).
func TestDockerDriverParallelChurn(t *testing.T) {
	const n = 20
	const deadline = 60 * time.Second

	drv, err := sandbox.NewDriver("docker")
	if err != nil {
		t.Fatalf("NewDriver(docker): %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()

	var (
		wg      sync.WaitGroup
		created atomic.Int64
		execOK  atomic.Int64
		leaked  atomic.Int64
	)

	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			id, err := drv.Create(ctx, sandbox.Config{Image: "alpine:3.19"})
			if err != nil {
				t.Errorf("worker %d: Create: %v", idx, err)
				return
			}
			created.Add(1)

			res, err := drv.Exec(ctx, id, fmt.Sprintf("echo churn-%d", idx))
			if err == nil && res.ExitCode == 0 {
				execOK.Add(1)
			} else if err != nil {
				t.Errorf("worker %d: Exec: %v", idx, err)
			}

			if err := drv.Destroy(ctx, id); err != nil {
				t.Errorf("worker %d: Destroy: %v", idx, err)
				leaked.Add(1)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("churn: created=%d exec_ok=%d leaked=%d", created.Load(), execOK.Load(), leaked.Load())

	if created.Load() != n {
		t.Errorf("expected %d creates, got %d", n, created.Load())
	}
	if leaked.Load() != 0 {
		t.Errorf("leaked %d sandboxes (destroy failures)", leaked.Load())
	}
	if execOK.Load() != n {
		t.Errorf("expected %d successful execs, got %d", n, execOK.Load())
	}
}
