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

## /sandboxes API Surface

| Method   | Path                            | Request                                          | Response                          | Description                                                  |
| -------- | ------------------------------- | ------------------------------------------------ | --------------------------------- | ------------------------------------------------------------ |
| `POST`   | `/sandboxes`                    | `{project_id, agent_role, image}`                | `201` sandbox record              | Create container, attach to sandbox-net, persist in registry |
| `GET`    | `/sandboxes`                    | `?status=running\|sleeping\|dead&project_id=...` | `200 {sandboxes:[...]}`           | List all sandboxes with optional filters                     |
| `GET`    | `/sandboxes/{id}`               | —                                                | `200` sandbox record or `404`     | Get a single sandbox by ID                                   |
| `POST`   | `/sandboxes/{id}/exec`          | `{cmd:["sh","-c","echo hello"]}`                 | `200 {stdout, stderr, exit_code}` | Exec command inside container, touches last_active           |
| `ANY`    | `/sandboxes/{id}/port/{port}/*` | passthrough                                      | proxied response                  | Reverse-proxy HTTP/WS to a server inside the sandbox         |
| `DELETE` | `/sandboxes/{id}`               | —                                                | `204`                             | Destroy container and remove from registry                   |

### Sandbox Record Shape

```json
{
  "id": "forge-sandbox-abcd1234",
  "ip": "10.0.5.3",
  "status": "running",
  "project_id": "my-project",
  "agent_role": "coder",
  "created_at": "2026-04-23T15:00:00Z",
  "last_active": "2026-04-23T15:05:00Z",
  "mem_usage": 0,
  "transport": "none"
}
```

### Idle Reaper

A background goroutine fires every 60 s:

- `running → sleeping` after `FORGE_IDLE_SLEEP_MIN` (default 10) min of inactivity
- `sleeping → destroyed` after `FORGE_IDLE_DESTROY_MIN` (default 30) min of continued inactivity

## Sandbox Runtime Image (`forge-sandbox:latest`)

Sandboxes default to a pre-baked Debian image (`Dockerfile.sandbox`) so agents have batteries included and don't waste iterations installing common tooling.

**Image layout:**

- Base: `debian:bookworm-slim`
- Non-root user `forge` (uid 1000) with **passwordless sudo**
- `HOME=/home/forge`, workspace at **`/home/forge/workspace`** (also `WorkingDir` and default cwd for every `Exec`)
- Pre-installed: `git`, `gh`, `make`, `build-essential`, `jq`, `ripgrep`, `curl`, `wget`, `vim`
- Languages: Go 1.23, Node 20 (`npm` / `pnpm` / `yarn`), Python 3 + `pip`
- Cloud CLIs: `kubectl`, `docker` CLI, `awscli`
- The image does **not** end with `USER forge` — startup runs as root so the driver can chown the workspace; every `docker exec` then drops to `forge` explicitly.

Agents install anything else at runtime via `sudo apt-get install -y …`, `pip install …`, `npm i -g …`, or `go install …`.

Build once, reuse for every sandbox:

```bash
make sandbox-image      # builds forge-sandbox:latest
```

## Exec Semantics

`DockerDriver.Exec` always tries the preferred path first: `User: "forge"`, `WorkingDir: /home/forge/workspace`, shell `bash -lc`. If the container is missing the `forge` user (custom base image), Docker returns exit 126 with stdout `"unable to find user forge"` — the driver caches that per-container and transparently falls back to root + `sh -c`. This keeps the `bash` tool usable on any image the caller passes.

Environment injected on every call:

```
HOME=/home/forge
USER=forge  LOGNAME=forge
SHELL=/bin/bash
PATH=/home/forge/.local/bin:/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
TERM=xterm  LANG=C.UTF-8
```

## Networking: `sandbox-net`

All sandboxes and the **backend container itself** join a shared Docker bridge network named `sandbox-net` (subnet `10.0.5.0/24`). This is what makes the port-proxy work end-to-end without any docker `-p` gymnastics.

```
┌──────────┐   host :7001    ┌──────────────────┐         ┌─────────────────┐
│ browser  │────────────────▶│  backend (Go)    │──sandbox-net──▶│ forge-sandbox-* │
└──────────┘                 │  10.0.5.3        │         │ 10.0.5.X:<port> │
                             └──────────────────┘         └─────────────────┘
```

Key points:

