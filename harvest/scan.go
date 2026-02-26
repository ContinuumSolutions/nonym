// Package harvest implements the Social Leverage Scanner.
// It identifies "Hidden Value" in professional and social networks:
// - Unreciprocated favors (Social Debt)
// - Bottleneck matches (your solution ↔ their problem)
// - Passive revenue opportunities
//
// In Phase 1 (Shadow), this runs in read-only mode and surfaces
// opportunities as recommendations. In Phase 2 (Hand), it initiates
// actual Value-Rebalance requests and Ghost-Agreements on-chain.
package main

import (
	"fmt"
	"sort"
	"time"
)

// ContactRecord represents a person in the user's network.
type ContactRecord struct {
	ID              string
	Name            string
	FavorsReceived  int     // favors they received from the user
	FavorsGiven     int     // favors they gave back
	Overlap         float64 // 0–1 similarity between their bottleneck and user's solutions
	ReputationScore int64   // their EK-1 reputation (if known)
	LastContact     time.Time
}

// SocialDebt is an outstanding imbalance owed to the user.
type SocialDebt struct {
	Contact       ContactRecord
	NetFavors     int     // favorsReceived − favorsGiven (positive = they owe you)
	EstimatedValue float64 // USD equivalent of the owed debt
	Action        string  // recommended action
}

// HarvestResult is the output of a full network scan.
type HarvestResult struct {
	Debts        []SocialDebt
	Opportunities []string
	TotalValue   float64
	ScannedAt    time.Time
}

// Scanner performs the social leverage scan.
type Scanner struct {
	contacts []ContactRecord
	// In production: connect to LinkedIn API, email graph, GitHub activity.
}

// NewScanner initializes the scanner with a contact list.
func NewScanner(contacts []ContactRecord) *Scanner {
	return &Scanner{contacts: contacts}
}

// Scan runs a full social graph analysis and returns the harvest result.
func (s *Scanner) Scan() HarvestResult {
	result := HarvestResult{ScannedAt: time.Now()}

	for _, c := range s.contacts {
		net := c.FavorsReceived - c.FavorsGiven

		if net > 0 {
			// They owe us. Calculate the estimated value.
			value := float64(net) * 3750.0 // $3,750 per unreciprocated favor (industry avg)
			action := "Send Value-Rebalance request: require Tier-1 introduction by EOD."
			if net >= 5 {
				action = "Issue Blind Favor Token request: provide solution in exchange for open-ended future favor."
			}
			result.Debts = append(result.Debts, SocialDebt{
				Contact:        c,
				NetFavors:      net,
				EstimatedValue: value,
				Action:         action,
			})
			result.TotalValue += value
		}

		// Check for bottleneck/solution overlap.
		if c.Overlap >= 0.95 {
			result.Opportunities = append(result.Opportunities, fmt.Sprintf(
				"[GHOST-AGREEMENT] %s has a %.0f%% overlap with your current solution set. "+
					"Initiate Ghost-Agreement: provide solution, receive Blind Favor Token.",
				c.Name, c.Overlap*100,
			))
		}
	}

	// Sort debts by estimated value (highest first).
	sort.Slice(result.Debts, func(i, j int) bool {
		return result.Debts[i].EstimatedValue > result.Debts[j].EstimatedValue
	})

	return result
}

// Print displays the harvest results to stdout.
func (r *HarvestResult) Print() {
	fmt.Printf("\n╔══════════════════════════════════════════════════╗\n")
	fmt.Printf("║           SOCIAL HARVEST RESULTS                 ║\n")
	fmt.Printf("║           Scanned: %-29s ║\n", r.ScannedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("╚══════════════════════════════════════════════════╝\n\n")

	if len(r.Debts) == 0 {
		fmt.Println("  No outstanding social debts detected.")
	} else {
		fmt.Println("  SOCIAL DEBTS (outstanding favors owed to you):")
		for _, d := range r.Debts {
			fmt.Printf("    %-20s | Net Favors: %+d | Est. Value: $%.0f\n",
				d.Contact.Name, d.NetFavors, d.EstimatedValue)
			fmt.Printf("    → %s\n\n", d.Action)
		}
	}

	if len(r.Opportunities) > 0 {
		fmt.Println("  GHOST-AGREEMENT OPPORTUNITIES:")
		for _, op := range r.Opportunities {
			fmt.Printf("    %s\n", op)
		}
	}

	fmt.Printf("\n  TOTAL ESTIMATED NETWORK VALUE: $%.0f\n", r.TotalValue)
}

func main() {
	// Demo contact list — in production this is pulled from API integrations.
	contacts := []ContactRecord{
		{
			ID: "c001", Name: "Alice Ventures",
			FavorsReceived: 12, FavorsGiven: 0,
			Overlap: 0.20, ReputationScore: 820,
			LastContact: time.Now().AddDate(0, -2, 0),
		},
		{
			ID: "c002", Name: "Bob Silent Broker",
			FavorsReceived: 3, FavorsGiven: 1,
			Overlap: 0.98, ReputationScore: 950,
			LastContact: time.Now().AddDate(0, 0, -5),
		},
		{
			ID: "c003", Name: "Carol Peer",
			FavorsReceived: 2, FavorsGiven: 2,
			Overlap: 0.45, ReputationScore: 780,
			LastContact: time.Now().AddDate(0, -1, 0),
		},
		{
			ID: "c004", Name: "Dave Time-Waster",
			FavorsReceived: 7, FavorsGiven: 0,
			Overlap: 0.05, ReputationScore: 200,
			LastContact: time.Now().AddDate(-1, 0, 0),
		},
	}

	scanner := NewScanner(contacts)
	result := scanner.Scan()
	result.Print()
}
