package integrations

import "time"

// ConnectionStatus represents the integration's connection state.
// 0=Pending, 1=InProgress, 2=Installed
type ConnectionStatus int

const (
	Pending    ConnectionStatus = iota // not installed
	InProgress                         // credentials being entered
	Installed                          // connected and active
)

// AuthMethod describes how the integration authenticates.
// 0=APIKeyAuth, 1=OAuth2Auth
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
	Color          string           `json:"color"` // brand hex color, e.g. "#4285F4"; empty for custom services
	Description    string           `json:"description"`
	AuthMethod     AuthMethod       `json:"auth_method" enums:"0,1"`
	Status         ConnectionStatus `json:"status" enums:"0,1,2"`
	Custom         bool             `json:"custom"`
	APIKey         string           `json:"api_key,omitempty"` // masked on read, never raw
	APIEndpoint    string           `json:"api_endpoint,omitempty"`
	OAuthConnected bool             `json:"oauth_connected"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}
