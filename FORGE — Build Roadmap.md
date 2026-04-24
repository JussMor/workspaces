# FORGE — Build Roadmap

**AI Software Engineering Platform · 12-Week Plan**

FORGE is a monolithic Go backend + Next.js dashboard that coordinates a team of local LLM agents to ship real software through real GitHub PRs. This document is the week-by-week path to building it.

**Companion artifacts**

- `platform-plan.jsx` — the canonical visual plan (committed at `docs/platform-plan.jsx`)

- `ARCHITECTURE.md` — dependency direction, Driver abstraction, env flags

- `README.md` — quickstart

---

## Architecture at a glance

| #   | Layer        | Package                   | Weeks   | Color     | Role                                        |
| --- | ------------ | ------------------------- | ------- | --------- | ------------------------------------------- |
| 01  | Sandbox      | `internal/sandbox`        | W1–W2   | `#00FFB2` | Execution foundation (Docker → Firecracker) |
| 02  | Agent Engine | `internal/agent`          | W3–W4   | `#FF6B35` | Ollama-powered ReAct loop + tools           |
| 03  | Context      | `internal/context`        | W5–W6   | `#A78BFA` | Shared project state + agent memory         |
| 04  | Coordinator  | `internal/coordinator`    | W7–W8   | `#F59E0B` | Multi-agent pipeline + fix loop             |
| 05  | GitHub       | `internal/github`         | W9      | `#34D399` | PRs, webhooks, review bridge                |
| 06  | API          | `internal/api`            | W10     | `#60A5FA` | Chi routes + WebSocket hub                  |
| 07  | Dashboard    | `apps/dashboard`          | W11–W12 | `#F472B6` | Next.js 15 control plane UI                 |
| 08  | Chat         | `apps/dashboard/app/chat` | W13–W14 | `#22D3EE` | Natural-language chat UI that drives the VM |

**Dependency direction (never reverse):**

```
api → coordinator → agent, context → sandbox, github
```

**Key abstraction:** `sandbox.Driver` interface. `DockerDriver` in dev, `FirecrackerDriver` in prod. Swap via `SANDBOX_DRIVER` env — zero callsite changes.

---

## Week 0 — Scaffold ✅

**Status:** complete (initial commit on `main`)

- Repo initialized at `github.com/JussMor/workspaces`

- Monolith layout: `cmd/forge/main.go` + `internal/*` packages

- Driver abstraction in place with Docker + Firecracker stubs

- Next.js 15 dashboard under `apps/dashboard` with 5 placeholder routes

- `docker-compose.yml`, `Makefile`, `README.md`, `ARCHITECTURE.md`, MIT `LICENSE`

- `/health` endpoint returns `{"status":"ok","service":"forge","version":"0.0.1"}`

---

## Week 1–2 — Layer 01: Sandbox ✅

**Status:** complete (April 2026)

**Goal:** Every agent runs inside an isolated, disposable sandbox. Local dev uses Docker; production swaps to Firecracker with no callsite changes.

### Features

- [x] `DockerDriver.Create` — spawn container on `sandbox-net` (10.0.5.0/24), 8 GB RAM cap, CPU limit, private IP

- [x] `DockerDriver.Exec` — bridge to `docker exec` with stdout/stderr capture + exit code

- [x] `DockerDriver.WriteFile` / `ReadFile` — tar stream into/out of container

- [x] `DockerDriver.Destroy` / `Status` / `IP` — full lifecycle

- [x] `Registry` backed by SQLite: `id, ip, status, project_id, agent_role, created_at, last_active, mem_usage, transport`

- [x] Idle-reaping job (sleep after N minutes, destroy after M)

- [ ] `FirecrackerDriver` — stubbed behind the same interface (real implementation deferred)

- [x] Driver integration test suite (`//go:build integration`) runnable against any Driver

### Deliverable

`make sandbox-demo` spawns a sandbox, runs commands, writes a file, reads it back, destroys it — verified against the `/sandboxes` registry.

### Exit criteria

- [x] Parallel sandbox churn test: 20 create/exec/destroy cycles in < 60 s with zero leaks (`make churn-test`)

