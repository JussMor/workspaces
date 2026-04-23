// cmd/forge is the single binary entrypoint for the FORGE platform.
//
// It reads configuration from environment variables, wires all internal
// layers via dependency injection (no inter-package HTTP calls), and starts
// the HTTP API server on the configured PORT.
//
// Environment variables:
//
//	PORT           - HTTP listen port (default: 7000)
//	SANDBOX_DRIVER - Sandbox implementation: "docker" or "firecracker" (default: docker)
//	GITHUB_TOKEN   - GitHub API token for PR operations
//	OLLAMA_URL     - Local LLM endpoint (default: http://localhost:11434)
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jussmor/workspaces/internal/agent"
	forgectx "github.com/jussmor/workspaces/internal/context"
	"github.com/jussmor/workspaces/internal/api"
	"github.com/jussmor/workspaces/internal/coordinator"
	"github.com/jussmor/workspaces/internal/github"
	"github.com/jussmor/workspaces/internal/sandbox"
)

func main() {
	port := envOr("PORT", "7000")
	driverKind := envOr("SANDBOX_DRIVER", "docker")

	log.Printf("forge starting: port=%s driver=%s", port, driverKind)

	// LAYER 01 — Sandbox Driver (selected by SANDBOX_DRIVER env).
	drv, err := sandbox.NewDriver(driverKind)
	if err != nil {
		log.Fatalf("forge: sandbox driver init failed: %v", err)
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
	srv := api.NewServer(coord, gh, ctxMgr, drv)

	// Graceful shutdown on SIGINT / SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	addr := ":" + port
	log.Printf("forge: listening on %s", addr)
	if err := srv.Run(ctx, addr); err != nil {
		log.Fatalf("forge: server error: %v", err)
	}
	log.Println("forge: shutdown complete")
}

// envOr returns the value of the environment variable named key, or
// fallback if the variable is unset or empty.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
