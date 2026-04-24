.PHONY: dev build run down health clean typecheck verify-docker sandbox-demo sandbox-image agent-demo integration-test churn-test

BINARY := forge
GO := go
BUN := $(shell which bun 2>/dev/null || echo npm)
BACKEND_HOST_PORT ?= 7001
VERIFY_PORT ?= 7999
REPO ?=
SANDBOX_ID ?=
TASK ?= add a /ping endpoint that returns {"pong":true}
ORIGIN_REPO_URL := $(shell git config --get remote.origin.url 2>/dev/null)

export BACKEND_HOST_PORT

## dev: run backend in background + dashboard in foreground (ctrl-c stops both)
dev:
	$(GO) run ./cmd/forge &
	BACKEND_PID=$$! ; \
	trap "kill $$BACKEND_PID 2>/dev/null" EXIT INT TERM ; \
	cd apps/dashboard && $(BUN) run dev

## build: compile the Go binary and dashboard
build:
	$(GO) build -o $(BINARY) ./cmd/forge
	cd apps/dashboard && $(BUN) run build

## run: start services with docker-compose
run:
	docker compose up --build

## down: stop docker-compose services
down:
	docker compose down

## health: curl the backend health endpoint
health:
	curl -fsS http://localhost:$(BACKEND_HOST_PORT)/health | python3 -m json.tool

## clean: remove compiled artifacts
clean:
	rm -f $(BINARY)
	rm -rf apps/dashboard/.next apps/dashboard/out

## sandbox-image: build the pre-baked forge-sandbox:latest image used by the agent
## Includes the `forge` user, git, gh, go, node, python, kubectl, docker CLI, awscli, sudo.
sandbox-image:
	docker build -f Dockerfile.sandbox -t forge-sandbox:latest .

## typecheck: verify Go + TypeScript compile without errors
typecheck:
	$(GO) build ./...
	cd apps/dashboard && npx tsc --noEmit

## integration-test: run DockerDriver acceptance suite (requires Docker daemon)
## Runs all sub-tests in TestDockerDriverSuite + the parallel churn test.
integration-test:
	$(GO) test -v -tags integration -timeout 120s ./internal/sandbox/...

## churn-test: 20 parallel create/exec/destroy cycles — must finish in < 60 s with zero leaks
churn-test:
	$(GO) test -v -tags integration -timeout 90s -run TestDockerDriverParallelChurn ./internal/sandbox/...

