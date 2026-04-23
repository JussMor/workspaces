# FORGE

**AI Software Engineering Platform** — autonomous code agent that reads GitHub issues, writes code, runs tests, and merges PRs.

## Overview

FORGE is a 7-layer monolith built with Go (backend) and Next.js 15 (dashboard). Each layer is an isolated `internal/` package wired together via dependency injection in a single binary (`cmd/forge`). The sandbox driver abstraction (`internal/sandbox`) makes the Docker→Firecracker migration a zero-callsite swap.

## Quickstart

```bash
# Start all services (backend :7000 + dashboard :3000)
make run

# Dashboard
open http://localhost:3000

# Backend health
curl http://localhost:7000/health
# → {"service":"forge","status":"ok","version":"0.0.1"}
```

## Sandbox Quickstart (requires local Docker)

```bash
# Run the full end-to-end verification (create → exec → destroy)
make verify-docker

# Or manually:

# 1. Create a sandbox
SB=$(curl -s -X POST localhost:7000/sandboxes \
  -H 'content-type: application/json' \
  -d '{"project_id":"demo","agent_role":"coder","image":"alpine:3.19"}')
echo "$SB" | python3 -m json.tool
ID=$(echo "$SB" | python3 -c 'import sys,json; print(json.load(sys.stdin)["id"])')

# 2. List sandboxes
curl -s localhost:7000/sandboxes | python3 -m json.tool

# 3. Exec a command inside the sandbox
curl -s -X POST localhost:7000/sandboxes/$ID/exec \
  -H 'content-type: application/json' \
  -d '{"cmd":["sh","-c","echo hello from sandbox"]}' | python3 -m json.tool
# → {"stdout":"hello from sandbox\n","stderr":"","exit_code":0}

# 4. Destroy the sandbox
curl -s -X DELETE localhost:7000/sandboxes/$ID -w "%{http_code}\n"
# → 204
```

## Architecture

```
cmd/forge/main.go
│
├── internal/api         (LAYER 06 — chi HTTP server, :7000)
│   └── routes.go        GET /health, POST /tasks, GET /sandboxes, …
│
├── internal/coordinator (LAYER 04 — pipeline orchestration)
│   └── coordinator.go   RunPipeline, RunFixLoop, Merge
│
├── internal/agent       (LAYER 02 — LLM agent engine)
│   ├── agent.go         Engine.Run(task)
│   └── tools.go         RunCommand, ReadFile, WriteFile, GitDiff, …
│
├── internal/context     (LAYER 03 — task context & state)
│   ├── context.go       ProjectContext, Delta, Event
│   └── manager.go       Apply, Snapshot, Subscribe
│
├── internal/sandbox     (LAYER 01 — execution isolation)
│   ├── driver.go        Driver interface + NewDriver(kind)
│   ├── docker.go        DockerDriver (dev)
│   └── firecracker.go   FirecrackerDriver (prod, stub)
│
├── internal/github      (LAYER 05 — GitHub API + webhooks)
│   ├── client.go        CreatePR, GetReviewComments, Merge
│   └── webhook.go       HandleWebhook
│
└── apps/dashboard       (LAYER 07 — Next.js 15 + Tailwind, :3000)
```

## Layer Status

| Layer | Package | Week | Status |
|-------|---------|------|--------|
| 01 — Sandbox | `internal/sandbox` | 1-2 | ✅ Live (DockerDriver + registry + reaper) |
| 02 — Agent Engine | `internal/agent` | 3-4 | 🏗 Stub |
| 03 — Context Engine | `internal/context` | 5-6 | 🏗 Stub |
| 04 — Coordinator | `internal/coordinator` | 7-8 | 🏗 Stub |
| 05 — GitHub | `internal/github` | 9 | 🏗 Stub |
| 06 — Orchestrator API | `internal/api` | 10 | ✅ /health live |
| 07 — Dashboard | `apps/dashboard` | 11-12 | 🏗 Scaffold |

See [ARCHITECTURE.md](./ARCHITECTURE.md) for detailed design rationale.
