package chat

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/egokernel/ek1/internal/activities"
	"github.com/egokernel/ek1/internal/ai"
	"github.com/egokernel/ek1/internal/biometrics"
	"github.com/egokernel/ek1/internal/brain"
	"github.com/egokernel/ek1/internal/datasync"
	"github.com/egokernel/ek1/internal/harvest"
	"github.com/egokernel/ek1/internal/integrations"
	"github.com/egokernel/ek1/internal/ledger"
	"github.com/egokernel/ek1/internal/notifications"
	"github.com/egokernel/ek1/internal/profile"
	"github.com/egokernel/ek1/internal/scheduler"
	"github.com/gofiber/fiber/v2"
	_ "modernc.org/sqlite"
)

// ── mock Chatter ──────────────────────────────────────────────────────────────

// stubChatter records the last call and returns a preset reply.
type stubChatter struct {
	reply        string
	err          error
	lastSystem   string
	lastTurns    []ai.ChatTurn
}

func (s *stubChatter) Chat(_ context.Context, system string, turns []ai.ChatTurn) (string, error) {
	s.lastSystem = system
	s.lastTurns = turns
	return s.reply, s.err
}

// ── helpers ───────────────────────────────────────────────────────────────────

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newTestHandler(t *testing.T, chatter Chatter) *Handler {
	t.Helper()

	prefs := profile.DecisionPreference{
		TimeSovereignty: 5, FinacialGrowth: 5, HealthRecovery: 5,
		ReputationBuilding: 5, PrivacyProtection: 5, Autonomy: 5,
	}
	l := ledger.NewLocalLedger()
	l.Initialize("test")
	brainSvc := brain.NewService("test", prefs, l)

	profStore := profile.NewStore(openDB(t))
	profStore.Migrate()

	bioStore := biometrics.NewStore(openDB(t))
	bioStore.Migrate()

	actStore := activities.NewStore(openDB(t))
	actStore.Migrate()

	notifsStore := notifications.NewStore(openDB(t))
	notifsStore.Migrate()

	harvestStore := harvest.NewStore(openDB(t))
	harvestStore.Migrate()

	// scheduler — empty engine, noop analyser
	intKey := make([]byte, 32)
	intStore := integrations.NewStore(openDB(t), intKey)
	intStore.Migrate()
	engine := datasync.NewEngine(intStore, nil)
	pipeline := brain.NewPipeline(brainSvc, &noopAnalyser{}, actStore, bioStore, nil)
	sched := scheduler.NewScheduler(engine, pipeline, brainSvc, notifsStore, time.Minute)

	historyStore := NewHistoryStore(openDB(t))
	historyStore.Migrate()

	return NewHandler(chatter, brainSvc, profStore, bioStore, actStore, l, notifsStore, harvestStore, sched, historyStore, "test")
}

// noopAnalyser satisfies brain.Analyser without an Ollama server.
type noopAnalyser struct{}

func (n *noopAnalyser) AnalyseBatch(_ context.Context, signals []datasync.RawSignal) ([]*ai.AnalysedSignal, []error) {
	return make([]*ai.AnalysedSignal, len(signals)), make([]error, len(signals))
}

func setupApp(t *testing.T, chatter Chatter) *fiber.App {
	t.Helper()
	app := fiber.New()
	newTestHandler(t, chatter).RegisterRoutes(app)
	return app
}

// ── POST /chat ────────────────────────────────────────────────────────────────

func TestChat_EmptyBodyReturns400(t *testing.T) {
	app := setupApp(t, &stubChatter{reply: "ok"})
	req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestChat_EmptyMessageReturns400(t *testing.T) {
	app := setupApp(t, &stubChatter{reply: "ok"})
	body, _ := json.Marshal(Request{Message: "   "})
	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
	var m map[string]any
	json.NewDecoder(resp.Body).Decode(&m)
	if _, ok := m["error"]; !ok {
		t.Error("want error field in 400 response")
	}
}

func TestChat_ValidMessageReturns200WithReply(t *testing.T) {
	stub := &stubChatter{reply: "Hello from kernel"}
	app := setupApp(t, stub)

	body, _ := json.Marshal(Request{Message: "What's my status?"})
	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}

	var r Response
	json.NewDecoder(resp.Body).Decode(&r)
	if r.Reply != "Hello from kernel" {
		t.Errorf("want reply %q, got %q", "Hello from kernel", r.Reply)
	}
	if r.Timestamp.IsZero() {
		t.Error("want non-zero Timestamp in response")
	}
}

