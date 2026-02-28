# EK-1 Implementation Plan

This plan covers the full path from the current API foundation to the three-stage vision in the README.
Each step is numbered for reference. When approving work, reference the step number (e.g. "implement step 4").

---

## Current State

The Fiber API is running. The following packages exist but are not wired to the API or to each other:

- `brain/` — `EgoKernel`, `ValueMatrix`, `TradeEngine`, soul-drift, identity entropy
- `ledger/` — `LocalLedger` (in-memory), reputation scoring with temporal decay, tiers
- `protocols/` — `Titan Handshake` (3-tier: ZK → Nash auction → Escalation + MAS)
- `profile/` — `EKProgress`, `ConnectionSetting`, `DecisionPreference`
- `scripts/harvest/` — `Scanner` (demo only, hardcoded contacts)
- `internal/integrations/` — service connections, OAuth2/API key, AES-encrypted credentials
- `internal/activities/` — `Event` model (written by brain, read by user)
- `internal/biometrics/` — single-row daily check-in

---

## Stage 1 — The Shadow
> *"What would your best self have decided today?"* — read-only mode, no actions taken.

---

### 1. User Profile API
**What:** Persist `DecisionPreference` and `ConnectionSetting` (from `profile/`) to the database and expose them via API. This is the user's "values" configuration — the foundation everything else runs from.

**Routes:**
- `GET  /profile` — return current profile (preferences + connection settings)
- `PUT  /profile/preferences` — update `DecisionPreference` weights
- `PUT  /profile/connection` — update `ConnectionSetting` (kernel name, timezone)

**Why now:** Every other system (brain, harvest, scheduler) needs a persisted value configuration to initialise. Nothing else meaningful can run without it.

---

### 2. Brain Initialisation — Profile → ValueMatrix
**What:** On startup (and on profile update), translate the stored `DecisionPreference` into the brain's `ValueMatrix` and start the `EgoKernel`. The mapping is:

| DecisionPreference field | ValueMatrix field |
|---|---|
| `TimeSovereignty` | `TemporalSovereignty` |
| `FinancialGrowth` | `UtilityThreshold` (inverse) |
| `HealthRecovery` | biometrics gate (see step 8) |
| `ReputationBuilding` | `ReputationImpact` |
| `PrivacyProtection` | `RiskTolerance` (inverse) |
| `Autonomy` | `SocialEntropy` (inverse) |

**Routes:**
- `GET  /brain/status` — return `KernelStatus`, decision count, identity entropy, current tier
- `POST /brain/sync-acknowledge` — call `AcknowledgeManualSync()` when user clears H2HI alert

**Why now:** The kernel must be running before any data can be triaged or decided on.

---

### 3. Ledger Persistence — SQLite (Phase 1/2)
**What:** Replace `LocalLedger` (in-memory map) with a SQLite-backed ledger. `InteractionRecord` rows persist across restarts. The scoring formula and tier logic stay unchanged.

**Schema:**
```sql
CREATE TABLE reputation_events (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    uid        TEXT    NOT NULL,
    success    INTEGER NOT NULL,
    impact     INTEGER NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
```

**Routes:**
- `GET /ledger/score` — current score + tier + trust tax
- `GET /ledger/history` — paginated list of interaction records

**Why now:** Steps 4–9 all produce reputation events. In-memory storage means every restart wipes the history.

---

### 4. Data Sync Engine — Pull from Connected Services
**What:** A `sync` package that uses the stored OAuth tokens / API keys from `integrations` to pull raw data from each installed service. Each service adapter reads data in read-only mode and normalises it into a common `RawSignal` struct.

**Initial adapters (one Go file per service):**
- `gmail.go` — fetch unread emails (subject, sender, body snippet, timestamp)
- `google_calendar.go` — fetch upcoming events (title, attendees, duration, time)
- `outlook_mail.go` / `outlook_calendar.go` — same shape as Google adapters
- `slack.go` — fetch unread DMs and mentions
- `plaid.go` — fetch recent transactions (merchant, amount, category)
- `stripe.go` — fetch recent charges and subscription changes
- `oura.go` / `fitbit.go` / `whoop.go` — fetch last night's sleep + HRV + recovery score

