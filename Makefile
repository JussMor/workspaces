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
