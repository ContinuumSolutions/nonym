# Deploying the On-Chain Reputation Ledger (Rust / Anchor)

The `ek-logic` Anchor program is the immutable "Court" of the Sovereign Protocol.
It stores every Kernel's reputation score, logs interactions, flags bad-faith actors,
and manages micro-escrow settlements — all on Solana.

---

## Program: `ek-logic`

| File | Description |
|------|-------------|
| `programs/ek-logic/src/lib.rs` | All on-chain logic |
| `programs/ek-logic/Cargo.toml` | Anchor 0.31.0 crate config |
| `Cargo.toml` | Workspace manifest |
| `Anchor.toml` | Cluster + wallet config |

### Instructions

| Instruction | Description |
|-------------|-------------|
| `initialize_kernel` | Mint a new KernelProfile (Soulbound identity NFT) |
| `log_interaction` | Record success or betrayal; adjust reputation score |
| `flag_bad_faith` | Attach a signed Dishonesty Hash; penalize target's score |
| `create_escrow` | Open a micro-escrow; lock lamports in a PDA |
| `settle_escrow` | Release to counterparty (success) or refund (betrayal) |

---

## Step 1 — Install Prerequisites

See [`00-prerequisites.md`](./00-prerequisites.md) for Rust, Solana CLI,
and Anchor CLI installation.

---

## Step 2 — Configure Your Wallet

```bash
# Generate a new keypair (skip if you already have one)
solana-keygen new --outfile ~/.config/solana/id.json

# Set the CLI to devnet
solana config set --url devnet

# Confirm your address
solana address

# Fund with devnet SOL (free airdrop)
solana airdrop 2
solana balance
```

---

## Step 3 — Generate a Program Address

```bash
# Create a fresh keypair for the program
solana-keygen new --outfile target/deploy/ek_logic-keypair.json --no-bip39-passphrase

# Extract the public key — this is your program ID
solana-keygen pubkey target/deploy/ek_logic-keypair.json
```

Update `Anchor.toml` and `programs/ek-logic/src/lib.rs` with this address:

```toml
# Anchor.toml
[programs.devnet]
ek_logic = "<YOUR_PROGRAM_ID>"
```

```rust
// programs/ek-logic/src/lib.rs  (line 7)
declare_id!("<YOUR_PROGRAM_ID>");
```

---

## Step 4 — Build

```bash
# Build the program (compiles to SBF bytecode)
anchor build

# Confirm the .so artifact exists
ls -lh target/deploy/ek_logic.so

# Review the generated IDL (used by the Go client in Phase 2)
cat target/idl/ek_logic.json
```

---

## Step 5 — Run On-Chain Tests (Localnet)

Spin up a local validator and run tests before going to devnet.

```bash
# Start the local test validator in the background
solana-test-validator &

# Run Anchor tests against localnet
anchor test --provider.cluster localnet

# Stop the validator when done
pkill solana-test-validator
```

Expected: all instructions initialize, log, flag, and settle correctly.

---

## Step 6 — Deploy to Devnet

```bash
# Deploy to Solana Devnet (costs ~2 SOL in rent)
anchor deploy --provider.cluster devnet

# Confirm deployment
solana program show <YOUR_PROGRAM_ID> --url devnet
```

You should see:

```
Program Id:   <YOUR_PROGRAM_ID>
Owner:        BPFLoaderUpgradeab1e11111111111111111111111
Data Length:  <bytes>
Authority:    <YOUR_WALLET>
Deployed In Slot: <slot>
```

---

## Step 7 — Initialize Your First Kernel

Use the Solana CLI to send a raw transaction, or use the Anchor TypeScript
client generated from the IDL.

### Option A — TypeScript (Anchor client)

```bash
pnpm install
pnpm add @coral-xyz/anchor @solana/web3.js
```

```typescript
// scripts/init-kernel.ts
import * as anchor from "@coral-xyz/anchor";
import { Program } from "@coral-xyz/anchor";
import { EkLogic } from "../target/types/ek_logic";

const provider = anchor.AnchorProvider.env();
anchor.setProvider(provider);

const program = anchor.workspace.EkLogic as Program<EkLogic>;

const [kernelPDA] = anchor.web3.PublicKey.findProgramAddressSync(
  [Buffer.from("kernel"), provider.wallet.publicKey.toBuffer()],
  program.programId
);

await program.methods
  .initializeKernel("Sovereign_001_Alpha")
  .accounts({
    kernelProfile: kernelPDA,
    user: provider.wallet.publicKey,
    systemProgram: anchor.web3.SystemProgram.programId,
  })
  .rpc();

console.log("Kernel initialized:", kernelPDA.toBase58());
```

```bash
anchor run init-kernel --provider.cluster devnet
```

### Option B — Solana CLI (raw)

```bash
# After building, you can call instructions directly with solana program invoke
# (complex; use Option A for readability)
```

---

## Step 8 — Verify On-Chain

```bash
# Check your program is live
solana program show <YOUR_PROGRAM_ID> --url devnet

# Watch live logs from the program
solana logs <YOUR_PROGRAM_ID> --url devnet

# Verify the deployed binary matches your local build
solana-verify verify-program <YOUR_PROGRAM_ID> \
  --program-lib-name ek_logic \
  --url devnet
```

---

## Step 9 — Deploy to Mainnet

Only proceed after devnet validation is complete.

```bash
# Switch wallet to mainnet-funded keypair
solana config set --url mainnet-beta

# Fund wallet (real SOL required — ~2–4 SOL for deployment)
# Transfer SOL from exchange or bridge

# Deploy
anchor deploy --provider.cluster mainnet

# Verify
solana-verify verify-program <YOUR_PROGRAM_ID> \
  --program-lib-name ek_logic \
  --url mainnet-beta
```

---

## Upgrade an Existing Deployment

The program is deployed as upgradeable by default (BPFLoaderUpgradeable).

```bash
# After modifying lib.rs, rebuild and upgrade
anchor build
anchor upgrade target/deploy/ek_logic.so \
  --program-id <YOUR_PROGRAM_ID> \
  --provider.cluster devnet
```

To make the program **immutable** (production hardening):

```bash
solana program set-upgrade-authority <YOUR_PROGRAM_ID> \
  --final \
  --url devnet
```

---

## Reputation Score: Account Layout

```
KernelProfile
├── authority        [Pubkey  32B]  — wallet that controls this kernel
├── uid              [String  68B]  — human-readable identity string
├── reputation_score [u64      8B]  — current score (baseline: 1,000)
├── total_interactions [u64    8B]
├── total_successes  [u64      8B]
├── total_betrayals  [u64      8B]
├── bad_faith_flags  [u8       1B]  — count of Dishonesty Hashes received
├── is_exiled        [bool     1B]  — true when score < 100
└── initialized_at   [i64      8B]  — Unix timestamp
```

Total account size: **~140 bytes** (~0.0014 SOL rent-exempt).

---

## Costs (Devnet / Mainnet)

| Action | Approx. Cost |
|--------|-------------|
| Deploy program | ~2–4 SOL |
| `initialize_kernel` | ~0.0014 SOL (rent-exempt account) |
| `log_interaction` | ~0.000005 SOL (transaction fee) |
| `create_escrow` | ~0.003 SOL (account rent) + escrowed amount |
| `settle_escrow` | ~0.000005 SOL |
| `flag_bad_faith` | ~0.000005 SOL |
