# Prerequisites

Install these tools before deploying any EK-1 module.

---

## 1. Go (off-chain brain + harvest scanner)

Required version: **Go 1.22+**

```bash
# Verify
go version

# Install (if missing) — Linux/macOS
wget https://go.dev/dl/go1.22.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.5.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

---

## 2. Rust + Cargo (on-chain Anchor program)

Required version: **Rust 1.79+** (stable)

```bash
# Install via rustup
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
source "$HOME/.cargo/env"

# Add the SBF (Solana Bytecode Format) target
rustup target add bpfel-unknown-none

# Verify
rustc --version
cargo --version
```

---

## 3. Solana CLI

Required version: **Solana CLI 1.18+** (Firedancer-compatible)

```bash
sh -c "$(curl -sSfL https://release.solana.com/v1.18.26/install)"
export PATH="$HOME/.local/share/solana/install/active_release/bin:$PATH"

# Verify
solana --version

# Generate a keypair (skip if you already have one)
solana-keygen new --outfile ~/.config/solana/id.json

# Fund devnet wallet for deployment gas
solana airdrop 2 --url devnet
```

---

## 4. Anchor CLI

Required version: **0.31.0**

```bash
# Install via avm (Anchor Version Manager)
cargo install --git https://github.com/coral-xyz/anchor avm --locked
avm install 0.31.0
avm use 0.31.0

# Verify
anchor --version
```

---

## 5. Node.js + pnpm (Anchor test runner)

Required only for running TypeScript integration tests.

```bash
# Install Node.js 20+
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt-get install -y nodejs

# Install pnpm
npm install -g pnpm

# Verify
node --version
pnpm --version
```

---

## Quick Environment Check

Run this to confirm all tools are ready:

```bash
echo "Go:      $(go version)"
echo "Rust:    $(rustc --version)"
echo "Cargo:   $(cargo --version)"
echo "Solana:  $(solana --version)"
echo "Anchor:  $(anchor --version)"
echo "Node:    $(node --version)"
echo "pnpm:    $(pnpm --version)"
```
