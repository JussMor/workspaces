package sandbox

import (
	"context"
	"log/slog"
	"time"
)

// Reaper is a background goroutine that manages idle sandbox lifecycle.
//
// Every tick it:
//  1. Marks sandboxes sleeping when idle > idleSleepMin.
//  2. Destroys sandboxes when they have been sleeping for > idleDestroyMin.
//
// Both thresholds are configurable via FORGE_IDLE_SLEEP_MIN and
// FORGE_IDLE_DESTROY_MIN env vars (minutes, defaults 10 and 30).
type Reaper struct {
	Registry       *Registry
	Driver         Driver
	IdleSleepMin   int // minutes before running→sleeping (default 10)
	IdleDestroyMin int // minutes before sleeping→destroyed (default 30)
	TickInterval   time.Duration
}

// NewReaper creates a Reaper with env-configurable thresholds.
func NewReaper(reg *Registry, drv Driver) *Reaper {
	sleep := envIntOr("FORGE_IDLE_SLEEP_MIN", 10)
	destroy := envIntOr("FORGE_IDLE_DESTROY_MIN", 30)
	return &Reaper{
		Registry:       reg,
		Driver:         drv,
		IdleSleepMin:   sleep,
		IdleDestroyMin: destroy,
		TickInterval:   60 * time.Second,
	}
}

// Start launches the reaper goroutine and returns immediately.
// The goroutine exits when ctx is cancelled.
func (r *Reaper) Start(ctx context.Context) {
	go r.run(ctx)
	slog.Info("reaper started",
		"idle_sleep_min", r.IdleSleepMin,
		"idle_destroy_min", r.IdleDestroyMin,
		"tick", r.TickInterval,
	)
}

func (r *Reaper) run(ctx context.Context) {
	ticker := time.NewTicker(r.TickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("reaper stopped")
			return
		case <-ticker.C:
			r.tick(ctx)
		}
	}
}

func (r *Reaper) tick(ctx context.Context) {
	now := time.Now()

	// Step 1: running → sleeping (idle > idleSleepMin and not already sleeping/dead).
	sleepCutoff := now.Add(-time.Duration(r.IdleSleepMin) * time.Minute)
	toSleep, err := r.Registry.ListOlderThan(sleepCutoff, string(StatusRunning))
	if err != nil {
		slog.Error("reaper: list for sleep", "err", err)
	}
	for _, sb := range toSleep {
		if err := r.Registry.SetStatus(sb.ID, StatusSleeping); err != nil {
			slog.Error("reaper: mark sleeping", "id", sb.ID, "err", err)
			continue
		}
		slog.Info("reaper: sandbox → sleeping", "id", sb.ID, "idle_since", sb.LastActive)
	}

	// Step 2: sleeping → destroyed (idle > idleDestroyMin).
	destroyCutoff := now.Add(-time.Duration(r.IdleDestroyMin) * time.Minute)
	toDestroy, err := r.Registry.ListOlderThan(destroyCutoff, string(StatusSleeping))
	if err != nil {
		slog.Error("reaper: list for destroy", "err", err)
	}
	for _, sb := range toDestroy {
		if err := r.Driver.Destroy(ctx, sb.ID); err != nil {
			slog.Error("reaper: driver destroy", "id", sb.ID, "err", err)
		}
		if err := r.Registry.Delete(sb.ID); err != nil {
			slog.Error("reaper: registry delete", "id", sb.ID, "err", err)
			continue
		}
		slog.Info("reaper: sandbox destroyed", "id", sb.ID, "idle_since", sb.LastActive)
	}
}
