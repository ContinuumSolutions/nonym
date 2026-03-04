// Package execution is Stage 2 of the Ego-Kernel (Hand stage).
// It turns brain decisions into real API actions and maintains an approval queue
// for actions that exceed the auto-execute threshold.
package execution

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/egokernel/ek1/internal/datasync"
	"github.com/egokernel/ek1/internal/integrations"
	"github.com/egokernel/ek1/internal/notifications"
)

// Engine routes actions to executors and manages the approval queue.
type Engine struct {
	integrations *integrations.Store
	executors    map[string]Executor
	queue        *Store
	notifs       *notifications.Store
	threshold    float64 // MICROWALLET_THRESHOLD, default 50.0
}

// NewEngine wires the execution engine.
// threshold is the max auto-execute amount; actions above it go to the queue.
func NewEngine(
	integrationsStore *integrations.Store,
	execs []Executor,
	queue *Store,
	notifs *notifications.Store,
	threshold float64,
) *Engine {
	m := make(map[string]Executor, len(execs))
	for _, ex := range execs {
		m[ex.Slug()] = ex
	}
	return &Engine{
		integrations: integrationsStore,
		executors:    m,
		queue:        queue,
		notifs:       notifs,
		threshold:    threshold,
	}
}

// DefaultExecutors returns all built-in executors.
func DefaultExecutors() []Executor {
	return []Executor{
		&GmailExecutor{},
		&OutlookMailExecutor{},
		&GoogleCalendarExecutor{},
		&OutlookCalendarExecutor{},
		&StripeExecutor{},
		&IntaSendExecutor{},
	}
}

