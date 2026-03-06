# UI Pivot: Simplified Signal Analysis

## Overview
EK-1 is pivoting from a complex decision/reputation system to a simple, focused signal analysis tool that helps users manage their communications intelligently.

## Core Functionality
**Input:** Emails, calendar invites, notifications
**Output:** Relevance analysis + reply drafts + smart categorization

## New User Flow

### 1. Signal Analysis Dashboard
Replace the complex "events" and "brain status" views with:

```
┌─ NEEDS YOUR ATTENTION (3) ─────────────────────────┐
│ 🔴 High Priority                                   │
│ • Email from Alice - Contract renewal deadline    │
│   → "This looks urgent - contract expires Friday" │
│   → [View Draft Reply] [Mark as Done]             │
│                                                    │
│ 🟡 Medium Priority                                 │
│ • Calendar: Team meeting moved to 3pm             │
│   → "Schedule conflict with your 3:30pm call"     │
│   → [Acknowledge] [Propose New Time]               │
│                                                    │
│ • Invoice from Vendor ($500) - Due in 5 days      │
│   → [Review] [Schedule Payment]                    │
└────────────────────────────────────────────────────┘

┌─ DRAFT REPLIES (2) ────────────────────────────────┐
│ To: Alice (Contract renewal)                       │
│ ┌─ Generated Reply ────────────────────────────────┐│
│ │ Hi Alice,                                       ││
│ │                                                 ││
│ │ Thanks for following up on the contract. I     ││
│ │ reviewed the terms and they look good. I can   ││
│ │ get this signed by Thursday.                    ││
│ │                                                 ││
│ │ Best regards,                                   ││
│ │ [Your name]                                     ││
│ └─────────────────────────────────────────────────┘│
│ [Edit Draft] [Send Now] [Skip]                     │
└────────────────────────────────────────────────────┘

┌─ AUTO-SORTED (15) ─────────────────────────────────┐
│ 📧 Newsletters (8)      💼 Automated (4)          │
│ 🔔 Notifications (3)                               │
│ [Review] [Mark All Read]                           │
└────────────────────────────────────────────────────┘
```

### 2. Simple Chat Interface
Instead of complex system prompts, focus on actionable queries:
- "What should I focus on today?" → Shows relevant signals
- "Draft a reply to Alice" → Creates email draft
- "What's urgent?" → High-priority items only

### 3. Signal Categories
Replace complex event types with simple categories:
- **Relevant** - needs user attention
- **Reply Needed** - requires a response
- **Newsletter** - automated content, low priority
- **Notification** - system/service updates
- **Automated** - bills, receipts, confirmations

## API Changes Needed

### New Endpoints
```
GET  /signals                    # All analyzed signals
GET  /signals/relevant           # Needs attention
GET  /signals/replies            # Drafts ready to send
GET  /signals/categories         # Auto-sorted by type
POST /signals/:id/reply          # Generate/edit draft reply
PUT  /signals/:id/status         # Mark done/ignored/etc.
```

### Simplified Data Structure
```json
{
  "signal": {
    "id": "email-123",
    "service": "gmail",
    "category": "relevant|newsletter|automated|notification",
    "priority": "high|medium|low",
    "title": "Contract renewal from Alice",
    "body": "Hi, following up on...",
    "analysis": {
      "is_relevant": true,
      "needs_reply": true,
      "reasoning": "Contract deadline approaching, requires action",
      "suggested_action": "Review terms and respond by Thursday"
    },
    "draft_reply": {
      "generated": true,
      "content": "Hi Alice,\n\nThanks for...",
      "tone": "professional"
    },
    "status": "pending|done|ignored",
    "created_at": "2024-03-06T10:30:00Z"
  }
}
```

## Removed Features
- ❌ Reputation ledger/scoring
- ❌ Complex decision states (Accept/Decline/Negotiate)
- ❌ Social debt tracking
- ❌ Gain calculations
- ❌ H2HI/entropy states
- ❌ Biometric shields
- ❌ Execution queues