- [ ] `SANDBOX_DRIVER=firecracker` boots on a Hetzner box and passes the same test (deferred)

---

## Week 3–4 — Layer 02: Agent Engine ✅

**Status:** complete (April 2026)

**Goal:** A single agent can complete a task inside a sandbox using Ollama and the built-in tool set. ReAct loop: think → tool call → observe → repeat.

### Features

- [x] `Agent.Run(ctx, task)` — ReAct loop, max 50 iterations, structured logging

- [x] Ollama client with role-based model routing (**two-model setup**, revised from original plan):
  - `planner` → `gemma4`

  - `coder` → `qwen3-coder:30b`

  - `reviewer` → `gemma4`

  - `tester` → `qwen3-coder:30b`

  - `pr_agent` → `gemma4`

- [x] **Minimal tool set (just-bash style)** — revised from original plan. Instead of a dozen bespoke wrappers, the agent composes workflows via shell:
  - `bash(command)` — run any shell command (git, tests, build, package install, search, etc.)

  - `read_file(path)` — structured file read

  - `write_file(path, content)` — structured file write (creates parent dirs)

- [x] Role-specific tools (GitHub API calls, not shell-expressible):
  - PR agent: `create_pr`, `set_pr_description` (stubs until Week 9)

  - Reviewer: `post_review_comment`, `approve_pr`, `request_changes` (stubs until Week 9)

- [x] Tool call validation + retry on transient failure (3 attempts with back-off)

- [x] Per-agent token/cost accounting (`RunStats`: iterations, prompt/completion tokens, tool calls, wall time)

- [x] `POST /agent/run` API endpoint + `make agent-demo REPO=<url> TASK="..."` deliverable

### Deliverable

`make agent-demo REPO=<url> TASK="add a /ping endpoint"` runs a single `coder` agent in a sandbox; it clones, edits, commits, pushes to a scratch branch. No PR yet — that's Week 9.

### Docker-compose wiring (live)

- `OLLAMA_URL=http://host.docker.internal:11434` so the containerised backend reaches Ollama on the host
- `/var/run/docker.sock` mounted into the backend so it can spawn sandbox containers
- Backend runs as root inside its container (sandboxes are the real security boundary)

### Exit criteria

- [x] Agent completes a multi-step task unattended (clone → edit → commit → push)

- [x] Max-iteration guard triggers cleanly on runaway loops

- [ ] Logs replayable from the registry (pending context-engine work in W5–6)

---

## Week 5–6 — Layer 03: Context Sync Engine

**Goal:** Shared state that all agents read and write coherently. Prevents the "two agents undo each other's work" failure mode.

### Features

- [ ] `ProjectContext` value type with full field set from the plan (RepoURL, Branch, LastCommit, FileTree, TaskID, Subtasks, CompletedWork, TestResults, LintResults, BuildStatus, ReviewComments, PRNumber, PRStatus, Decisions, Patterns, FailedApproaches)

- [ ] `ContextManager` with atomic `Apply(delta)` — agents return deltas, manager merges under mutex

- [ ] Versioned SQLite storage: every snapshot persisted, every apply appends

- [ ] Pub/sub stream so the dashboard sees state changes live

- [ ] `AgentMemory` — persistent memories keyed by repo (`always use zap for logging`, `reviewer prefers small focused PRs`, etc.)

- [ ] Memory weighting + decay so stale memories fade

- [ ] Context injection into every agent call — agent never queries raw storage

### Deliverable

Two coder agents run in parallel on the same task; their deltas merge without conflict and the resulting context is consistent.

### Exit criteria

- Concurrent delta stress test: 100 parallel `Apply` calls converge to a deterministic snapshot

- Memory retrieval at task start returns the 5 most-relevant memories for the repo

- Time-travel: any past snapshot is reconstructable from the append log

---

## Week 7–8 — Layer 04: Multi-Agent Coordinator

**Goal:** End-to-end task lifecycle — intake → plan → code → review → test → PR → merge — with a fix loop that re-engages the right agent on failure.

### Features

