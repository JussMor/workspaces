# FORGE

AI Software Engineering Platform. FORGE runs LLM agents inside Linux sandboxes to read tasks, edit code, run tests, and expose development services.

## Current Status (April 2026)

- Go + Next.js monolith is operational
- Docker sandbox runtime is operational locally with private `sandbox-net`
- ReAct Agent Engine is operational with role-based model routing
- Persistent sandbox execution (sandbox is not destroyed after `/agent/run`)
- Sandbox port proxy is live: `/sandboxes/{id}/port/{port}/...`

## Weekly Roadmap

| Week    | Layer                 | Status      | Notes                                                       |
| ------- | --------------------- | ----------- | ----------------------------------------------------------- |
| W0      | Scaffold              | DONE        | Repo, `internal/*` layout, compose, Makefile, `/health`     |
| W1-W2   | Layer 01 Sandbox      | DONE        | DockerDriver, SQLite registry, reaper, churn test           |
| W3-W4   | Layer 02 Agent Engine | DONE        | ReAct loop, tools `bash/read_file/write_file`, `/agent/run` |
| W5-W6   | Layer 03 Context      | PENDING     | Context sync, memory, snapshots/versioning                  |
| W7-W8   | Layer 04 Coordinator  | PENDING     | Multi-agent pipeline + fix loop                             |
| W9      | Layer 05 GitHub       | PENDING     | Webhooks + full PR automation                               |
| W10     | Layer 06 API          | IN PROGRESS | Core routes + sandbox proxy; WS/auth/metrics pending        |
| W11-W12 | Layer 07 Dashboard    | IN PROGRESS | Base UI; live operational views pending                     |
| W13-W14 | Chat UI               | PENDING     | Conversational control surface                              |

## Quick Architecture

```text
browser/dashboard -> API (chi, :7000 in container / :7001 host)
                  -> Agent Engine (ReAct)
                  -> Sandbox Driver (Docker)
                  -> GitHub layer (partial stub)
```

Main dependency direction:

```text
api -> coordinator -> agent, context -> sandbox, github
```

## Agent <-> Sandbox Orchestration

### 1) Control plane

- `POST /agent/run` creates or reuses a sandbox and runs the requested role
- `POST /sandboxes/{id}/exec` executes shell commands inside the sandbox
- `GET /sandboxes`, `GET /sandboxes/{id}`, `DELETE /sandboxes/{id}` manage lifecycle

### 2) Data plane

- The agent works in `/home/forge/workspace`
- `read_file` and `write_file` are used for structured I/O
- `bash` covers git, tests, builds, installs, and system operations

### 3) Networking

- Sandboxes run on Docker network `sandbox-net` (`10.0.5.0/24`)
- Backend also connects to `sandbox-net` to reach sandbox private IPs
- Host never connects directly to containers; it uses backend reverse proxy

### 4) Port proxy para servicios dentro del sandbox

If the agent starts a server on port `P` inside sandbox `SID`, access it from your machine at:

```text
http://localhost:7001/sandboxes/SID/port/P/
```

Real example:

```text
http://localhost:7001/sandboxes/forge-sandbox-d642f68d/port/8081/
```

## Sandbox Runtime Linux (forge-sandbox:latest)

Base image: Debian (`Dockerfile.sandbox`).

Includes:

- User `forge` (uid 1000), passwordless `sudo`
- HOME: `/home/forge`
- Workspace: `/home/forge/workspace`
- Dev tooling: `git`, `gh`, `make`, `build-essential`, `jq`, `ripgrep`, `curl`, `wget`
- Languages: Go 1.23, Node 20 (`npm/pnpm/yarn`), Python3 + pip
- Cloud/ops: `kubectl`, Docker CLI, `awscli`

Install additional package inside sandbox:

```bash
sudo apt-get update && sudo apt-get install -y <package>
```

## Models and Roles

- `planner` -> `gemma4`
- `coder` -> `qwen3-coder:30b`
- `reviewer` -> `gemma4`
- `tester` -> `qwen3-coder:30b`
- `pr_agent` -> `gemma4`

## Quickstart

### 1) Build sandbox image (one-time)

```bash
make sandbox-image
```

### 2) Start backend + dashboard

```bash
make run
```

### 3) Health

```bash
curl -s http://localhost:7001/health
```

### 4) Run agent (new sandbox)

```bash
make agent-demo REPO=https://github.com/JussMor/magickQA.api TASK="add a /ping endpoint"
```

### 5) Reuse existing sandbox

```bash
make agent-demo SANDBOX_ID=forge-sandbox-xxxxxxxx TASK="run tests and summarize"
```

### 6) Explicitly destroy sandbox

```bash
curl -X DELETE http://localhost:7001/sandboxes/forge-sandbox-xxxxxxxx
```

## Base API Available Today

- `GET /health`
- `POST /sandboxes`
- `GET /sandboxes`
- `GET /sandboxes/{id}`
- `POST /sandboxes/{id}/exec`
- `DELETE /sandboxes/{id}`
- `POST /agent/run`
- `ANY /sandboxes/{id}/port/{port}/*`

## Important Operational Notes

- If `agent-demo` fails with `sandbox image forge-sandbox:latest not found locally`, run `make sandbox-image`.
- Sandbox is persistent by design. If you do not call `DELETE`, it stays alive for handoff between agents.
- If an agent says it started a server, validate with curl on the proxy URL before assuming success.

## References

- `ARCHITECTURE.md`: detailed design, decisions, and networking
- `FORGE — Build Roadmap.md`: full weekly plan
