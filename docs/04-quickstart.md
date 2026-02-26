# Quickstart — Run EK-1 in 5 Minutes

This gets you from zero to a running Shadow-Mode demo using only Go.
No blockchain, no Rust, no external accounts required.

---

## Requirements

- Go 1.22+ (`go version`)
- Git

---

## 1. Clone and Enter the Repo

```bash
git clone https://github.com/egokernel/ek1.git
cd ek1
```

---

## 2. Run the Shadow Demo

```bash
go run ./cmd/ek1/
```

You will see:

```
╔═══════════════════════════════════════════════════════════════╗
║            E G O - K E R N E L   (EK-1)                      ║
║         Sovereign Architecture — Off-Chain Brain              ║
║                    Phase 1: The Shadow                        ║
╚═══════════════════════════════════════════════════════════════╝

[SHADOW MODE] Evaluating today's queue...

  TRIAGE  Raydium/financial/arbitrage   → ACCEPT
  TRIAGE  EmailAPI/meeting/status-update → REJECT  (financial insignificance)
  TRIAGE  VentureCapital/deal           → REJECT  (cognitive tax exceeds ROI)
  TRIAGE  EnergyMarket/financial/futures → ACCEPT
  TRIAGE  LinkedInAPI/social/toxic      → REJECT  (manipulative syntax)

  TOP AUTO-EXECUTABLE OPPORTUNITIES:
    1. EXECUTE [Raydium/financial/arbitrage]: utility=1082.19 ...

[TITAN HANDSHAKE] Initiating P2P negotiation demo...
  HANDSHAKE RESULT: DEADLOCK_RESOLVED | price=$10000.00 | duration=83µs
```

---

## 3. Run the Social Harvest Scanner

```bash
go run ./scripts/harvest/scan.go
```

---

## 4. Run All Tests

```bash
go test ./src/brain/... ./src/ledger/... -v
```

Expected: **14/14 PASS**

---

## 5. Tune Your Values

Open `src/brain/values.go` and edit `DefaultMatrix()`:

```go
func DefaultMatrix() *ValueMatrix {
    return &ValueMatrix{
        TemporalSovereignty: 0.80,  // raise to value time more aggressively
        RiskTolerance:       0.20,  // lower to be more conservative
        ReputationImpact:    0.90,  // keep high for a clean ledger
        BaseHourlyRate:      500.0, // set to your real $/hr
        UtilityThreshold:    1000.0,// raise to filter more aggressively
    }
}
```

Re-run `go run ./cmd/ek1/` to see how the decision log changes.

---

## What's Next

| Step | Guide |
|------|-------|
| Install Rust + Anchor + Solana CLI | [`00-prerequisites.md`](./00-prerequisites.md) |
| Deploy the Reputation Ledger on-chain | [`02-deploy-anchor.md`](./02-deploy-anchor.md) |
| Connect real API data to the Harvest Scanner | [`03-deploy-harvest.md`](./03-deploy-harvest.md) |
| Full Phase-by-Phase deployment | [`05-phase-deployment.md`](./05-phase-deployment.md) |
