# Phase-by-Phase Deployment Guide

Full 12-week sprint from local Shadow Mode to live multi-agent Titan Handshakes
on Solana Mainnet.

---

## Phase 1 — "The Shadow" (Weeks 1–4)

**Goal:** Read-only decision mirror. No execution. No blockchain.

### What gets deployed

| Component | Where | Status |
|-----------|-------|--------|
| Go brain (brain, ledger, protocols) | Local machine / VM | Ready |
| Social Harvest Scanner | Local machine / cron | Ready |
| Local Reputation Ledger | In-memory (LocalLedger) | Ready |

### Steps

```bash
# 1. Confirm Go 1.22+ is installed
go version

# 2. Run all tests
go test ./src/brain/... ./src/ledger/... -v

# 3. Start the Shadow daemon
EK_UID="Sovereign_001" go run ./cmd/ek1/

# 4. Run the harvest scanner
go run ./scripts/harvest/scan.go

# 5. Schedule harvest as a daily cron
crontab -e
# Add: 0 7 * * * cd /path/to/egokernel && ./harvest >> ~/ek1-harvest.log 2>&1
```

### Demo checkpoint

- Shadow log shows ACCEPT/REJECT/GHOST decisions for all incoming signals.
- Titan Handshake resolves in < 1ms between two demo kernels.
- Social scanner identifies outstanding debts and opportunities.

---

## Phase 2 — "The Hand" (Weeks 5–8)

**Goal:** Give the Kernel a wallet. Automate micro-escrow settlements ($5 disputes).

### What gets deployed

| Component | Where | Status |
|-----------|-------|--------|
| `ek-logic` Anchor program | Solana Devnet | Deploy now |
| Go brain wired to Solana RPC | Local / VM | Wire after deploy |
| TypeScript Anchor client | Local scripts | Write after deploy |

### Steps

```bash
# 1. Install prerequisites (Rust, Solana CLI, Anchor)
#    See docs/00-prerequisites.md

# 2. Generate program keypair
solana-keygen new --outfile target/deploy/ek_logic-keypair.json --no-bip39-passphrase
export PROGRAM_ID=$(solana-keygen pubkey target/deploy/ek_logic-keypair.json)
echo "Program ID: $PROGRAM_ID"

# 3. Update declare_id! in programs/ek-logic/src/lib.rs
#    and [programs.devnet] in Anchor.toml with $PROGRAM_ID

# 4. Build the on-chain program
anchor build

# 5. Fund your wallet
solana config set --url devnet
solana airdrop 4   # need ~2 SOL for deployment rent

# 6. Deploy to devnet
anchor deploy --provider.cluster devnet

# 7. Verify deployment
solana program show $PROGRAM_ID --url devnet

# 8. Initialize your first Kernel on-chain
#    (use the TypeScript snippet in docs/02-deploy-anchor.md Step 7)

# 9. Wire the Go brain to the on-chain ledger
#    Edit cmd/ek1/main.go:
#    repClient := ledger.NewSolanaLedger(rpc.DevNet_RPC, "~/.config/solana/id.json", programID)

# 10. Test a micro-escrow settlement ($5)
#     Use the create_escrow + settle_escrow instructions via the Anchor client
```

### Demo checkpoint

- On-chain `KernelProfile` visible on Solana Explorer (devnet).
- A micro-escrow is created, settled, and the reputation score updated — zero
  human intervention, confirmed on-chain in < 1 second.

---

## Phase 3 — "The Voice" (Weeks 9–12)

**Goal:** Live multi-agent Titan Handshake demo. Two Kernels resolve a conflict
in < 50ms; both Reputation Ledgers update atomically on-chain.

### What gets deployed

| Component | Where |
|-----------|-------|
| `ek-logic` program | Solana Mainnet |
| Go brain (100 goroutines) | Cloud VM (or TEE) |
| 100 Bot Kernels | Cloud (automated) |
| Live visualization | Web dashboard |

### Steps

```bash
# 1. Harden the devnet deployment first (run 2 weeks of real transactions)

# 2. Deploy to mainnet
solana config set --url mainnet-beta
anchor deploy --provider.cluster mainnet

# 3. Make the program immutable (optional — removes upgrade authority)
solana program set-upgrade-authority $PROGRAM_ID --final --url mainnet-beta

# 4. Spin up 100 Bot Kernels (each with a funded devnet/mainnet wallet)
for i in $(seq 1 100); do
  solana-keygen new --outfile "bots/bot-$i.json" --no-bip39-passphrase --silent
  solana airdrop 0.1 $(solana-keygen pubkey "bots/bot-$i.json") --url devnet
done

# 5. Initialize Bot Kernel profiles on-chain
#    (loop the TypeScript init script for each bot wallet)

# 6. Start the bot simulation (goroutine pool)
#    go run ./cmd/ek1/ --bots=100 --resource="GPU_Cluster_Alpha"

# 7. Watch live negotiations
solana logs $PROGRAM_ID --url devnet
```

### Demo checkpoint (the live demo)

Show the audience:
1. Two Kernels (User + Rival) both want the same GPU cluster slot.
2. The Titan Handshake log runs through all 3 tiers in < 50ms.
3. The Solana Explorer shows both `KernelProfile` accounts updated in the
   same transaction block.
4. One Kernel is flagged with a `BadFaithFlagged` event visible on-chain.

---

## Environment Summary

| Phase | Blockchain | Go Brain | Ledger |
|-------|-----------|----------|--------|
| 1 — Shadow | None | Local | In-memory |
| 2 — Hand | Solana Devnet | Local / VM | On-chain (devnet) |
| 3 — Voice | Solana Mainnet | Cloud VM / TEE | On-chain (mainnet) |

---

## Rollback Procedure

If an on-chain deployment has a critical bug before making it immutable:

```bash
# Upgrade to a patched build
anchor build
anchor upgrade target/deploy/ek_logic.so \
  --program-id $PROGRAM_ID \
  --provider.cluster devnet

# Emergency: close the program and recover rent
solana program close $PROGRAM_ID --url devnet
```

If already immutable, the only path is deploying a **new program ID** and
migrating reputation data via a migration instruction.

---

## Security Checklist Before Mainnet

- [ ] Anchor program audited by a third party (e.g., OtterSec, Neodyme)
- [ ] All `has_one` and `seeds` constraints verified in tests
- [ ] `settle_escrow` cannot be called by anyone except the `authority`
- [ ] `flag_bad_faith` requires the flagger to have a valid (non-exiled) kernel
- [ ] Identity Entropy threshold tuned and validated in simulation
- [ ] TEE attestation report generated for the Go brain enclave
- [ ] Upgrade authority transferred to a multisig (e.g., Squads Protocol)
      before making any user-facing announcement
