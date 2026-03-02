// Package scheduler runs the full sync → LLM → brain pipeline on a
// configurable interval and surfaces notable outcomes as notifications.
package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egokernel/ek1/internal/brain"
	"github.com/egokernel/ek1/internal/datasync"
	"github.com/egokernel/ek1/internal/notifications"
)

// RunNowResponse is the result returned by POST /scheduler/run-now.
// It is a type alias for brain.PipelineResult exposed here so that
// swag can resolve it from within the scheduler package.
type RunNowResponse = brain.PipelineResult

// Status is the read-only snapshot returned by GET /scheduler/status.
type Status struct {
	Running         bool                     `json:"running"`                 // true while a pipeline cycle is executing
	IntervalMinutes int                      `json:"interval_minutes"`
	LastRunAt       *time.Time               `json:"last_run_at"`             // nil if never run
	NextRunAt       *time.Time               `json:"next_run_at"`             // nil if not started
	LastSignalCount int                      `json:"last_signal_count"`
	LastResult      *brain.PipelineResult    `json:"last_result"`             // nil if never run
	LastError       string                   `json:"last_error,omitempty"`    // non-empty when last cycle failed
	Services        []datasync.ServiceStatus `json:"services"`
}

// Scheduler orchestrates periodic sync cycles.
// A single goroutine runs the ticker; RunNowAsync provides an immediate non-blocking trigger.
// runMu prevents overlapping runs if a cycle takes longer than the interval.
type Scheduler struct {
	engine   *datasync.Engine
	pipeline *brain.Pipeline
	svc      *brain.Service
	notifs   *notifications.Store
	interval time.Duration

	mu       sync.Mutex
	status   Status
	prevH2HI bool

	runMu   sync.Mutex   // prevents concurrent pipeline runs
	running atomic.Bool  // true while runLocked is executing; read by GetStatus
	stopCh  chan struct{}
}

// NewScheduler wires the sync engine, brain pipeline, and notification store.
// interval is parsed from SYNC_INTERVAL_MINUTES; pass 15*time.Minute as default.
func NewScheduler(
	engine *datasync.Engine,
	pipeline *brain.Pipeline,
	svc *brain.Service,
	notifs *notifications.Store,
	interval time.Duration,
) *Scheduler {
	return &Scheduler{
		engine:   engine,
		pipeline: pipeline,
		svc:      svc,
		notifs:   notifs,
		interval: interval,
		stopCh:   make(chan struct{}),
		status:   Status{IntervalMinutes: int(interval.Minutes())},
	}
}

// Start launches the background ticker goroutine. Safe to call once at startup.
func (s *Scheduler) Start() {
	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
				if _, err := s.run(ctx); err != nil {
					log.Printf("scheduler: run error: %v", err)
				}
				cancel()
			case <-s.stopCh:
				return
			}
		}
	}()
	log.Printf("scheduler: started — interval %s", s.interval)
}

// Stop shuts down the ticker goroutine.
func (s *Scheduler) Stop() {
	close(s.stopCh)
}

// RunNow triggers an immediate pipeline cycle synchronously.
// Blocks until the run completes. Used internally by tests and direct callers.
func (s *Scheduler) RunNow(ctx context.Context) (brain.PipelineResult, error) {
	return s.run(ctx)
}

// RunNowAsync fires a pipeline cycle in the background and returns immediately.
// Returns true if the cycle was started, false if one is already in progress.
// Poll GET /scheduler/status for the result.
func (s *Scheduler) RunNowAsync() bool {
	if !s.runMu.TryLock() {
		return false // already running
	}
	go func() {
		defer s.runMu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		if _, err := s.runLocked(ctx); err != nil {
			log.Printf("scheduler: run-now error: %v", err)
		}
	}()
	return true
}

// GetStatus returns a point-in-time snapshot of scheduler state.
// Running is always fresh from the atomic flag; the rest comes from the last completed cycle.
func (s *Scheduler) GetStatus() Status {
	s.mu.Lock()
	st := s.status
	s.mu.Unlock()
	st.Running = s.running.Load()
	return st
}

// run acquires runMu and executes the full cycle synchronously.
// Called by the background ticker.
func (s *Scheduler) run(ctx context.Context) (brain.PipelineResult, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	return s.runLocked(ctx)
}

// runLocked is the core cycle body. Must be called with runMu held.
// sync → pipeline → notifications → status update.
func (s *Scheduler) runLocked(ctx context.Context) (brain.PipelineResult, error) {
	s.running.Store(true)
	defer s.running.Store(false)

	log.Printf("scheduler: cycle starting")

	var (
		signals   []datasync.RawSignal
		result    brain.PipelineResult
		cycleErr  error
	)

	// ── 1. Pull raw signals from all installed services ──────────────────────
	signals, cycleErr = s.engine.Run(ctx)
	if cycleErr != nil {
		cycleErr = fmt.Errorf("sync: %w", cycleErr)
		log.Printf("scheduler: %v", cycleErr)
	} else {
		log.Printf("scheduler: pulled %d signals", len(signals))

		// ── 2. Brain pipeline: LLM → Triage → Decide → Events ───────────────
		result, cycleErr = s.pipeline.Run(ctx, signals)
		if cycleErr != nil {
			cycleErr = fmt.Errorf("pipeline: %w", cycleErr)
			log.Printf("scheduler: %v", cycleErr)
		} else {
			log.Printf("scheduler: pipeline done — accepted=%d rejected=%d ghosted=%d shielded=%v",
				result.Accepted, result.Rejected, result.Ghosted, result.Shielded)

			// ── 3. Notifications ─────────────────────────────────────────────
			s.checkH2HI()
		}
	}

	// ── 4. Update status — always, so failures are visible ───────────────────
	now := time.Now()
	next := now.Add(s.interval)
	s.mu.Lock()
	s.status.LastRunAt = &now
	s.status.NextRunAt = &next
	s.status.LastSignalCount = len(signals)
	s.status.Services = s.engine.ServiceStatuses()
	if cycleErr != nil {
		s.status.LastError = cycleErr.Error()
	} else {
		s.status.LastError = ""
		s.status.LastResult = &result
	}
	s.mu.Unlock()

	if cycleErr != nil {
		return result, fmt.Errorf("scheduler: %w", cycleErr)
	}
	return result, nil
}

// checkH2HI creates an H2HI notification on the first cycle where the kernel
// enters that state, and clears the flag when the kernel recovers.
func (s *Scheduler) checkH2HI() {
	isH2HI := s.svc.IsH2HI()

	s.mu.Lock()
	prev := s.prevH2HI
	s.prevH2HI = isH2HI
	s.mu.Unlock()

	if isH2HI && !prev {
		_, err := s.notifs.Create(notifications.Notification{
			Type:  notifications.TypeH2HI,
			Title: "Identity entropy spike — manual sync required",
			Body: "Your kernel has entered H2HI mode. Decision patterns have diverged " +
				"from your core values over the last 50 decisions. Review recent events " +
				"in /activities/events and call POST /brain/sync-acknowledge to resume " +
				"autonomous operation.",
		})
		if err != nil {
			log.Printf("scheduler: create H2HI notification: %v", err)
		} else {
			log.Printf("scheduler: H2HI notification created")
		}
	}
}
