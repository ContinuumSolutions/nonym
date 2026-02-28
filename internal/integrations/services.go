package integrations

import "time"

type ConnectionStatus int

const (
	Pending    ConnectionStatus = iota // not installed
	InProgress                         // credentials being entered
	Installed                          // connected and active
)

type AuthMethod int

const (
	APIKeyAuth AuthMethod = iota
	OAuth2Auth
)

type Service struct {
	ID             int              `json:"id"`
	Slug           string           `json:"slug"`
	Name           string           `json:"name"`
	Category       string           `json:"category"`
	Icon           string           `json:"icon"`
	Description    string           `json:"description"`
	AuthMethod     AuthMethod       `json:"auth_method"`
	Status         ConnectionStatus `json:"status"`
	Custom         bool             `json:"custom"`
	APIKey         string           `json:"api_key,omitempty"` // masked on read, never raw
	APIEndpoint    string           `json:"api_endpoint,omitempty"`
	OAuthConnected bool             `json:"oauth_connected"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}
