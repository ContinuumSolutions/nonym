package activities

import (
	"time"
)

// EventType classifies the category of an event.
// 0=Finance, 1=Calendar, 2=Communication, 3=Billing, 4=Health, 5=Other
type EventType int

const (
	Finance EventType = iota
	Calendar
	Communication
	Billing
	Health
	Other
)

// Decision is the outcome the kernel reached for an event.
// 0=Pending, 1=Accepted, 2=Declined, 3=Negotiated, 4=Automated, 5=Cancelled
type Decision int

const (
	Pending Decision = iota
	Accepted
	Declined
	Negotiated
	Automated
	Cancelled
)

// Importance rates how significant an event is.
// 0=Low, 1=Medium, 2=High
type Importance int

const (
	Low Importance = iota
	Medium
	High
)

// GainType describes whether the outcome is beneficial or costly.
// 0=Positive, 1=Negative
type GainType int

const (
	Positive GainType = iota
	Negative
)

// GainKind distinguishes what the gain represents.
// 0=Money, 1=Time
type GainKind int

const (
	Money GainKind = iota
	Time
)

type Gain struct {
	Type    GainType `json:"type" enums:"0,1"`
	Kind    GainKind `json:"kind" enums:"0,1"`
	Value   float32  `json:"_value"`
	Symbol  string   `json:"_symbol"` // How to present the gain e.g $ for money type
	Details string   `json:"details"`
}

type Event struct {
	ID            int        `json:"id"`
	EventType     EventType  `json:"event_type" enums:"0,1,2,3,4,5"`
	Decision      Decision   `json:"decision" enums:"0,1,2,3,4,5"`
	Importance    Importance `json:"importance" enums:"0,1,2"`
	Narrative     string     `json:"narrative"` // Detail description of exactly what happened
	Gain          Gain       `json:"gain"`
	SourceService string     `json:"source_service"`
	Read          bool       `json:"read"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