// MicroWalletThreshold reads MICROWALLET_THRESHOLD from the environment (default 50.0).
func MicroWalletThreshold() float64 {
	if v := os.Getenv("MICROWALLET_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return 50.0
}

// Process is called by the pipeline for every signal where eval.Execute=true.
//
//   - ActionNone → no-op, returns (false, nil); caller keeps Decision=Automated
//   - IsAutoExecutable → fetch creds, execute; returns (false, nil) on success
//   - !IsAutoExecutable OR executor error → enqueue + notify; returns (true, nil)
func (e *Engine) Process(ctx context.Context, action Action, eventID int) (queued bool, err error) {
	if action.Type == ActionNone {
		return false, nil
	}

	// Special case: IntaSend above threshold — initiate now, approve later.
	if action.ServiceSlug == "intasend" && !IsAutoExecutable(action, e.threshold) {
		return e.initiateIntaSend(ctx, action, eventID)
	}

	if IsAutoExecutable(action, e.threshold) {
		executor, ok := e.executors[action.ServiceSlug]
		if ok {
			creds, credErr := e.fetchCreds(action.ServiceSlug)
			if credErr == nil {
				if execErr := executor.Execute(ctx, creds, action); execErr != nil {
					log.Printf("execution: [%s] executor failed: %v — queueing for manual review", action.ServiceSlug, execErr)
				} else {
					return false, nil // executed successfully
				}
			} else {
				log.Printf("execution: [%s] credential lookup failed: %v — queueing", action.ServiceSlug, credErr)
			}
		}
	}

	return e.enqueue(action, eventID)
}

// ApproveAndExecute is called by the approval handler on POST /brain/queue/:id/approve.
// It re-executes the action (fetching fresh credentials), then marks the entry as executed.
func (e *Engine) ApproveAndExecute(ctx context.Context, entryID int) error {
	entry, err := e.queue.Get(entryID)
	if err != nil {
		return err
	}
	if entry.Status != "pending" {
		return fmt.Errorf("queue entry %d is not pending (status=%s)", entryID, entry.Status)
	}

	// IntaSend approve path: tracking_id was stored during pipeline initiate step.
	if entry.ServiceSlug == "intasend" {
		if trackingID, ok := entry.ResourceMeta["tracking_id"]; ok && trackingID != "" {
			creds, credErr := e.fetchCreds("intasend")
			if credErr != nil {
				_ = e.queue.SetStatus(entryID, "failed")
				return credErr
			}
			isExec, ok := e.executors["intasend"].(*IntaSendExecutor)
			if !ok {
				_ = e.queue.SetStatus(entryID, "failed")
				return fmt.Errorf("intasend executor not registered")
			}
			if approveErr := isExec.ApproveByTrackingID(ctx, creds, trackingID); approveErr != nil {
				_ = e.queue.SetStatus(entryID, "failed")
				return approveErr
			}
			return e.queue.SetStatus(entryID, "executed")
		}
	}

	// Normal executor path.
	executor, ok := e.executors[entry.ServiceSlug]
	if !ok {
		_ = e.queue.SetStatus(entryID, "failed")
		return fmt.Errorf("no executor for service %q", entry.ServiceSlug)
	}

	creds, credErr := e.fetchCreds(entry.ServiceSlug)
	if credErr != nil {
		_ = e.queue.SetStatus(entryID, "failed")
		return credErr
	}

	action := Action{
		Type:           ActionType(entry.ActionType),
		ServiceSlug:    entry.ServiceSlug,
		ResourceID:     entry.ResourceID,
		ResourceMeta:   entry.ResourceMeta,
		Reason:         entry.Reason,
		EstimatedCost:  entry.EstimatedCost,
		ReputationRisk: entry.ReputationRisk,
	}

	if execErr := executor.Execute(ctx, creds, action); execErr != nil {
		_ = e.queue.SetStatus(entryID, "failed")
		return execErr
	}

	return e.queue.SetStatus(entryID, "executed")
}

// initiateIntaSend calls the initiate endpoint and stores the tracking_id in the queue entry.
func (e *Engine) initiateIntaSend(ctx context.Context, action Action, eventID int) (bool, error) {
	isExec, ok := e.executors["intasend"].(*IntaSendExecutor)
	if !ok {
		return e.enqueue(action, eventID) // fall back to normal queue
	}

	creds, credErr := e.fetchCreds("intasend")
	if credErr != nil {
		return e.enqueue(action, eventID) // can't initiate — queue for manual
	}

	trackingID, initErr := isExec.Initiate(ctx, creds, action)
	if initErr != nil {
		log.Printf("execution: intasend initiate failed: %v — queueing without tracking_id", initErr)
		return e.enqueue(action, eventID)
	}

	// Initiate succeeded: store tracking_id in resource_meta so approve works.
	meta := copyMeta(action.ResourceMeta)
	meta["tracking_id"] = trackingID
	action.ResourceMeta = meta

	queued, err := e.enqueue(action, eventID)
	if err != nil {
		return queued, err
	}
	return queued, nil
}

// enqueue adds the action to the approval queue and creates a notification.
func (e *Engine) enqueue(action Action, eventID int) (bool, error) {
	entry := QueueEntry{
		EventID:        eventID,
		ActionType:     string(action.Type),
		ServiceSlug:    action.ServiceSlug,
		ResourceID:     action.ResourceID,
		ResourceMeta:   action.ResourceMeta,
		Reason:         action.Reason,
		EstimatedCost:  action.EstimatedCost,
		ReputationRisk: action.ReputationRisk,
	}
	if _, qErr := e.queue.Enqueue(entry); qErr != nil {
		log.Printf("execution: enqueue failed: %v", qErr)
		return false, qErr
	}

	e.notifs.Create(notifications.Notification{ //nolint:errcheck
		Type:  notifications.TypeQueueEntry,
		Title: fmt.Sprintf("Action queued: %s", action.Type),
		Body:  fmt.Sprintf("[%s] %s", action.ServiceSlug, action.Reason),
	})

	return true, nil
}

// fetchCreds retrieves decrypted credentials for the named service slug.
func (e *Engine) fetchCreds(slug string) (datasync.Credentials, error) {
	svc, err := e.integrations.GetConnectedBySlug(slug)
	if err != nil {
		return datasync.Credentials{}, fmt.Errorf("execution: fetch creds for %s: %w", slug, err)
	}
	return datasync.Credentials{
		APIKey:           svc.APIKey,
		APIEndpoint:      svc.APIEndpoint,
		OAuthAccessToken: svc.OAuthAccessToken,
	}, nil
}
