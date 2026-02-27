package integrations

import (
	"time"
)

type ConnectionStatus int

const (
	Pending ConnectionStatus = iota
	InProgress
	Installed
)

type Category struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Service struct {
	Name        string           `json:"name"`
	Category    Category         `json:"category"`
	Icon        string           `json:"icon"`
	Description string           `json:"description"`
	Status      ConnectionStatus `json:"status"`
	Custom      bool             `json:"custom"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}
