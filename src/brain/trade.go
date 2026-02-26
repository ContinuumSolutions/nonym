package brain

import (
	"fmt"
	"sort"
	"sync"
)

// MarketSignal represents a real-time opportunity from external data feeds.
type MarketSignal struct {
	Source         string  // e.g., "Raydium", "Jupiter", "LinkedInAPI"
	Type           string  // "financial", "social", "arbitrage"
	ExpectedROI    float64 // gross expected return in USD
	TimeCommitment float64 // hours of active oversight required
	ReputationRisk float64 // 0–1 probability of ledger hit
	AutoExecutable bool    // true if fully automatable (no human oversight)
}

// TradeEngine scans live signals and dispatches execution decisions
// across thousands of goroutines simultaneously.
type TradeEngine struct {
	kernel  *EgoKernel
	signals chan MarketSignal
	results chan EvalResult
	wg      sync.WaitGroup
}

// NewTradeEngine creates a TradeEngine with a configurable worker pool.
func NewTradeEngine(ek *EgoKernel, concurrency int) *TradeEngine {
	te := &TradeEngine{
		kernel:  ek,
		signals: make(chan MarketSignal, concurrency*2),
		results: make(chan EvalResult, concurrency*2),
	}
	for i := 0; i < concurrency; i++ {
		te.wg.Add(1)
		go te.worker()
	}
	return te
}

func (te *TradeEngine) worker() {
	defer te.wg.Done()
	for sig := range te.signals {
		op := TradeOpportunity{
			Name:           fmt.Sprintf("%s/%s", sig.Source, sig.Type),
			ExpectedROI:    sig.ExpectedROI,
			TimeCommitment: sig.TimeCommitment,
			ReputationRisk: sig.ReputationRisk,
		}
		result := te.kernel.Decide(op)
		te.results <- result
	}
}

// Submit queues a market signal for evaluation.
func (te *TradeEngine) Submit(sig MarketSignal) {
	te.signals <- sig
}

// Close shuts down the engine and waits for all workers.
func (te *TradeEngine) Close() {
	close(te.signals)
	te.wg.Wait()
	close(te.results)
}

// Results returns the results channel for consuming decisions.
func (te *TradeEngine) Results() <-chan EvalResult {
	return te.results
}

// ScoreAndRank takes a batch of signals and returns them ranked by utility,
// filtered to only auto-executable opportunities above threshold.
func ScoreAndRank(vm *ValueMatrix, signals []MarketSignal) []EvalResult {
	type scored struct {
		result EvalResult
	}
	var mu sync.Mutex
	var scored_ []scored
	var wg sync.WaitGroup

	for _, sig := range signals {
		wg.Add(1)
		go func(s MarketSignal) {
			defer wg.Done()
			op := TradeOpportunity{
				Name:           fmt.Sprintf("%s/%s", s.Source, s.Type),
				ExpectedROI:    s.ExpectedROI,
				TimeCommitment: s.TimeCommitment,
				ReputationRisk: s.ReputationRisk,
			}
			res := vm.Evaluate(op)
			if res.Execute && s.AutoExecutable {
				mu.Lock()
				scored_ = append(scored_, scored{result: res})
				mu.Unlock()
			}
		}(sig)
	}
	wg.Wait()

	// Sort descending by utility.
	sort.Slice(scored_, func(i, j int) bool {
		return scored_[i].result.Utility > scored_[j].result.Utility
	})

	out := make([]EvalResult, len(scored_))
	for i, s := range scored_ {
		out[i] = s.result
	}
	return out
}
