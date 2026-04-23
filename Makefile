.PHONY: dev build run down health clean typecheck

BINARY := forge
GO := go
BUN := $(shell which bun 2>/dev/null || echo npm)

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
	curl -fsS http://localhost:7000/health | python3 -m json.tool

## clean: remove compiled artifacts
clean:
	rm -f $(BINARY)
	rm -rf apps/dashboard/.next apps/dashboard/out

## typecheck: verify Go + TypeScript compile without errors
typecheck:
	$(GO) build ./...
	cd apps/dashboard && npx tsc --noEmit

## verify-docker: run live sandbox end-to-end test (requires Docker daemon)
## Run this locally — the CI sandbox does not expose the Docker socket.
## Usage: make verify-docker
verify-docker:
	@echo "▶ Starting forge on :7999..."
	@PORT=7999 $(GO) run ./cmd/forge &
	@FORGE_PID=$$! ; \
	cleanup() { kill $$FORGE_PID 2>/dev/null; wait $$FORGE_PID 2>/dev/null; }; \
	trap cleanup EXIT INT TERM; \
	sleep 2; \
	echo "▶ Health check"; \
	curl -fsS http://localhost:7999/health | python3 -m json.tool; \
	echo ""; \
	echo "▶ Creating sandbox (alpine:3.19)"; \
	SB=$$(curl -s -X POST http://localhost:7999/sandboxes \
	  -H 'content-type: application/json' \
	  -d '{"project_id":"verify","agent_role":"coder","image":"alpine:3.19"}'); \
	echo "$$SB" | python3 -m json.tool; \
	ID=$$(echo "$$SB" | python3 -c 'import sys,json; print(json.load(sys.stdin)["id"])'); \
	echo "Sandbox ID: $$ID"; \
	echo ""; \
	echo "▶ List sandboxes"; \
	curl -s http://localhost:7999/sandboxes | python3 -m json.tool; \
	echo ""; \
	echo "▶ Exec: echo hello from sandbox"; \
	RESULT=$$(curl -s -X POST http://localhost:7999/sandboxes/$$ID/exec \
	  -H 'content-type: application/json' \
	  -d '{"cmd":["sh","-c","echo hello from sandbox"]}'); \
	echo "$$RESULT" | python3 -m json.tool; \
	STDOUT=$$(echo "$$RESULT" | python3 -c 'import sys,json; print(json.load(sys.stdin)["stdout"])'); \
	if echo "$$STDOUT" | grep -q "hello from sandbox"; then \
	  echo "✅ exec output correct: $$STDOUT"; \
	else \
	  echo "❌ exec output wrong: $$STDOUT"; exit 1; \
	fi; \
	echo ""; \
	echo "▶ Destroying sandbox"; \
	HTTP_CODE=$$(curl -s -o /dev/null -w '%{http_code}' -X DELETE http://localhost:7999/sandboxes/$$ID); \
	if [ "$$HTTP_CODE" = "204" ]; then \
	  echo "✅ DELETE returned 204"; \
	else \
	  echo "❌ DELETE returned $$HTTP_CODE"; exit 1; \
	fi; \
	echo ""; \
	echo "✅ verify-docker passed"
