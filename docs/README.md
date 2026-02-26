# EK-1 Deployment Documentation

Guides for deploying every module of the Ego-Kernel MVP.

---

## Guides

| # | Guide | What It Covers |
|---|-------|----------------|
| [00](./00-prerequisites.md) | Prerequisites | Go, Rust, Solana CLI, Anchor, Node - all version-pinned |
| [01](./01-deploy-go-brain.md) | Go Brain | Build, test, run, tune, and TEE-deploy the off-chain orchestrator |
| [02](./02-deploy-anchor.md) | Anchor Program | Build, test, and deploy the on-chain Reputation Ledger to devnet/mainnet |
| [03](./03-deploy-harvest.md) | Harvest Scanner | Run locally, wire live APIs, schedule as cron/systemd/Docker |
| [04](./04-quickstart.md) | Quickstart | Zero to running Shadow demo in 5 minutes (Go only, no blockchain) |
| [05](./05-phase-deployment.md) | Phase Deployment | Full 12-week sprint: Shadow → Hand → Voice |

---

## Architecture at a Glance

```
┌─────────────────────────────────────────────────────┐
│                  USER CONSCIOUSNESS                  │
└───────────────────────┬─────────────────────────────┘
                        │  Manual Sync (H2HI only)
┌───────────────────────▼─────────────────────────────┐
│              EGO-KERNEL BRAIN  (Go / TEE)            │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────┐  │
│  │  Value-      │  │   Titan      │  │  Harvest  │  │
│  │  Weighting   │  │  Handshake   │  │  Scanner  │  │
│  │  Matrix      │  │  Protocol    │  │           │  │
│  └──────────────┘  └──────────────┘  └───────────┘  │
└───────────────────────┬─────────────────────────────┘
                        │  Signed transactions
┌───────────────────────▼─────────────────────────────┐
│         SOLANA  (Firedancer / Sonic SVM)              │
│  ┌─────────────────────────────────────────────────┐ │
│  │         ek-logic  (Rust / Anchor 0.31)           │ │
│  │  initialize_kernel  │  log_interaction           │ │
│  │  flag_bad_faith     │  create_escrow             │ │
│  │  settle_escrow                                   │ │
│  └─────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

---

## Start Here

**First time?** → [`04-quickstart.md`](./04-quickstart.md) - running in 5 minutes.

**Ready for blockchain?** → [`00-prerequisites.md`](./00-prerequisites.md) then
[`02-deploy-anchor.md`](./02-deploy-anchor.md).

**Planning the full MVP sprint?** → [`05-phase-deployment.md`](./05-phase-deployment.md).
