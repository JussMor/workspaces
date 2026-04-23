# FORGE Architecture

## Monolithic Layout Rationale

FORGE uses a single Go binary (`cmd/forge`) rather than microservices. The reasons:

1. **Operational simplicity** — one process to deploy, monitor, and restart.
2. **Zero-latency inter-layer calls** — direct function calls instead of HTTP; no serialisation overhead.
3. **Testability** — each `internal/` package is independently testable; the monolith boundary is enforced by Go's import graph, not service boundaries.
4. **Incremental decomposition** — if a layer needs to scale independently later, the Driver/interface pattern makes extraction surgical.

## Dependency Direction

Dependencies flow strictly downward. No reverse imports.

```
api → coordinator → agent / context → sandbox
                  → github
```

The `types` package sits at the bottom and is imported by all layers.
The `api` package may import any layer; no other layer imports `api`.

## The Driver Abstraction

`internal/sandbox.Driver` is the key seam that makes Docker→Firecracker a zero-callsite swap:

```go
// All callsites depend only on this interface:
type Driver interface {
    Create(ctx, cfg) (id, error)
    Exec(ctx, id, cmd) (ExecResult, error)
    // …
}

// Selection at startup — callsites never change:
drv, _ := sandbox.NewDriver(os.Getenv("SANDBOX_DRIVER")) // "docker" | "firecracker"
```

To add a new driver (e.g. `gvisor`):
1. Create `internal/sandbox/gvisor.go` with a `GVisorDriver` struct implementing `Driver`.
2. Add `case "gvisor": return &GVisorDriver{}, nil` in `NewDriver`.
3. Set `SANDBOX_DRIVER=gvisor` at runtime.
4. Zero callsite changes required.

## Environment Flags

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `7000` | Backend HTTP listen port |
| `SANDBOX_DRIVER` | `docker` | Sandbox implementation (`docker` or `firecracker`) |
| `GITHUB_TOKEN` | — | GitHub API token for PR operations |
| `OLLAMA_URL` | `http://localhost:11434` | Local LLM endpoint |

## Running Locally

```bash
# Backend only (hot path for API dev)
go run ./cmd/forge

# Dashboard only
cd apps/dashboard && npm run dev

# Both via docker-compose
make run

# Health check
make health
```

## Adding a New Layer Stub

1. Create `internal/<layer>/doc.go` — package doc comment with layer number and week.
2. Define the primary struct and its interface in `<layer>.go`.
3. Wire it into `cmd/forge/main.go` via dependency injection.
4. Expose it through `internal/api/routes.go` if it needs an HTTP surface.
5. Add a row to the layer status table in `README.md`.
