.PHONY: dev build run down health clean typecheck verify-docker sandbox-demo

BINARY := forge
GO := go
BUN := $(shell which bun 2>/dev/null || echo npm)
BACKEND_HOST_PORT ?= 7001
VERIFY_PORT ?= 7999

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