**Common output:**
```go
type RawSignal struct {
    ServiceSlug string
    Category    string    // maps to activities.EventType
    Title       string
    Body        string
    Metadata    map[string]string
    OccurredAt  time.Time
}
```

**Why now:** All AI analysis and brain decisions are downstream of this raw data. Nothing can run without it.

---

### 5. Local AI Integration — Ollama / Llama
**What:** Integrate a locally running LLM (via [Ollama](https://ollama.com)) to analyse `RawSignal` batches. Ollama exposes an OpenAI-compatible REST API at `localhost:11434` — no external calls, fully private.

**What the LLM does per signal:**
1. Classify the signal into `EventType` (Finance / Calendar / Communication / Billing / Health)
2. Assign `Importance` (Low / Medium / High) based on content
3. Estimate `EstimatedROI` and `TimeCommitment` (used by the brain's `Triage`)
4. Detect manipulation score (`ManipulationPct`) — guilt language, urgency traps, false scarcity
5. Write a `Narrative` — one sentence describing what happened and why it matters
6. Identify a `Gain` if present (saved time, saved money, earned favour)

**Output:** An `IncomingRequest` (for triage) + partial `Event` struct ready for the activities table.

**Config added to `.env-temp`:**
```
OLLAMA_HOST=http://localhost:11434
OLLAMA_MODEL=llama3.2
```

**Why now:** The LLM is the bridge between raw service data and structured decisions. Steps 6–8 depend on it.

---

### 6. Brain Pipeline — Triage → Decide → Write Events
**What:** Wire the data flow: `RawSignal → LLM analysis → IncomingRequest → EgoKernel.Triage() → TradeOpportunity → EgoKernel.Decide() → activities.Event`.

**Flow:**
```
Sync Engine
    └── RawSignal[]
           └── LLM (step 5) → IncomingRequest + partial Event
                  └── EgoKernel.Triage()
                         ├── GHOST  → Event{Decision: Declined, Narrative: "Ghosted: manipulation detected"}
                         ├── REJECT → Event{Decision: Declined, Narrative: "Rejected: ROI below threshold"}
                         └── ACCEPT → EgoKernel.Decide(TradeOpportunity)
                                ├── REJECT → Event{Decision: Declined}
                                └── EXECUTE → Event{Decision: Automated} + reputation LogSuccess
```

Every outcome writes an `Event` row to the `activities` table. In Stage 1 (Shadow), `Decision: Automated` means "would have acted" — nothing is actually executed.

**Routes:**
- `GET /brain/events` — same as `/activities/events` but filtered to brain-generated events (alias)

---

### 7. Harvest Engine — Real Social Graph Scanner
**What:** Replace the hardcoded demo contacts in `scripts/harvest/scan.go` with a real implementation inside `internal/harvest/`. It pulls actual contact data from connected services:

- **Gmail / Outlook** — parse email threads to count favour exchanges (you helped them = `FavorsGiven`, they helped you = `FavorsReceived`)
- **Calendar** — count meetings initiated by each contact vs. meetings that produced outcomes
- **Slack** — track response times and message-initiation ratios

The LLM (step 5) reads email/message content to classify each interaction as a favour, a request, or neutral. The existing `Scanner.Scan()` logic (social debt calculation, ghost-agreement detection) runs on this real data.

**Output:** Harvest results are written as `Event` rows with `EventType: Communication`, enriched `Narrative`, and a `Gain` representing the estimated social debt value.

**Route:**
- `POST /harvest/scan` — trigger a manual scan, returns `HarvestResult`
- `GET  /harvest/results` — last scan result

---

### 8. Biometrics Gate — Health-Aware Triage
**What:** Before the brain processes any signals, check the day's biometrics check-in. If `StressLevel > 7` or `Sleep < 5`, activate `StatusShielded` — the kernel filters more aggressively (raises `UtilityThreshold`) and cancels non-essential calendar events automatically (in Stage 2).

In Stage 1 this adds a `Narrative` flag to events: *"Note: kernel operating in reduced-load mode due to elevated stress."*

This wires `biometrics.CheckIn` → `brain.EgoKernel` for the first time.

---

### 9. Scheduler — Automated Sync Cycles
**What:** A background goroutine that runs the full pipeline (sync → LLM → brain → events) on a configurable interval. Default: every 15 minutes while the app is running.

**Config added to `.env-temp`:**
```
SYNC_INTERVAL_MINUTES=15
```

**Routes:**
- `GET  /scheduler/status` — last run time, next run time, signal count from last cycle
- `POST /scheduler/run-now` — trigger an immediate cycle (for testing)

---

### 10. Notification System — H2HI Alerts and Opportunities
**What:** When the brain enters `StatusH2HI` (identity entropy spike), or when a high-value opportunity is found (harvest debt > $10k, ghost-agreement > 95% overlap), the kernel needs to surface this to the user. In Stage 1 this is a simple in-app notifications table.

**Schema:**
```sql
CREATE TABLE notifications (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    type       TEXT    NOT NULL,  -- "H2HI", "OPPORTUNITY", "HARVEST", "SOUL_DRIFT"
    title      TEXT    NOT NULL,
    body       TEXT    NOT NULL,
    read       INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
```

**Routes:**
- `GET  /notifications` — unread notifications
- `PUT  /notifications/:id/read` — mark as read
- `PUT  /notifications/read-all` — mark all read

---

## Stage 2 — The Hand
> *"Handle the small stuff automatically."*

Steps 11–13 turn Shadow decisions into real actions. The kernel executes only decisions that are `AutoExecutable: true` in the `TradeEngine` — those with zero time commitment and deterministic outcomes.

---

### 11. Execution Layer — Act on Decisions
**What:** When the brain returns `Execute: true` and the signal is `AutoExecutable`, the execution layer calls back into the originating service adapter to perform the action:

| Signal type | Automated action |
|---|---|
| Late delivery detected | Call service API to request refund |
| Subscription price increase | Cancel subscription via API |
| Low-value calendar invite | Decline and send templated response |
| Manipulation-flagged email | Archive and add sender to filter list |
| High-debt contact reaches out | Queue value-rebalance message for review |

Each execution writes the `Event` with `Decision: Automated` (real action taken, not shadow).

---

### 12. Micro-Wallet Integration
**What:** Add a small-balance wallet (Solana or Stripe) so the kernel can settle minor financial disputes (< configurable threshold, default $50) without user intervention. Plaid/Stripe adapters already pull transaction data — this adds write capability for refund claims and payment disputes.

---

### 13. User Approval Queue — Escalated Decisions
**What:** Not everything is auto-executable. When a decision exceeds the automation threshold (cost > $50, or `ReputationRisk > 0.3`), it is placed in an approval queue for the user to review and confirm.

**Routes:**
- `GET  /brain/queue` — pending decisions awaiting user approval
- `POST /brain/queue/:id/approve` — user approves, kernel executes
- `POST /brain/queue/:id/reject` — user rejects, kernel logs and moves on

---

## Stage 3 — The Voice
> *"Two digital selves negotiate faster than two humans can shake hands."*

---

### 14. P2P Kernel Discovery
**What:** EK-1 instances need to find each other. Each kernel exposes a public `ConnectionSetting.APIEndpoint` (from `profile/connection.go`). Implement a simple peer registry — either a DHT (decentralised) or a lightweight central registry during development.

---

### 15. Titan Handshake API
**What:** Expose the existing `protocols/handshake.go` logic as an HTTP endpoint so two kernels can negotiate over the network. The `Handshake.Execute()` logic is already complete — this step wraps it in a network-accessible API.

**Routes:**
- `POST /handshake/initiate` — start a handshake with a peer kernel (supply peer endpoint + resource params)
- `POST /handshake/respond` — receive and respond to an inbound handshake request
- `GET  /handshake/history` — log of past negotiations and outcomes

---

### 16. On-Chain Reputation — Solana Ledger
**What:** Replace `LocalLedger` (SQLite-backed from step 3) with Solana RPC calls to the `programs/ek-logic` on-chain Anchor program. Every `LogSuccess` and `LogBetrayal` call becomes a signed on-chain transaction. The `Score()` query reads from the chain.

The `ledger` package already notes this migration path in its comments. The API surface (`Score`, `Tier`, `LogSuccess`, `LogBetrayal`) does not change — only the backing store switches.

---

### 17. Zero-Knowledge Privacy Layer
**What:** Implement ZK proofs so the kernel can prove it acted correctly (honoured a contract, met a commitment) without revealing the underlying data (email content, financial details). Uses the existing Proof of Intent concept from the Titan Handshake Tier 1.

Library: `gnark` (Go-native ZK proof library).

---

## Architecture Overview (when complete)

```
┌─────────────────────────────────────────────────────────┐
│                    Fiber HTTP API                        │
│  /profile  /brain  /harvest  /ledger  /notifications     │
│  /integrations  /activities  /biometrics  /handshake     │
└────────────────────────┬────────────────────────────────┘
                         │
          ┌──────────────▼──────────────┐
          │         Scheduler            │  ← step 9
          │   (every N minutes)          │
          └──────────────┬──────────────┘
                         │
          ┌──────────────▼──────────────┐
          │       Sync Engine            │  ← step 4
          │  Gmail · Calendar · Plaid    │
          │  Slack · Stripe · Oura ...   │
          └──────────────┬──────────────┘
                         │  RawSignal[]
          ┌──────────────▼──────────────┐
          │     Local LLM (Ollama)       │  ← step 5
          │  classify · score · narrate  │
          └──────────────┬──────────────┘
                         │  IncomingRequest[]
          ┌──────────────▼──────────────┐
          │         EgoKernel            │  ← step 2/6
          │  Triage → Decide → Entropy   │
          └──────┬───────────────┬───────┘
                 │               │
    ┌────────────▼──┐    ┌───────▼────────────┐
    │ Activities DB  │    │  Execution Layer    │  ← step 11
    │ (Shadow log)   │    │  (Stage 2 actions)  │
    └───────────────┘    └───────┬─────────────┘
                                 │
                    ┌────────────▼────────────┐
                    │   Titan Handshake        │  ← step 15
                    │   (P2P negotiation)      │
                    └────────────┬────────────┘
                                 │
                    ┌────────────▼────────────┐
                    │   Solana On-Chain Ledger │  ← step 16
                    │   (tamper-proof rep)     │
                    └─────────────────────────┘
```

---

## Implementation Order Summary

| # | Step | Stage | Depends on |
|---|---|---|---|
| 1 | User Profile API | Foundation | — |
| 2 | Brain initialisation (Profile → ValueMatrix) | Foundation | 1 |
| 3 | Ledger persistence (SQLite) | Foundation | — |
| 4 | Data Sync Engine | Shadow | integrations |
| 5 | Local AI (Ollama/Llama) | Shadow | 4 |
| 6 | Brain pipeline (Triage → Events) | Shadow | 2, 5 |
| 7 | Harvest engine (real social graph) | Shadow | 4, 5 |
| 8 | Biometrics gate | Shadow | 2, biometrics API |
| 9 | Scheduler | Shadow | 4, 6 |
| 10 | Notification system | Shadow | 6, 7 |
| 11 | Execution layer | Hand | 6, 9 |
| 12 | Micro-wallet | Hand | 11 |
| 13 | User approval queue | Hand | 11 |
| 14 | P2P kernel discovery | Voice | 1 |
| 15 | Titan Handshake API | Voice | 3, 14 |
| 16 | Solana on-chain ledger | Voice | 15 |
| 17 | ZK privacy layer | Voice | 15 |