- `internal/sandbox/docker.go` creates `sandbox-net` on first use and attaches every new sandbox to it.
- `docker-compose.yml` declares `sandbox-net` as **`external: true`** and attaches the backend to both `workspaces_default` (dashboard ↔ backend) and `sandbox-net` (backend ↔ sandboxes).
- Without this, `10.0.5.X` is unreachable from the backend and the proxy returns `dial tcp … i/o timeout`.

## Sandbox Port Proxy

Agents often need to run a dev server (Go HTTP, Next.js, Vite, …) and let the developer preview it in a browser. FORGE exposes every sandbox port through a single well-known URL pattern:

```
http://localhost:7001/sandboxes/{sandbox_id}/port/{port}/<anything>
```

How it works (`internal/api/proxy.go`):

1. Chi routes `/sandboxes/{id}/port/{port}` and `…/port/{port}/*` to `handleSandboxProxy`.
2. The handler validates the port is numeric (SSRF guard), looks up the sandbox IP in the registry (falls back to `Driver.IP` if blank), strips the `/sandboxes/{id}/port/{port}` prefix from the request path, and hands the request to `httputil.NewSingleHostReverseProxy(http://<ip>:<port>)`.
3. WebSocket upgrades are forwarded transparently (`httputil.ReverseProxy` passes `Upgrade` headers through).
4. Errors return `502 Bad Gateway` with a hint: `proxy error (is the server running on port X?)`.

Important server knobs that enable long-lived proxying:

- `http.Server.WriteTimeout = 0` — no server-side write deadline (required for SSE, long polls, and WebSockets).
- `ReadTimeout = 15s`, `IdleTimeout = 120s` remain in place.

### Why reverse-proxy instead of Docker port mapping

Docker `-p` requires ports to be declared at `docker run` time and cannot be added later. Since the agent decides at runtime which port(s) to use, an upfront mapping doesn't scale. The reverse-proxy model gives us:

- any port, any time, no pre-declaration
- one predictable URL pattern per sandbox
- multiple sandboxes can each bind `:3000` without host collisions
- traffic is contained on a private bridge network

For **external** (public-internet) URLs the agent can install `ngrok` or similar inside the sandbox — both approaches coexist.

## Agent Lifecycle & Sandbox Reuse

`POST /agent/run` is **non-destructive by default**:

Request:

```json
{
  "repo_url": "https://github.com/owner/repo",
  "task": "add a /ping endpoint …",
  "role": "coder",
  "image": "forge-sandbox:latest",
  "sandbox_id": "forge-sandbox-abcd1234"
}
```

Response:

```json
{
  "task_id": "agent-run-…",
  "role": "coder",
  "model": "qwen3-coder:30b",
  "answer": "…",
  "sandbox_id": "forge-sandbox-abcd1234",
  "stats": { "iterations": 9, "tool_calls": 8, "wall_time": "39s" }
}
```

Behaviour:

- **No `sandbox_id`** → backend creates a new sandbox, registers it, runs the agent, and **keeps it alive**. The caller owns cleanup.
- **With `sandbox_id`** → backend runs the agent inside the existing sandbox (multi-agent pipelines: coder → reviewer → tester → pr_agent all share one `/home/forge/workspace`).
- Cleanup is always explicit: `DELETE /sandboxes/{id}` (or let the idle reaper collect it after `FORGE_IDLE_DESTROY_MIN`).

A 10-minute `context.WithTimeout` wraps each run to guard against agents spawning long-running foreground commands. Agents are instructed in the coder prompt to daemonise any server (`nohup … & disown`) and report the proxy URL in their final answer.

## Environment Flags

| Variable                 | Default                  | Description                                        |
| ------------------------ | ------------------------ | -------------------------------------------------- |
| `PORT`                   | `7000`                   | Backend HTTP listen port                           |
| `SANDBOX_DRIVER`         | `docker`                 | Sandbox implementation (`docker` or `firecracker`) |
| `GITHUB_TOKEN`           | —                        | GitHub API token for PR operations                 |
| `OLLAMA_URL`             | `http://localhost:11434` | Local LLM endpoint                                 |
| `FORGE_DB_PATH`          | `./data/forge.db`        | SQLite registry path                               |
| `FORGE_MEM_MB`           | `8192`                   | Container memory cap (MB)                          |
| `FORGE_CPUS`             | `2`                      | Container CPU cores                                |
| `FORGE_IDLE_SLEEP_MIN`   | `10`                     | Minutes idle before sandbox sleeps                 |
| `FORGE_IDLE_DESTROY_MIN` | `30`                     | Minutes sleeping before sandbox is destroyed       |

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