- [ ] `Coordinator.RunPipeline(ctx, task)`:
  - Planner produces subtasks

  - Coders run subtasks in parallel (one sandbox each)

  - Reviewer + Tester run in parallel after coding

  - PR agent creates the PR last

- [ ] `RunFixLoop` — on failing tests or pending review comments, re-engages the coder with full context; max retries configurable

- [ ] Parallelism controller (respects sandbox pool size)

- [ ] Cancellation + graceful teardown on user abort

- [ ] Per-task event log for dashboard replay

### Deliverable

`POST /tasks` with a task description produces a merged PR end-to-end with zero human input on a trivial change.

### Exit criteria

- Fix loop resolves one round of failing tests on a realistic Go package

- Parallel coder phase uses N sandboxes and finishes in near-max-subtask-time, not sum-of-subtask-times

- "Needs human intervention" surfaces cleanly when retries exhaust

---

## Week 9 — Layer 05: GitHub Integration

**Goal:** Full bidirectional GitHub. Agents open PRs; your review comments trigger the fix loop.

### Features

- [ ] `GitHubClient` wrapping `go-github` — clone, branch, push, create PR, list review comments, merge

- [ ] Webhook endpoint `POST /webhooks/github` with signature verification

- [ ] Route `pull_request_review` → `RunFixLoop` when `changes_requested`

- [ ] Route `pull_request_review` + `approved` → merge (if policy allows)

- [ ] Per-repo App/PAT config; secret rotation

- [ ] PR description templating with task summary, subtask checklist, test results

### Deliverable

Leaving a review comment on an agent-created PR triggers a follow-up commit within N minutes; approving auto-merges.

### Exit criteria

- Webhook signature verification passes fuzz tests

- PR↔task lookup survives a restart

- Review-driven fix loop handles multi-file changes

---

## Week 10 — Layer 06: Orchestrator API

**Goal:** One HTTP/WS surface the dashboard (and future clients) talks to. Everything the platform knows is exposed here.

### Features

- [ ] `POST /tasks` — create task, returns task ID

- [ ] `GET /tasks`, `GET /tasks/{id}` — state + subtasks + PR link

- [ ] `GET /sandboxes` — live registry view

- [ ] `POST /webhooks/github` — already wired in Week 9

- [ ] `GET /tasks/{id}/logs/ws` — live agent log stream

- [ ] `GET /notifications/ws` — platform-level events (sandbox churn, fix-loop retries, etc.)

- [ ] Auth middleware (token-based for now)

- [ ] Rate limiting per client

- [ ] Structured logs + Prometheus metrics endpoint `/metrics`

### Deliverable

`curl`-driven end-to-end demo: create task, watch logs over WS, see PR URL appear.

### Exit criteria

- WS survives 10 k messages/min without buffer blow-up

- All endpoints documented in `openapi.yaml`

- `/metrics` scraped by a local Prometheus

---

## Week 11–12 — Layer 07: Dashboard

**Goal:** Pollux-style control plane. See every sandbox, every task, every PR at a glance.

### Features

- [ ] `/sandboxes` — live table (ID, IP, status, agent, memory, age, transport)

- [ ] `/tasks` — list with status pills

- [ ] `/tasks/[id]` — subtask tree + live log stream + sandbox map

- [ ] `/tasks/[id]/diff` — PR diff viewer with inline review comments

- [ ] `/notifications` — event feed (fix-loop retries, merges, errors)

- [ ] Dark FORGE theme (JetBrains Mono, `#070710` bg, `#00FFB2` accent)

- [ ] WS-backed live updates across all views

- [ ] Task creation modal (`POST /tasks`)

- [ ] Deployed behind backend on same origin; `NEXT_PUBLIC_API_URL` for dev split

### Deliverable

You log in, type a task description, and watch agents work in real time — sandboxes spinning up, agents typing, tests running, PR appearing.

### Exit criteria

- All 5 routes use live WS data, no polling

- First paint under 1 s

- Keyboard shortcuts for task creation + sandbox actions

---

## Week 13–14 — Layer 08: Chat UI

**Goal:** A single chat bar is the primary way you interact with FORGE. Type a task in natural language, watch the VM (sandbox + agents) carry it out inline, reply to iterate. The monitoring dashboard from W11–W12 becomes a secondary surface.

