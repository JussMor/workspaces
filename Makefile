.PHONY: dev build run down health clean typecheck verify-docker sandbox-demo agent-demo

BINARY := forge
GO := go
BUN := $(shell which bun 2>/dev/null || echo npm)
BACKEND_HOST_PORT ?= 7001
VERIFY_PORT ?= 7999
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

## typecheck: verify Go + TypeScript compile without errors
typecheck:
	$(GO) build ./...
	cd apps/dashboard && npx tsc --noEmit

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

## agent-demo: Week 3-4 deliverable
## Runs a single coder agent inside a sandbox against a repo + task.
## Requires a local Docker daemon + Ollama running.
## Usage: make agent-demo REPO=<url> TASK="add a /ping endpoint"
AGENT_TASK ?= add a /ping endpoint that returns JSON {"pong":true}
AGENT_REPO ?= $(ORIGIN_REPO_URL)
AGENT_ROLE ?= coder
AGENT_IMAGE ?= alpine:3.19
agent-demo:
	@echo "═══ FORGE — Layer 02 Agent Engine Demo ═══"
	@echo ""
	@echo "▶ Ensuring forge is reachable..."; \
	RUN_PORT=$(VERIFY_PORT); FORGE_PID=""; STARTED_FORGE=0; \
	HEALTH_OK=0; AGENT_OK=0; \
	if curl -fsS "http://localhost:$$RUN_PORT/health" >/dev/null 2>&1; then \
	  HEALTH_OK=1; \
	  PROBE_CODE=$$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://localhost:$$RUN_PORT/agent/run" \
	    -H 'content-type: application/json' \
	    -d '{}'); \
	  if [ "$$PROBE_CODE" != "404" ]; then AGENT_OK=1; fi; \
	fi; \
	if [ "$$HEALTH_OK" = "1" ] && [ "$$AGENT_OK" = "1" ]; then \
	  echo "ℹ️  Reusing existing forge on :$$RUN_PORT"; \
	else \
	  if [ "$$HEALTH_OK" = "1" ] && [ "$$AGENT_OK" != "1" ]; then \
	    echo "ℹ️  Existing service on :$$RUN_PORT is not forge API compatible (/agent/run missing)"; \
	    RUN_PORT=$$((RUN_PORT + 1)); \
	    while lsof -iTCP:$$RUN_PORT -sTCP:LISTEN >/dev/null 2>&1; do \
	      RUN_PORT=$$((RUN_PORT + 1)); \
	    done; \
	  fi; \
	  echo "▶ Starting forge on :$$RUN_PORT..."; \
	  PORT=$$RUN_PORT $(GO) run ./cmd/forge & \
	  FORGE_PID=$$!; STARTED_FORGE=1; \
	fi; \
	cleanup() { \
	  if [ "$$STARTED_FORGE" = "1" ] && [ -n "$$FORGE_PID" ]; then \
	    kill $$FORGE_PID 2>/dev/null; \
	    wait $$FORGE_PID 2>/dev/null; \
	  fi; \
	}; \
	trap cleanup EXIT INT TERM; \
	for i in 1 2 3 4 5 6 7 8 9 10; do \
	  if curl -fsS "http://localhost:$$RUN_PORT/health" >/dev/null 2>&1; then \
	    PROBE_CODE=$$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://localhost:$$RUN_PORT/agent/run" \
	      -H 'content-type: application/json' \
	      -d '{}'); \
	    if [ "$$PROBE_CODE" != "404" ]; then break; fi; \
	  fi; \
	  if [ "$$STARTED_FORGE" = "1" ] && ! kill -0 $$FORGE_PID 2>/dev/null; then \
	    echo "❌ forge exited before becoming healthy"; exit 1; \
	  fi; \
	  sleep 1; \
	done; \
	if ! curl -fsS "http://localhost:$$RUN_PORT/health" >/dev/null 2>&1; then \
	  echo "❌ forge not healthy on :$$RUN_PORT"; exit 1; \
	fi; \
	PROBE_CODE=$$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://localhost:$$RUN_PORT/agent/run" \
	  -H 'content-type: application/json' \
	  -d '{}'); \
	if [ "$$PROBE_CODE" = "404" ]; then \
	  echo "❌ forge on :$$RUN_PORT does not expose /agent/run"; exit 1; \
	fi; \
	echo "✅ Health OK"; \
	echo ""; \
	echo "▶ Running agent (role=$(AGENT_ROLE), task=\"$(AGENT_TASK)\")"; \
	echo "  This may take several minutes while the LLM reasons..."; \
	echo ""; \
	BODY=$$(printf '{"repo_url":"%s","task":"%s","role":"%s","image":"%s"}' \
	  "$(AGENT_REPO)" "$(AGENT_TASK)" "$(AGENT_ROLE)" "$(AGENT_IMAGE)"); \
	TMP=$$(mktemp); \
	HTTP_CODE=$$(curl -sS -o "$$TMP" -w '%{http_code}' -X POST "http://localhost:$$RUN_PORT/agent/run" \
	  -H 'content-type: application/json' \
	  -d "$$BODY"); \
	RESULT=$$(cat "$$TMP"); \
	rm -f "$$TMP"; \
	if [ "$$HTTP_CODE" != "200" ]; then \
	  echo "❌ /agent/run failed (HTTP $$HTTP_CODE)"; \
	  printf '%s\n' "$$RESULT"; \
	  exit 1; \
	fi; \
	echo "── Agent Result ──"; \
	printf '%s\n' "$$RESULT" | python3 -m json.tool; \
	echo ""; \
	echo "═══ ✅ Layer 02 agent-demo complete ═══"