func TestChat_ChatterErrorReturns500(t *testing.T) {
	stub := &stubChatter{err: fmt.Errorf("ollama offline")}
	app := setupApp(t, stub)

	body, _ := json.Marshal(Request{Message: "ping"})
	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestChat_NewMessageAppendedAsLastTurn(t *testing.T) {
	stub := &stubChatter{reply: "noted"}
	app := setupApp(t, stub)

	body, _ := json.Marshal(Request{Message: "What is my energy level?"})
	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	app.Test(req)

	if len(stub.lastTurns) == 0 {
		t.Fatal("want at least one turn, got none")
	}
	last := stub.lastTurns[len(stub.lastTurns)-1]
	if last.Role != "user" {
		t.Errorf("last turn role: want %q, got %q", "user", last.Role)
	}
	if last.Content != "What is my energy level?" {
		t.Errorf("last turn content: want the user message, got %q", last.Content)
	}
}

func TestChat_KernelRoleRemappedToAssistant(t *testing.T) {
	stub := &stubChatter{reply: "ok"}
	app := setupApp(t, stub)

	history := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "kernel", Content: "Hi there"},
	}
	body, _ := json.Marshal(Request{Message: "How are you?", History: history})
	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	app.Test(req)

	// Turns = history[0], history[1], new user message → 3 total
	if len(stub.lastTurns) != 3 {
		t.Fatalf("want 3 turns, got %d", len(stub.lastTurns))
	}
	if stub.lastTurns[1].Role != "assistant" {
		t.Errorf("history 'kernel' role: want remapped to 'assistant', got %q", stub.lastTurns[1].Role)
	}
}

func TestChat_UserRolePreservedInHistory(t *testing.T) {
	stub := &stubChatter{reply: "ok"}
	app := setupApp(t, stub)

	history := []Message{{Role: "user", Content: "First message"}}
	body, _ := json.Marshal(Request{Message: "Second message", History: history})
	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	app.Test(req)

	if stub.lastTurns[0].Role != "user" {
		t.Errorf("user role should be preserved, got %q", stub.lastTurns[0].Role)
	}
}

func TestChat_SystemPromptSentToChatter(t *testing.T) {
	stub := &stubChatter{reply: "ok"}
	app := setupApp(t, stub)

	body, _ := json.Marshal(Request{Message: "hi"})
	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	app.Test(req)

	if stub.lastSystem == "" {
		t.Error("want non-empty system prompt sent to Chatter")
	}
	if !strings.Contains(stub.lastSystem, "EK-1") {
		t.Error("system prompt should mention EK-1")
	}
}

// ── buildSystemPrompt ─────────────────────────────────────────────────────────

func TestBuildSystemPrompt_ContainsRequiredSections(t *testing.T) {
	h := newTestHandler(t, &stubChatter{reply: "ok"})
	prompt := h.buildSystemPrompt()

	sections := []string{"## Identity", "## Kernel State", "## Reputation", "## Biometrics", "## Recent Activity"}
	for _, s := range sections {
		if !strings.Contains(prompt, s) {
			t.Errorf("system prompt missing section %q", s)
		}
	}
}

func TestBuildSystemPrompt_ContainsCurrentTime(t *testing.T) {
	h := newTestHandler(t, &stubChatter{reply: "ok"})
	prompt := h.buildSystemPrompt()
	year := fmt.Sprintf("%d", time.Now().UTC().Year())
	if !strings.Contains(prompt, year) {
		t.Errorf("system prompt should contain current year %s", year)
	}
}

func TestBuildSystemPrompt_EmptyStoresDoNotPanic(t *testing.T) {
	h := newTestHandler(t, &stubChatter{reply: "ok"})
	// Must not panic when all stores are empty.
	prompt := h.buildSystemPrompt()
	if prompt == "" {
		t.Error("want non-empty system prompt even with empty stores")
	}
}