### Features

- [ ] `/chat` route in `apps/dashboard` — full-viewport chat with a persistent input bar

- [ ] Per-project conversation thread persisted to SQLite via the API (new endpoint: `POST /conversations/{id}/messages`)

- [ ] Message types rendered inline:
  - User message (natural-language task)

  - Agent thought (ReAct reasoning)

  - Tool call (run_command, write_file, git_commit, …) with collapsible arg/result

  - Sandbox event (created, exec, destroyed)

  - PR card (opened, review state, merge button)

  - System event (fix-loop retry, error, needs-intervention)

- [ ] WebSocket stream — messages arrive live as the coordinator emits them

- [ ] **Iterate by replying** — new user message on the same thread re-enters the coordinator with full prior context; the fix loop handles "change X", "undo that", "also add Y"

- [ ] Slash commands for power use: `/sandbox ls`, `/task cancel`, `/pr open`, `/memory add …`

- [ ] Inline PR review: comment on a diff block in chat → routed to the GitHub review tool

- [ ] Attach context (paste a file, drop an image, link a repo path) as a message part

- [ ] Keyboard-first: `Cmd+K` focuses input, `Cmd+Enter` sends, `Esc` cancels current task

- [ ] Dashboard views from W11–W12 remain available as a collapsible right panel (live sandbox table + task tree for the current conversation)

### Deliverable

You open FORGE, type "add a `/ping` endpoint and open a PR", and watch sandboxes spin up, the coder type, tests run, and the PR land — all in one chat thread. Reply "rename it to `/healthz`" and it iterates without you touching the monitoring UI.

### Exit criteria

- Full end-to-end task completed from the chat surface alone (no modals, no table clicks)

- Iteration works: three consecutive replies on the same thread each produce a new commit on the same PR

- Chat replay — reload the page and the full conversation + live state rehydrates

- Latency: first agent token visible in chat < 2 s after send

---

## Cross-cutting work

These aren't a single layer — they run alongside from Week 3 onward.

| Track         | Start   | Description                                                        |
| ------------- | ------- | ------------------------------------------------------------------ |
| Observability | W3      | Structured logs (zap), trace IDs, Prometheus metrics               |
| Security      | W5      | Secret management, token rotation, webhook signature verification  |
| CI            | W2      | GitHub Actions: build, test, lint for every PR                     |
| Docs          | ongoing | This file + architecture decision records in `docs/adr/`           |
| Benchmarks    | W8      | Task-completion benchmark harness (success rate, time-to-PR, cost) |

---

## Milestones

| Milestone                    | Week | Gate                                                                  | Status |
| ---------------------------- | ---- | --------------------------------------------------------------------- | ------ |
| **M0** Scaffold green        | W0   | `/health` passes, commit on `main`                                    | ✅     |
| **M1** Sandbox parity        | W2   | Docker driver passes churn + integration suite (Firecracker deferred) | ✅     |
| **M2** Single agent ships    | W4   | `coder` completes a real task in a sandbox                            | ✅     |
| **M3** Coherent team         | W6   | Parallel agents produce merge-able output                             | ⏳     |
| **M4** End-to-end loop       | W8   | `POST /tasks` → merged PR, no human input (trivial task)              | ⏳     |
| **M5** Review-driven loop    | W9   | Review comment → fix commit                                           | ⏳     |
| **M6** API complete          | W10  | Dashboard can be built against the API alone                          | ⏳     |
| **M7** Dogfood ready         | W12  | FORGE ships its own features as PRs you review                        | ⏳     |
| **M8** Chat-driven iteration | W14  | End-to-end task + 3-turn iteration done entirely from the chat UI     | ⏳     |

---

## Progress tracking

Every layer lives in the autobuild initiative **FORGE — Monolithic Backend Scaffold** (`ini_u7AvcQUr`). Each layer becomes a feature; each feature breaks into executables that ship as individual PRs.

New work starts by adding a feature + executables under this initiative — never by editing week definitions in this doc. This doc is the map; the initiative is the work.
