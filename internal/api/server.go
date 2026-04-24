package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/jussmor/workspaces/internal/agent"
	forgectx "github.com/jussmor/workspaces/internal/context"
	"github.com/jussmor/workspaces/internal/coordinator"
	"github.com/jussmor/workspaces/internal/github"
	"github.com/jussmor/workspaces/internal/sandbox"
)

// Server is the FORGE HTTP API server — LAYER 06.
//
// All dependencies are injected at construction time; the Server itself
// holds no global state.
type Server struct {
	Coordinator *coordinator.Coordinator
	GitHub      *github.Client
	Context     *forgectx.Manager
	Sandbox     sandbox.Driver
	Registry    *sandbox.Registry
	Agent       *agent.Engine
}

// NewServer constructs a Server with all dependencies wired in.
func NewServer(
	coord *coordinator.Coordinator,
	gh *github.Client,
	ctxMgr *forgectx.Manager,
	drv sandbox.Driver,
	reg *sandbox.Registry,
	agentEngine *agent.Engine,
) *Server {
	return &Server{
		Coordinator: coord,
		GitHub:      gh,
		Context:     ctxMgr,
		Sandbox:     drv,
		Registry:    reg,
		Agent:       agentEngine,
	}
}

// Handler returns the configured chi.Mux with all routes registered.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	s.registerRoutes(r)
	return r
}

// Run starts the HTTP server on addr and gracefully shuts down when ctx is cancelled.
// TODO(forge): implement per docs/platform-plan.jsx Week 10
func (s *Server) Run(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: s.Handler(),
		// ReadTimeout / WriteTimeout are intentionally generous:
		// agent runs take up to 10 min, and the sandbox proxy keeps
		// connections open for as long as the browser holds them.
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // no write timeout — needed for SSE / proxy / long agent runs
		IdleTimeout:  120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	case err := <-errCh:
		return err
	}
}
