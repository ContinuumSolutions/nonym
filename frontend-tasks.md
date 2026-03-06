# Frontend Tasks & API Contract Notes

This document captures the current EK-1 system after major simplification and cleanup.
Updated as the backend evolves — check this file after every backend session.

---

## System Overview: UI Pivot Complete

EK-1 has been simplified from a complex decision/reputation system to a focused **signal analysis tool** that helps users manage communications intelligently.

**Core Flow:** Emails, calendar invites, notifications → AI analysis → categorization + reply drafts + smart prioritization

---

## 1. Authentication System — JWT with PIN

### New Authentication Flow
EK-1 now uses **PIN-based JWT authentication** instead of sessions. All protected endpoints require JWT tokens.

### Authentication Endpoints
```
POST /api/v1/auth/pin/setup        # First-time PIN creation
POST /api/v1/auth/login            # Login with PIN → JWT token
POST /api/v1/auth/pin/status       # Check if PIN is configured
PUT  /api/v1/auth/pin/change       # Change PIN (requires current PIN)
POST /api/v1/auth/logout           # Logout (blacklist token)
DELETE /api/v1/auth/pin            # Remove PIN protection
```

### PIN Setup Flow (First Time Users)
```json
// POST /auth/pin/setup
{
  "pin": "123456"  // 6-digit string
}

// Response
{
  "message": "PIN setup successful",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

### Login Flow
```json
// POST /auth/pin/login
{
  "pin": "123456"
}

// Success (200)
{
  "message": "Login successful",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": "2026-03-13T10:30:00Z"
}

// Failed (401)
{
  "error": "Invalid PIN"
}
```

### Token Usage
- Include in all requests: `Authorization: Bearer <token>`
- Tokens expire after 7 days
- Invalid/expired tokens return 401
- Refresh by logging in again

### Rate Limiting
- **5 attempts per 15 minutes** for PIN operations
- After 5 failed attempts: 15 minute lockout
- Returns 429 with `retry_after` seconds

---

## 2. Signals System — New Core API

### What Replaced What
- ❌ **Removed:** `/activities/events`, `/brain/events`, `/brain/status`, `/brain/queue`
- ❌ **Removed:** Activities, Events, Decisions, Reputation, Ledger, Harvest, Execution
- ✅ **New:** `/signals` API with AI-analyzed communications
- ✅ **New:** `/replies` API for AI-generated draft responses

### Core Signals Endpoints
```
GET  /api/v1/signals                    # All analyzed signals
GET  /api/v1/signals/relevant           # High/medium priority needing attention
GET  /api/v1/signals/replies            # Signals that need replies
GET  /api/v1/signals/summary            # Dashboard counts
PUT  /api/v1/signals/:id/status         # Mark done/ignored/snoozed

GET  /api/v1/replies/pending            # AI-generated drafts ready to send
PUT  /api/v1/replies/:id                # Approve/edit/reject draft replies
```

### Signal Data Structure
```json
{
  "id": 123,
  "raw_signal": {
    "service_slug": "gmail",
    "category": "Communication",
    "title": "Contract renewal from Alice",
    "body": "Hi, following up on...",
    "metadata": {
      "from": "alice@example.com",
      "message_id": "abc123"
    },
    "occurred_at": "2026-03-06T10:30:00Z"
  },
  "analysis": {
    "category": "relevant",           // relevant|newsletter|automated|notification
    "priority": "high",               // high|medium|low
    "is_relevant": true,
    "needs_reply": true,
    "reasoning": "Contract deadline approaching, requires action",
    "suggested_action": "Review terms and respond by Thursday",
    "reply_draft": "Hi Alice,\n\nThanks for...",
    "reply_tone": "professional",     // professional|casual|friendly|formal
    "summary": "Client needs contract renewal response by Friday"
  },
  "status": "pending",                // pending|done|ignored|snoozed
  "created_at": "2026-03-06T10:30:05Z",
  "updated_at": "2026-03-06T10:30:05Z"
}
```

### Signal Categories
Replace complex event types with simple categories:
- **relevant** — needs user attention (personal emails, deadlines, important notifications)
- **newsletter** — automated marketing/updates (can be archived)
- **automated** — system notifications, receipts, confirmations
- **notification** — service alerts, status updates

### Signal Status Management
```json
// PUT /signals/:id/status
{
  "status": "done"  // pending|done|ignored|snoozed
}
```

### Dashboard Summary
```json
// GET /signals/summary
{
  "relevant_count": 5,        // High/medium priority needing attention
  "replies_needed": 3,        // Signals requiring responses
  "total_pending": 12,        // All unprocessed signals
  "last_sync": "2026-03-06T10:25:00Z"
}
```

---

## 3. Reply Drafting System

### AI-Generated Replies
The system automatically generates draft replies for signals that `needs_reply: true`.

### Reply Endpoints
```json
// GET /replies/pending — List all pending drafts
[
  {
    "id": 456,
    "signal_id": 123,
    "content": "Hi Alice,\n\nThanks for following up...",
    "tone": "professional",
    "status": "pending",        // pending|approved|edited|rejected|sent
    "created_at": "2026-03-06T10:30:05Z"
  }
]

