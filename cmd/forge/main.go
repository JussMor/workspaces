// cmd/forge is the single binary entrypoint for the FORGE platform.
//
// It reads configuration from environment variables, wires all internal
// layers via dependency injection (no inter-package HTTP calls), and starts
// the HTTP API server on the configured PORT.
//
// Environment variables:
//
//	PORT              - HTTP listen port (default: 7000)
//	SANDBOX_DRIVER    - Sandbox implementation: "docker" or "firecracker" (default: docker)
//	GITHUB_TOKEN      - GitHub API token for PR operations
//	OLLAMA_URL        - Local LLM endpoint (default: http://localhost:11434)
//	FORGE_DB_PATH     - SQLite database path (default: ./data/forge.db)
//	FORGE_MEM_MB      - Container memory cap in MB (default: 8192)
//	FORGE_CPUS        - Container CPU cores (default: 2)
//	FORGE_IDLE_SLEEP_MIN   - Minutes of idle before marking sandbox sleeping (default: 10)
//	FORGE_IDLE_DESTROY_MIN - Minutes sleeping before destroying sandbox (default: 30)
package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jussmor/workspaces/internal/agent"
	"github.com/jussmor/workspaces/internal/api"
	forgectx "github.com/jussmor/workspaces/internal/context"
	"github.com/jussmor/workspaces/internal/coordinator"
	"github.com/jussmor/workspaces/internal/github"
	"github.com/jussmor/workspaces/internal/sandbox"
)

func main() {
	port := envOr("PORT", "7000")
	driverKind := envOr("SANDBOX_DRIVER", "docker")
	dbPath := envOr("FORGE_DB_PATH", "./data/forge.db")

	slog.Info("forge starting", "port", port, "driver", driverKind)

	// LAYER 01 — Sandbox Driver (selected by SANDBOX_DRIVER env).
	drv, err := sandbox.NewDriver(driverKind)
	if err != nil {
		log.Fatalf("forge: sandbox driver init failed: %v", err)
	}

	// LAYER 01 — SQLite Registry.
	reg, err := sandbox.NewRegistry(dbPath)
	if err != nil {
		log.Fatalf("forge: registry init failed: %v", err)
	}

	// LAYER 03 — Context Manager.
	ctxMgr := &forgectx.Manager{}

	// LAYER 02 — Agent Engine (wired with sandbox).
	tools := &agent.Tools{}
	agentEngine := &agent.Engine{
		Sandbox: drv,
		Tools:   tools,
	}

	// LAYER 05 — GitHub Client.
	gh := &github.Client{
		Token: os.Getenv("GITHUB_TOKEN"),
	}

	// LAYER 04 — Coordinator (wires agent, context, github, sandbox).
	coord := &coordinator.Coordinator{
		Agent:   agentEngine,
		Context: ctxMgr,
		GitHub:  gh,
		Sandbox: drv,
	}

	// LAYER 06 — API Server.
	srv := api.NewServer(coord, gh, ctxMgr, drv, reg)

	// Graceful shutdown on SIGINT / SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// LAYER 01 — Idle Reaper.
	reaper := sandbox.NewReaper(reg, drv)
	reaper.Start(ctx)

	addr := ":" + port
	slog.Info("forge: listening", "addr", addr)
	if err := srv.Run(ctx, addr); err != nil {
		log.Fatalf("forge: server error: %v", err)
	}
	slog.Info("forge: shutdown complete")
}

// envOr returns the value of the environment variable named key, or
// fallback if the variable is unset or empty.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