## Kept Features
- ✅ Multi-service integration (Gmail, Calendar, etc.)
- ✅ Smart signal analysis via Ollama
- ✅ User preferences for personalization
- ✅ Chat interface for queries
- ✅ Automatic sync/scheduling

## Frontend Migration Tasks

### 1. Dashboard Redesign
- [ ] Remove brain status/reputation widgets
- [ ] Create "Needs Attention" priority list
- [ ] Add draft reply management interface
- [ ] Build category-based signal organization

### 2. API Integration
- [ ] Update to new `/signals` endpoints
- [ ] Remove references to events/decisions/gains
- [ ] Implement reply drafting workflow
- [ ] Add signal status management

### 3. Chat Improvements
- [ ] Simplify chat intents (focus on relevance, not complex data)
- [ ] Add reply generation commands
- [ ] Remove complex system status queries

### 4. Settings Simplification
- [ ] Keep user preferences for signal analysis
- [ ] Remove reputation/decision thresholds
- [ ] Add reply tone/style preferences

## Implementation Status

### ✅ Completed (Phase 1)
- **Simplified AI analysis schema** — focuses on relevance, categorization, reply drafting
- **New signals package** — `internal/signals/` with Signal, DraftReply models
- **Signals store** — SQLite storage with filtering, status management
- **Signals API handlers** — REST endpoints for signal management
- **Fixed Gmail base64 decoding** — emails now readable by AI
- **Ollama optimization** — `OLLAMA_KEEP_ALIVE=-1` for faster responses
- **Focused chat prompting** — embeds targeted data in user messages for better grounding

### 🔄 In Progress (Phase 2)
- **Chat handler integration** — signals store added to chat handlers (needs main.go wiring)
- **Focused intent handlers** — "what should I focus on" now uses signals (other intents need updating)

### ⏳ TODO (Phase 3 & 4)
- **Main.go integration** — wire signals store into the app startup
- **Remove old brain pipeline** — replace with simple signal processing loop
- **Update remaining chat intents** — financial, health, etc. to use signals
- **Frontend API migration** — update dashboard to use `/signals/*` endpoints
- **Reply drafting UI** — interface for managing AI-generated replies

### 🚧 Migration Notes

**To complete the backend:**
1. Add signals store to `main.go` startup
2. Create simple processing pipeline that calls `ai.AnalyseBatch()` → stores results in signals
3. Replace brain/activities endpoints with signals endpoints
4. Update chat focused handlers for remaining intents

**API Changes Made:**
```go
// New signals endpoints (implemented)
GET  /signals                    # List signals with filtering
GET  /signals/relevant           # Signals needing attention
GET  /signals/replies            # Signals needing replies
GET  /signals/summary            # Dashboard counts
PUT  /signals/:id/status         # Mark done/ignored/snoozed
GET  /replies/pending            # AI-generated drafts
PUT  /replies/:id                # Approve/edit/reject drafts

// Old endpoints (to be replaced)
GET  /activities/events          → /signals
GET  /brain/events              → /signals/relevant
GET  /brain/queue               → /replies/pending
```

**Data Model Changes:**
```go
// Before: Complex Event with decision states
type Event struct {
    Decision Decision  // Accepted/Declined/etc.
    Gain     Gain     // Complex financial tracking
    // ... many fields
}

// After: Simple Signal with AI analysis
type Signal struct {
    Analysis ai.AnalysedSignal  // Category, priority, needs_reply, etc.
    Status   Status            // pending/done/ignored/snoozed
    // ... fewer, clearer fields
}
```

## Success Metrics
- **Relevance accuracy:** % of "relevant" signals user actually acts on
- **Reply usage:** % of drafted replies sent (with/without edits)
- **Time saved:** Reduced time from signal → action
- **User satisfaction:** Simpler, more focused experience