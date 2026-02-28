package ledger

// Ledger is the reputation store interface.
// Phase 1/2: backed by SQLiteLedger (this file's companion).
// Phase 3: replaced by Solana RPC calls to the on-chain Anchor program.
type Ledger interface {
	Initialize(uid string)
	LogSuccess(uid string, impact int64)
	LogBetrayal(uid string, impact int64)
	Score(uid string) int64
	Tier(uid string) ReputationTier
	IsExiled(uid string) bool
	Summary(uid string) string
}
