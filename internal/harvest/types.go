package harvest

import "time"

// ContactRecord represents a person in the user's network, aggregated from
// real service signals (email, calendar, Slack).
type ContactRecord struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	FavorsReceived int       `json:"favors_received"` // favours the contact received from the user
	FavorsGiven    int       `json:"favors_given"`    // favours the contact gave to the user
	Overlap        float64   `json:"overlap"`         // max 0-1 solution/bottleneck similarity seen
	LastContact    time.Time `json:"last_contact"`
}

// SocialDebt is an outstanding imbalance owed to the user by a contact.
type SocialDebt struct {
	Contact        ContactRecord `json:"contact"`
	NetFavors      int           `json:"net_favors"`      // positive = contact owes the user
	EstimatedValue float64       `json:"estimated_value"` // USD equivalent
	Action         string        `json:"action"`          // recommended next step
}

// HarvestResult is the output of a full network scan.
type HarvestResult struct {
	ScannedAt     time.Time    `json:"scanned_at"`
	ContactsFound int          `json:"contacts_found"`
	Debts         []SocialDebt `json:"debts"`
	Opportunities []string     `json:"opportunities"`
	TotalValue    float64      `json:"total_value"`
}
