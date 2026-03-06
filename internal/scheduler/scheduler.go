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

	"github.com/egokernel/ek1/internal/datasync"
	"github.com/egokernel/ek1/internal/notifications"
	"github.com/egokernel/ek1/internal/signals"
)

// RunNowResponse is the result returned by POST /scheduler/run-now.
// It is a type alias for signals.ProcessResult exposed here so that
// swag can resolve it from within the scheduler package.
type RunNowResponse = signals.ProcessResult

// Status is the read-only snapshot returned by GET /scheduler/status.
type Status struct {
	Running         bool                     `json:"running"`                 // true while a pipeline cycle is executing
	IntervalMinutes int                      `json:"interval_minutes"`
	LastRunAt       *time.Time               `json:"last_run_at"`             // nil if never run
	NextRunAt       *time.Time               `json:"next_run_at"`             // nil if not started
	LastSignalCount int                      `json:"last_signal_count"`
	LastResult      *signals.ProcessResult   `json:"last_result"`             // nil if never run
	LastError       string                   `json:"last_error,omitempty"`    // non-empty when last cycle failed
	Services        []datasync.ServiceStatus `json:"services"`
}

// Scheduler orchestrates periodic sync cycles.
// A single goroutine runs the ticker; RunNowAsync provides an immediate non-blocking trigger.
// runMu prevents overlapping runs if a cycle takes longer than the interval.
type Scheduler struct {
	engine    *datasync.Engine
	processor *signals.Processor
	notifs    *notifications.Store
	interval  time.Duration

	mu     sync.Mutex
	status Status

	runMu   sync.Mutex   // prevents concurrent pipeline runs
	running atomic.Bool  // true while runLocked is executing; read by GetStatus
	stopCh  chan struct{}
}

// NewScheduler wires the sync engine, signals processor, and notification store.
// interval is parsed from SYNC_INTERVAL_MINUTES; pass 15*time.Minute as default.
func NewScheduler(
	engine *datasync.Engine,
	processor *signals.Processor,
	notifs *notifications.Store,
	interval time.Duration,
) *Scheduler {
	return &Scheduler{
		engine:    engine,
		processor: processor,
		notifs:    notifs,
		interval:  interval,
		stopCh:    make(chan struct{}),
		status:    Status{IntervalMinutes: int(interval.Minutes())},
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

// RunNow triggers an immediate processing cycle synchronously.
// Blocks until the run completes. Used internally by tests and direct callers.
func (s *Scheduler) RunNow(ctx context.Context) (signals.ProcessResult, error) {
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
// Running and Services are always live; the rest comes from the last completed cycle.
func (s *Scheduler) GetStatus() Status {
	s.mu.Lock()
	st := s.status
	s.mu.Unlock()
	st.Running = s.running.Load()
	st.Services = s.engine.ServiceStatuses() // always live — shows Active flag per adapter
	return st
}

// run acquires runMu and executes the full cycle synchronously.
// Called by the background ticker.
func (s *Scheduler) run(ctx context.Context) (signals.ProcessResult, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	return s.runLocked(ctx)
}

// runLocked is the core cycle body. Must be called with runMu held.
// sync → processor → notifications → status update.
func (s *Scheduler) runLocked(ctx context.Context) (signals.ProcessResult, error) {
	s.running.Store(true)
	defer s.running.Store(false)

	log.Printf("scheduler: cycle starting")

	var (
		rawSignals []datasync.RawSignal
		result     signals.ProcessResult
		cycleErr   error
	)

	// ── 1. Pull raw signals from all installed services ──────────────────────
	rawSignals, cycleErr = s.engine.Run(ctx)
	if cycleErr != nil {
		cycleErr = fmt.Errorf("sync: %w", cycleErr)
		log.Printf("scheduler: %v", cycleErr)
	} else {
		log.Printf("scheduler: pulled %d signals", len(rawSignals))

		// ── 2. Signals processor: LLM → Analysis → Storage ──────────────────
		result, cycleErr = s.processor.Process(ctx, rawSignals)
		if cycleErr != nil {
			cycleErr = fmt.Errorf("processor: %w", cycleErr)
			log.Printf("scheduler: %v", cycleErr)
		} else {
			log.Printf("scheduler: processor done — processed=%d relevant=%d replies=%d",
				result.ProcessedOK, result.RelevantSignals, result.RepliesGenerated)
		}
	}

	// ── 4. Update status — always, so failures are visible ───────────────────
	now := time.Now()
	next := now.Add(s.interval)
	s.mu.Lock()
	s.status.LastRunAt = &now
	s.status.NextRunAt = &next
	s.status.LastSignalCount = len(rawSignals)
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

