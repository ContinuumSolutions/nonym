package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/egokernel/ek1/src/brain"
	"github.com/egokernel/ek1/src/ledger"
	"github.com/egokernel/ek1/src/protocols"
)

func main() {
	fmt.Println(`
╔═══════════════════════════════════════════════════════════════╗
║            E G O - K E R N E L   (EK-1)                      ║
║         Sovereign Architecture — Off-Chain Brain              ║
║                    Phase 1: The Shadow                        ║
╚═══════════════════════════════════════════════════════════════╝
`)

	uid := os.Getenv("EK_UID")
	if uid == "" {
		uid = "Sovereign_001_Alpha"
	}

	// --- Initialize the Kernel ---
	values := brain.DefaultMatrix()
	ek := brain.NewKernel(uid, values)

	// --- Reputation Ledger (Phase 2: connects to Solana Devnet) ---
	repClient := ledger.NewLocalLedger()
	repClient.Initialize(uid)

	// --- Trade Engine (concurrent signal processing) ---
	engine := brain.NewTradeEngine(ek, 100)
	go consumeResults(engine)

	// --- Simulate Phase 1: "The Shadow" Decision Mirror ---
	fmt.Println("\n[SHADOW MODE] Evaluating today's queue against Value-Weighting Matrix...\n")
	runShadowDemo(uid, ek, engine, repClient)

	// --- Titan Handshake Demo ---
	fmt.Println("\n[TITAN HANDSHAKE] Initiating P2P negotiation demo...\n")
	runHandshakeDemo(uid, ek, repClient)

	// --- Daily Summary ---
	ek.DailySummary()

	// Wait for interrupt.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	engine.Close()
	fmt.Println("\nEK-1 offline. Sovereignty preserved.")
}

func runShadowDemo(uid string, ek *brain.EgoKernel, engine *brain.TradeEngine, rep *ledger.LocalLedger) {
	// Simulate the "Day in the Life" queue.
	signals := []brain.MarketSignal{
		{
			Source: "Raydium", Type: "financial/arbitrage",
			ExpectedROI: 1200, TimeCommitment: 0.01, ReputationRisk: 0.01,
			AutoExecutable: true,
		},
		{
			Source: "EmailAPI", Type: "meeting/status-update",
			ExpectedROI: 50, TimeCommitment: 1.0, ReputationRisk: 0.0,
			AutoExecutable: false,
		},
		{
			Source: "VentureCapital", Type: "deal/prestige-project",
			ExpectedROI: 5000, TimeCommitment: 10.0, ReputationRisk: 0.3,
			AutoExecutable: false,
		},
		{
			Source: "EnergyMarket", Type: "financial/futures",
			ExpectedROI: 420, TimeCommitment: 0.001, ReputationRisk: 0.02,
			AutoExecutable: true,
		},
		{
			Source: "LinkedInAPI", Type: "social/toxic-connection",
			ExpectedROI: 0, TimeCommitment: 0.5, ReputationRisk: 0.8,
			AutoExecutable: false,
		},
	}

	// Evaluate each via triage + trade engine.
	for _, sig := range signals {
		req := brain.IncomingRequest{
			ID:             sig.Source + "/" + sig.Type,
			SenderID:       sig.Source,
			Description:    sig.Type,
			EstimatedROI:   sig.ExpectedROI,
			TimeCommitment: sig.TimeCommitment,
			ManipulationPct: func() float64 {
				if sig.ReputationRisk > 0.5 {
					return 0.20 // simulate manipulative source
				}
				return 0.02
			}(),
		}
		action, reason := ek.Triage(req)
		fmt.Printf("  TRIAGE  %-40s → %-6s | %s\n", req.ID, action, reason)

		if action == "ACCEPT" {
			engine.Submit(sig)
		}
	}

	// Ranked auto-executable opportunities.
	ranked := brain.ScoreAndRank(ek.Values, signals)
	fmt.Printf("\n  TOP AUTO-EXECUTABLE OPPORTUNITIES:\n")
	for i, r := range ranked {
		fmt.Printf("    %d. %s\n", i+1, r.Reason)
	}

	// Log a simulated reputation event.
	rep.LogSuccess(uid, 50)
	fmt.Printf("\n  Reputation after shadow run: %d\n", rep.Score(uid))
}

func runHandshakeDemo(uid string, ek *brain.EgoKernel, rep *ledger.LocalLedger) {
	rivalID := "RivalAE_VC_772"
	rep.Initialize(rivalID)
	rep.LogBetrayal(rivalID, 3) // Rival has 3 ghosted contracts

	hs := protocols.NewHandshake(ek.UID, rivalID, rep)
	result := hs.Execute(protocols.HandshakeParams{
		ResourceName:   "GPU Cluster / High-Priority Access",
		UserDesire:     0.85,
		RivalDesire:    0.70,
		UserRepScore:   float64(rep.Score(ek.UID)),
		RivalRepScore:  float64(rep.Score(rivalID)),
		MarketRate:     10000.0,
	})

	fmt.Printf("  HANDSHAKE RESULT: %s\n", result.Outcome)
	fmt.Printf("  Final Price: $%.2f\n", result.FinalPrice)
	fmt.Printf("  Duration: %s\n", result.Duration)
	fmt.Printf("  Log:\n")
	for _, line := range result.Log {
		fmt.Printf("    %s\n", line)
	}
}

func consumeResults(engine *brain.TradeEngine) {
	for range engine.Results() {
		// Results already logged by the kernel's emit() calls.
		// In Phase 2, this goroutine submits EXECUTE decisions to the Solana RPC.
	}
}
