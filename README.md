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
| 01 — Sandbox | `internal/sandbox` | 1-2 | 🏗 Stub |
| 02 — Agent Engine | `internal/agent` | 3-4 | 🏗 Stub |
| 03 — Context Engine | `internal/context` | 5-6 | 🏗 Stub |
| 04 — Coordinator | `internal/coordinator` | 7-8 | 🏗 Stub |
| 05 — GitHub | `internal/github` | 9 | 🏗 Stub |
| 06 — Orchestrator API | `internal/api` | 10 | ✅ /health live |
| 07 — Dashboard | `apps/dashboard` | 11-12 | 🏗 Scaffold |

See [ARCHITECTURE.md](./ARCHITECTURE.md) for detailed design rationale.