## verify-docker (aka sandbox-demo): Week 1-2 deliverable
## Spawn sandbox → exec command → write file → read it back → destroy → verify registry.
## Requires a local Docker daemon. Usage: make verify-docker
verify-docker: sandbox-demo
sandbox-demo:
	@echo "═══ FORGE — Layer 01 Sandbox Verification ═══"
	@echo ""
	@echo "▶ Starting forge on :$(VERIFY_PORT)..."
	@PORT=$(VERIFY_PORT) $(GO) run ./cmd/forge &
	@FORGE_PID=$$! ; \
	cleanup() { kill $$FORGE_PID 2>/dev/null; wait $$FORGE_PID 2>/dev/null; }; \
	trap cleanup EXIT INT TERM; \
	for i in 1 2 3 4 5 6 7 8 9 10; do \
	  if curl -fsS http://localhost:$(VERIFY_PORT)/health >/dev/null 2>&1; then break; fi; \
	  if ! kill -0 $$FORGE_PID 2>/dev/null; then \
	    echo "❌ forge exited before becoming healthy"; exit 1; \
	  fi; \
	  sleep 1; \
	done; \
	if ! curl -fsS http://localhost:$(VERIFY_PORT)/health >/dev/null 2>&1; then \
	  echo "❌ forge not healthy on :$(VERIFY_PORT)"; exit 1; \
	fi; \
	echo "✅ Health OK"; \
	echo ""; \
	echo "▶ 1/5 Create sandbox (alpine:3.19)"; \
	SB=$$(curl -s -X POST http://localhost:$(VERIFY_PORT)/sandboxes \
	  -H 'content-type: application/json' \
	  -d '{"project_id":"verify","agent_role":"coder","image":"alpine:3.19"}'); \
	printf '%s\n' "$$SB" | python3 -m json.tool; \
	ID=$$(printf '%s' "$$SB" | python3 -c 'import sys,json; print(json.load(sys.stdin)["id"])'); \
	echo "  Sandbox ID: $$ID"; \
	echo ""; \
	echo "▶ 2/5 Exec: uname -a (prove command execution)"; \
	RESULT=$$(curl -s -X POST http://localhost:$(VERIFY_PORT)/sandboxes/$$ID/exec \
	  -H 'content-type: application/json' \
	  -d '{"cmd":["uname","-a"]}'); \
	printf '%s\n' "$$RESULT" | python3 -m json.tool; \
	EXIT=$$(printf '%s' "$$RESULT" | python3 -c 'import sys,json; print(json.load(sys.stdin)["exit_code"])'); \
	if [ "$$EXIT" = "0" ]; then \
	  echo "✅ exec exit_code=0"; \
	else \
	  echo "❌ exec exit_code=$$EXIT"; exit 1; \
	fi; \
	echo ""; \
	echo "▶ 3/5 Write file inside sandbox"; \
	RESULT=$$(curl -s -X POST http://localhost:$(VERIFY_PORT)/sandboxes/$$ID/exec \
	  -H 'content-type: application/json' \
	  -d '{"cmd":["sh","-c","echo hello-from-forge > /tmp/verify.txt"]}'); \
	EXIT=$$(printf '%s' "$$RESULT" | python3 -c 'import sys,json; print(json.load(sys.stdin)["exit_code"])'); \
	if [ "$$EXIT" = "0" ]; then \
	  echo "✅ file written"; \
	else \
	  echo "❌ write failed (exit $$EXIT)"; exit 1; \
	fi; \
	echo ""; \
	echo "▶ 4/5 Read file back"; \
	RESULT=$$(curl -s -X POST http://localhost:$(VERIFY_PORT)/sandboxes/$$ID/exec \
	  -H 'content-type: application/json' \
	  -d '{"cmd":["cat","/tmp/verify.txt"]}'); \
	printf '%s\n' "$$RESULT" | python3 -m json.tool; \
	STDOUT=$$(printf '%s' "$$RESULT" | python3 -c 'import sys,json; print(json.load(sys.stdin)["stdout"].strip())'); \
	if [ "$$STDOUT" = "hello-from-forge" ]; then \
	  echo "✅ readback correct: $$STDOUT"; \
	else \
	  echo "❌ readback wrong: $$STDOUT"; exit 1; \
	fi; \
	echo ""; \
	echo "▶ 5/5 Destroy sandbox"; \
	HTTP_CODE=$$(curl -s -o /dev/null -w '%{http_code}' -X DELETE http://localhost:$(VERIFY_PORT)/sandboxes/$$ID); \
	if [ "$$HTTP_CODE" = "204" ]; then \
	  echo "✅ DELETE returned 204"; \
	else \
	  echo "❌ DELETE returned $$HTTP_CODE"; exit 1; \
	fi; \
	echo ""; \
	echo "▶ Verify sandbox removed from registry"; \
	REMAINING=$$(curl -s http://localhost:$(VERIFY_PORT)/sandboxes \
	  | python3 -c "import sys,json; print(sum(1 for s in json.load(sys.stdin)['sandboxes'] if s['id']=='$$ID'))"); \
	if [ "$$REMAINING" = "0" ]; then \
	  echo "✅ sandbox gone from /sandboxes registry"; \
	else \
	  echo "❌ sandbox still in registry"; exit 1; \
	fi; \
	echo ""; \
	echo "═══ ✅ Layer 01 sandbox-demo passed ═══"

## agent-demo: Week 3-4 deliverable — run a coder agent on a running forge backend
## Requires: forge to be running (either via `make run` or `go run ./cmd/forge`)
## Sends a task to POST /agent/run (role=coder, model=Qwen3-coder:30b).
##
## Usage:
##   make run &                  # start docker-compose in background
##   make agent-demo REPO=https://github.com/JussMor/magickQA.api
##   make agent-demo REPO=... TASK="custom task description"
agent-demo:
	@if [ -z "$(REPO)" ]; then \
	  printf "Repo URL (blank = no repo): "; \
	  read REPO_IN; REPO="$$REPO_IN"; \
	else \
	  REPO="$(REPO)"; \
	fi; \
	if [ -z "$(TASK)" ] || [ "$(TASK)" = 'add a /ping endpoint that returns {"pong":true}' ]; then \
	  printf "Task [default: add a /ping endpoint that returns {pong:true}]: "; \
	  read TASK_IN; \
	  if [ -z "$$TASK_IN" ]; then TASK='add a /ping endpoint that returns {"pong":true}'; else TASK="$$TASK_IN"; fi; \
	else \
	  TASK="$(TASK)"; \
	fi; \
	if [ -z "$$REPO" ]; then \
	  echo "═══ FORGE — Layer 02 Agent Demo (no repo) ═══"; \
	else \
	  echo "═══ FORGE — Layer 02 Agent Demo: $$REPO ═══"; \
	fi; \
	echo ""; \
	echo "▶ Checking if forge is running on :$(BACKEND_HOST_PORT)..."; \
	if ! curl -fsS http://localhost:$(BACKEND_HOST_PORT)/health >/dev/null 2>&1; then \
	  echo "❌ forge not reachable on localhost:$(BACKEND_HOST_PORT)"; \
	  echo "   Start it with: make run   (or: PORT=7001 go run ./cmd/forge)"; \
	  exit 1; \
	fi; \
	if ! docker image inspect forge-sandbox:latest >/dev/null 2>&1; then \
	  echo "❌ sandbox image forge-sandbox:latest not found locally"; \
	  echo "   Build it once with: make sandbox-image"; \
	  exit 1; \
	fi; \
	echo "✅ Forge is healthy on :$(BACKEND_HOST_PORT)"; \
	echo ""; \
	echo "▶ Dispatching coder agent (model=Qwen3-coder:30b)..."; \
	echo "  Task : $$TASK"; \
	if [ -n "$$REPO" ]; then echo "  Repo : $$REPO"; fi; \
	echo ""; \
	PAYLOAD=$$(TASK="$$TASK" REPO="$$REPO" SANDBOX_ID='$(SANDBOX_ID)' python3 -c 'import json, os; d={"role":"coder","task":os.environ.get("TASK","")}; repo=os.environ.get("REPO","").strip(); sid=os.environ.get("SANDBOX_ID","").strip(); d.update({"repo_url":repo} if repo else {}); d.update({"sandbox_id":sid} if sid else {}); print(json.dumps(d))'); \
	TMP_BODY=$$(mktemp); \
	HTTP_CODE=$$(curl -sS --max-time 300 -o "$$TMP_BODY" -w '%{http_code}' -X POST http://localhost:$(BACKEND_HOST_PORT)/agent/run \
	  -H 'content-type: application/json' \
	  -d "$$PAYLOAD" || true); \
	RESULT=$$(cat "$$TMP_BODY"); \
	rm -f "$$TMP_BODY"; \
	if [ "$$HTTP_CODE" -lt 200 ] || [ "$$HTTP_CODE" -ge 300 ]; then \
	  echo "❌ /agent/run failed with HTTP $$HTTP_CODE"; \
	  if [ -n "$$RESULT" ]; then printf '%s\n' "$$RESULT"; fi; \
	  exit 1; \
	fi; \
	if [ -z "$$RESULT" ]; then \
	  echo "❌ /agent/run returned HTTP $$HTTP_CODE but empty body"; exit 1; \
	fi; \
	echo "══ Answer ══"; \
	printf '%s' "$$RESULT" | python3 -c 'import sys,json; r=json.load(sys.stdin); print(r.get("answer","(no answer)"))'; \
	echo ""; \
	echo "══ Stats ══"; \
	printf '%s' "$$RESULT" | jq '.stats | "  role       : \(.role)\n  model      : \(.model)\n  iterations : \(.iterations)\n  tool_calls : \(.tool_calls)\n  tokens     : prompt=\(.prompt_tokens) completion=\(.completion_tokens)\n  wall_time  : \(.wall_time)"' -r 2>/dev/null || echo "  (stats unavailable)"; \
	echo ""; \
	SID=$$(printf '%s' "$$RESULT" | jq -r '.sandbox_id' 2>/dev/null); \
	if [ -n "$$SID" ] && [ "$$SID" != "null" ]; then \
	  echo "══ Sandbox (kept alive) ══"; \
	  echo "  id : $$SID"; \
	  echo "  reuse : make agent-demo SANDBOX_ID=$$SID TASK=\"...\""; \
	  echo "  destroy: curl -X DELETE http://localhost:$(BACKEND_HOST_PORT)/sandboxes/$$SID"; \
	fi; \
	echo ""; \
	echo "═══ ✅ Layer 02 agent-demo complete ═══"