// PUT /replies/:id — Approve/edit/reject draft
{
  "action": "approve",        // approve|edit|reject
  "content": "Modified draft content..."  // Required if action=edit
}
```

### Reply Actions
- **approve** — Mark ready to send (frontend implements actual sending)
- **edit** — Update content, mark as approved
- **reject** — Discard draft, mark signal as done

---

## 4. Biometrics Integration — Simplified

### What's Kept
Biometrics are **simplified but preserved** for signal prioritization and user focus management.

### Biometrics Endpoints
```
GET /api/v1/biometrics/checkin
PUT /api/v1/biometrics/checkin
```

### How Biometrics Affect Signals
The system applies biometric-based prioritization:
- **High stress (>7) OR low sleep (<5)** → Enhanced filtering
- Medium priority signals get upgraded to high if they contain urgent keywords
- Low energy states reduce signal processing noise

### Biometrics Data Structure
```json
{
  "id": 1,
  "mood": 7,                  // 1-10
  "stress_level": 8,          // 1-10
  "sleep": 4.5,               // hours (float)
  "energy": 6,                // 1-10
  "updated_at": "2026-03-06T08:00:00Z"
}
```

---

## 5. Chat System — Temporarily Disabled

### Current Status
- **Chat endpoints exist** but are **temporarily disabled** during cleanup
- Chat handler was removed from main.go routes
- Will be re-enabled with simplified intents focused on signal management

### Planned Chat Commands (When Re-enabled)
- "What should I focus on today?" → Shows relevant signals
- "Draft a reply to Alice" → Creates email draft
- "What's urgent?" → High-priority items only
- "Mark email from Bob as done" → Update signal status

---

## 6. User Preferences — Simplified

### Profile Endpoints (Unchanged)
```
GET /api/v1/profile
PUT /api/v1/profile/preferences
PUT /api/v1/profile/connection
```

### Simplified Preferences
Removed complex decision thresholds. Now used only for signal analysis tuning:

```json
{
  "preferences": {
    "time_sovereignty": 7,      // 1-10, affects time-sensitive filtering
    "financial_growth": 6,      // 1-10, affects ROI thresholds
    "health_recovery": 5,       // 1-10, affects stress-based filtering
    "reputation_building": 8,   // 1-10, affects networking signal priority
    "privacy_protection": 5,    // 1-10, affects data sharing warnings
    "autonomy": 6               // 1-10, affects auto-processing vs manual review
  }
}
```

### Connection Settings
```json
{
  "kernel_name": "EK-1",
  "api_endpoint": "https://genesis.egokernel.com",
  "timezone": "America/New_York"
}
```

---

## 7. Service Integrations — Unchanged

### Integration Endpoints (Preserved)
```
GET    /integrations/services
GET    /integrations/services/:id
POST   /integrations/services/:id/connect
PUT    /integrations/services/:id/connect
DELETE /integrations/services/:id/connect
```

OAuth flow and service management remain the same.

---

## 8. Scheduler — Simplified

### Scheduler Endpoints (Simplified)
```
GET  /scheduler/status
POST /scheduler/run-now
```

### What Changed
- **Removed:** Complex brain pipeline status, reputation tracking
- **Kept:** Sync scheduling, signal processing status
- **New:** Shows signal processing results instead of decision outcomes

### Status Response
```json
{
  "running": false,
  "interval_minutes": 15,
  "last_run_at": "2026-03-06T10:15:00Z",
  "next_run_at": "2026-03-06T10:30:00Z",
  "last_signal_count": 12,
  "last_result": {
    "processed_ok": 12,
    "relevant_signals": 5,
    "replies_generated": 3,
    "errors": 0
  },
  "services": [...]  // Service connection status
}
```

---

## 9. Health Check — Unchanged

```
GET /health
```

Returns `{"status": "ok"}` for system monitoring.

---

## Frontend Migration Tasks

### 1. Authentication UI
- [ ] **Create PIN setup screen** — 6-digit PIN entry for first-time users
- [ ] **Create login screen** — PIN entry with error handling and rate limiting
- [ ] **Add token management** — Store JWT, handle expiration, logout
- [ ] **Protect all routes** — Redirect to login if no valid token

### 2. Dashboard Redesign
- [ ] **Remove brain/reputation widgets** — No more H2HI, reputation scores, complex status
- [ ] **Create "Needs Attention" list** — High/medium priority signals from `/signals/relevant`
- [ ] **Add reply management interface** — Show pending drafts from `/replies/pending`
- [ ] **Build signal categories view** — Organized by relevant/newsletter/automated/notification
- [ ] **Add signal summary counters** — Use `/signals/summary` for dashboard stats

### 3. Signal Management
- [ ] **Update signal list view** — Replace events with signals, show category badges
- [ ] **Add signal status controls** — Mark done/ignored/snoozed buttons
- [ ] **Show AI analysis** — Display reasoning, suggested actions, summaries
- [ ] **Implement signal filtering** — By category, priority, status, service

### 4. Reply Drafting
- [ ] **Create reply review interface** — Show AI-generated drafts with approve/edit/reject
- [ ] **Add draft editing** — Rich text editor for modifying AI content
- [ ] **Implement reply sending** — Frontend handles actual email sending after approval
- [ ] **Show reply status** — Track pending/approved/sent states

### 5. Settings Simplification
- [ ] **Remove reputation settings** — No more decision thresholds, reputation tracking
- [ ] **Simplify preferences UI** — Focus on signal analysis tuning (6 sliders, 1-10 scale)
- [ ] **Keep biometrics form** — Mood, stress, sleep, energy (preserved for signal prioritization)
- [ ] **Preserve connection settings** — Kernel name, API endpoint, timezone

### 6. Remove Obsolete Features
- [ ] **Delete activities/events pages** — Replace with signals dashboard
- [ ] **Remove brain status displays** — No more kernel state, entropy, H2HI
- [ ] **Delete reputation displays** — No more scores, ledger, social debt tracking
- [ ] **Remove execution queue** — No more approval workflows
- [ ] **Clean up navigation** — Update menu items to reflect new structure

### 7. Chat Integration (Future)
- [ ] **Disable chat temporarily** — Remove chat routes until re-implementation
- [ ] **Plan simplified intents** — Focus on signal queries, not complex system state
- [ ] **Design signal-focused commands** — "What's urgent?", "Draft reply to X", etc.

---

## Error Handling Updates

### New Error Patterns
- **401 Unauthorized** — Invalid/expired JWT token → redirect to login
- **429 Rate Limited** — Too many PIN attempts → show lockout timer
- **404 Signal Not Found** — Signal may have been processed → refresh list
- **422 Invalid Status** — Bad signal status transition → show valid options

### Removed Error Patterns
- ❌ Brain state errors (H2HI, entropy)
- ❌ Reputation calculation failures
- ❌ Complex decision validation errors
- ❌ Execution queue conflicts

---

## Data Migration Notes

### What to Update in Existing Frontend Code
1. **Replace all `/activities/*` calls** with `/signals/*`
2. **Replace all `/brain/*` calls** with appropriate `/signals/*` or remove
3. **Add JWT token to all requests** in Authorization header
4. **Update data models** — Event → Signal, Decision → Status
5. **Remove reputation/ledger components** — No longer exists in backend

### Backward Compatibility
- **None** — This is a breaking change requiring frontend rewrite
- Old API endpoints will return 404
- Data structures are completely different

---

## Success Metrics

### Signal Analysis Quality
- **Relevance accuracy:** % of "relevant" signals user actually acts on
- **Reply usage:** % of drafted replies sent (with/without edits)
- **Time saved:** Reduced time from signal → action
- **Category accuracy:** % of auto-categorized signals correctly sorted

### User Experience
- **Authentication friction:** Login success rate, PIN reset requests
- **Feature adoption:** Usage of reply drafts, signal status management
- **System responsiveness:** Signal processing speed, UI responsiveness