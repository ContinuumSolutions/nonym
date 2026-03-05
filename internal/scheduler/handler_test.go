package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/egokernel/ek1/internal/activities"
	"github.com/egokernel/ek1/internal/ai"
	"github.com/egokernel/ek1/internal/biometrics"
	"github.com/egokernel/ek1/internal/brain"
	"github.com/egokernel/ek1/internal/datasync"
	"github.com/egokernel/ek1/internal/integrations"
	"github.com/egokernel/ek1/internal/ledger"
	"github.com/egokernel/ek1/internal/notifications"
	"github.com/egokernel/ek1/internal/profile"
	"github.com/gofiber/fiber/v2"
	_ "modernc.org/sqlite"
)

// noopAnalyser satisfies brain.Analyser for scheduler tests.
// It returns nil for every signal (which the pipeline skips gracefully).
type noopAnalyser struct{}

func (n *noopAnalyser) AnalyseBatch(_ context.Context, signals []datasync.RawSignal) ([]*ai.AnalysedSignal, []error) {
	return make([]*ai.AnalysedSignal, len(signals)), make([]error, len(signals))
}

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newTestScheduler(t *testing.T) *Scheduler {
	t.Helper()

	// integrations store — empty, no services installed
	intKey := make([]byte, 32)
	intStore := integrations.NewStore(openDB(t), intKey)
	intStore.Migrate()

	// datasync engine — no adapters
	engine := datasync.NewEngine(intStore, nil)

	// brain service
	prefs := profile.DecisionPreference{
		TimeSovereignty: 5, FinacialGrowth: 5, HealthRecovery: 5,
		ReputationBuilding: 5, PrivacyProtection: 5, Autonomy: 5,
	}
	l := ledger.NewLocalLedger()
	svc := brain.NewService("test-uid", prefs, l)

	// activities store
	actStore := activities.NewStore(openDB(t))
	actStore.Migrate()

	// biometrics store
	bioStore := biometrics.NewStore(openDB(t))
	bioStore.Migrate()

	// pipeline with noop analyser
	pipeline := brain.NewPipeline(svc, &noopAnalyser{}, actStore, bioStore, nil)

	// notifications store
	notifsStore := notifications.NewStore(openDB(t))
	notifsStore.Migrate()

	return NewScheduler(engine, pipeline, svc, notifsStore, time.Minute)
}

func setupSchedulerApp(t *testing.T) *fiber.App {
	t.Helper()
	s := newTestScheduler(t)
	app := fiber.New()
	NewHandler(s).RegisterRoutes(app)
	return app
}

func TestHandlerStatus_200(t *testing.T) {
	app := setupSchedulerApp(t)
	req := httptest.NewRequest(http.MethodGet, "/scheduler/status", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var status Status
	json.NewDecoder(resp.Body).Decode(&status)
	if status.IntervalMinutes != 1 {
		t.Errorf("want IntervalMinutes=1, got %d", status.IntervalMinutes)
	}
}

func TestHandlerStatus_LastRunAtNilBeforeFirstRun(t *testing.T) {
	app := setupSchedulerApp(t)
	req := httptest.NewRequest(http.MethodGet, "/scheduler/status", nil)
	resp, _ := app.Test(req)
	var m map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&m)
	if m["last_run_at"] != nil {
		t.Errorf("want last_run_at=null before first run, got %v", m["last_run_at"])
	}
}

func TestHandlerRunNow_202(t *testing.T) {
	app := setupSchedulerApp(t)
	req := httptest.NewRequest(http.MethodPost, "/scheduler/run-now", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("want 202, got %d", resp.StatusCode)
	}
	var m map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&m)
	if m["status"] != "started" {
		t.Errorf("want status=started, got %v", m["status"])
	}
}

func TestHandlerRunNow_UpdatesStatus(t *testing.T) {
	s := newTestScheduler(t)
	app := fiber.New()
	NewHandler(s).RegisterRoutes(app)

	// Fire async run-now
	req1 := httptest.NewRequest(http.MethodPost, "/scheduler/run-now", nil)
	app.Test(req1)

	// Poll until the background goroutine completes (max 5s).
	var m map[string]interface{}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		req := httptest.NewRequest(http.MethodGet, "/scheduler/status", nil)
		resp, _ := app.Test(req)
		json.NewDecoder(resp.Body).Decode(&m)
		running, _ := m["running"].(bool)
		if !running && m["last_run_at"] != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if m["last_run_at"] == nil {
		t.Error("want non-null last_run_at after RunNow")
	}
	if m["last_result"] == nil {
		t.Error("want non-null last_result after RunNow")
	}
}
