package activities

import (
	"time"
)

type EventType int

const (
	Finance EventType = iota
	Calendar
	Communication
	Billing
	Health
)

type Decision int

const (
	Pending Decision = iota
	Accepted
	Declined
	Negotiated
	Automated
	Cancelled
)

type Importance int

const (
	Low Importance = iota
	Medium
	High
)

type GainType int

const (
	Positive GainType = iota
	Negative
)

type Gain struct {
	Type    GainType `json:"type"`
	Value   float32  `json:"_value"`
	Symbol  string   `json:"_symbol"` // How to present the gain e.g $ for money type
	Details string   `json:"details"`
}

type Event struct {
	ID         int        `json:"id"`
	EventType  EventType  `json:"event_type"`
	Decision   Decision   `json:"decision"`
	Importance Importance `json:"importance"`
	Narrative  string     `json:"narrative"` // Detail description of exactly what happened
	Gain       Gain       `json:"gain"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
