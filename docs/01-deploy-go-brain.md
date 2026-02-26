# Deploying the Go Brain (Off-Chain Orchestrator)

The Go brain is the TEE-resident intelligence layer of EK-1. It runs the
Value-Weighting Matrix, evaluates every incoming signal, and drives the
Titan Handshake protocol. It requires **no blockchain connection** for
Phase 1 (Shadow Mode).

---

## Packages

| Package | Path | Role |
|---------|------|------|
| `brain` | `src/brain/` | Kernel lifecycle, Triage, Decide, Soul-Drift Guard |
| `protocols` | `src/protocols/` | Titan Handshake (P2P negotiation) |
| `ledger` | `src/ledger/` | Reputation scoring (local in Phase 1) |
| `ek1` (main) | `cmd/ek1/` | Entry point - runs Shadow demo + Handshake demo |
| `harvest` | `scripts/harvest/` | Social leverage scanner |

---

## 1. Build

```bash
# Build all packages
go build ./...

# Build just the main binary
go build -o ek1 ./cmd/ek1/

# Build the harvest scanner
go build -o harvest ./scripts/harvest/
```

---

## 2. Run Tests

```bash
# Run all tests with verbose output
go test ./src/brain/... ./src/ledger/... -v

# Run with race detector (recommended before production)
go test -race ./src/brain/... ./src/ledger/...

# Run a specific test
go test ./src/brain/ -run TestEvaluate_AcceptsAutoArbitrage -v
```

Expected output: **14/14 tests pass.**

---

## 3. Run Phase 1 - Shadow Mode (Local)

Shadow Mode runs entirely in-process with no external dependencies.
It mirrors what EK-1 *would* decide, without executing anything.

```bash
# Using go run
go run ./cmd/ek1/

# Using the compiled binary
./ek1

# Set a custom Kernel UID via env var
EK_UID="Sovereign_YourName_001" go run ./cmd/ek1/
```

**What you will see:**

- Triage log: each incoming signal rated ACCEPT / REJECT / GHOST
- Top auto-executable opportunities ranked by utility
- Titan Handshake negotiation running in < 1ms
- Daily summary with status, decision count, and recent log

---

## 4. Run the Social Harvest Scanner

```bash
go run ./scripts/harvest/scan.go
```

This scans the demo contact list and prints:
- Outstanding social debts (unreciprocated favors) with estimated USD value
- Ghost-Agreement opportunities (high-overlap contacts)
- Total estimated network value

To plug in real data, edit the `contacts` slice in `scripts/harvest/scan.go`
or replace it with API calls to LinkedIn / GitHub / email graph.

---

## 5. Tuning the Value-Weighting Matrix

The matrix is in `src/brain/values.go`. The defaults are:

```go
TemporalSovereignty: 0.80,  // how much you value free time (0–1)
RiskTolerance:       0.20,  // max acceptable reputation risk per trade
ReputationImpact:    0.90,  // how much you care about ledger cleanliness
SocialEntropy:       0.10,  // human-friction injection rate
BaseHourlyRate:      500.0, // your baseline $/hr for cognitive tax
UtilityThreshold:    1000.0,// minimum utility to execute any action
PresentBiasDiscount: 0.05,  // ρ: discount rate for future rewards
```

Override at runtime by constructing a custom `brain.ValueMatrix` in
`cmd/ek1/main.go` before calling `brain.NewKernel()`.

---

## 6. Phase 2 - Connecting to Solana Devnet

Once the on-chain Reputation Ledger is deployed (see
[`02-deploy-anchor.md`](./02-deploy-anchor.md)), replace the `LocalLedger`
in `cmd/ek1/main.go` with the Solana RPC client:

```go
// Phase 2: swap this line
repClient := ledger.NewLocalLedger()

// With a Solana-backed ledger (after on-chain deploy)
repClient := ledger.NewSolanaLedger(
    rpc.DevNet_RPC,
    "~/.config/solana/id.json",
    programID,
)
```

The `SolanaLedger` implementation lives in `src/ledger/solana.go`
(stub ready for Phase 2 wiring).

---

## 7. Production Deployment (TEE)

For Phase 3, the Go brain runs inside a Trusted Execution Environment
to keep the Value-Weighting heuristics private.

### Intel SGX (via Gramine)

```bash
# Install Gramine
sudo apt install gramine

# Build a Gramine manifest for ek1
cat > ek1.manifest.template << 'EOF'
loader.entrypoint = "file:{{ gramine.libos }}"
libos.entrypoint = "/ek1"
loader.argv = ["/ek1"]
loader.env.EK_UID = "Sovereign_001"
sgx.enclave_size = "256M"
sgx.max_threads = 16
EOF

gramine-manifest ek1.manifest.template ek1.manifest
gramine-sgx-sign --manifest ek1.manifest --output ek1.manifest.sgx

# Run inside the enclave
gramine-sgx ./ek1
```

### NVIDIA H100 Confidential Computing

```bash
# Build with CC support
go build -tags cc -o ek1-cc ./cmd/ek1/
# Deploy via NVIDIA Confidential Computing SDK (see NVIDIA docs)
```

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `EK_UID` | `Sovereign_001_Alpha` | Kernel identity string |
| `EK_RPC_URL` | *(unset)* | Solana RPC endpoint (Phase 2+) |
| `EK_WALLET` | `~/.config/solana/id.json` | Path to Solana keypair |
| `EK_PROGRAM_ID` | *(unset)* | Deployed on-chain program address |
