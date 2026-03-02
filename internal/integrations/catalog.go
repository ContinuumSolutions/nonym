package integrations

// ServiceDef describes a built-in service in the registry.
// To add a new service, append one struct literal to registry below — nothing else needs to change.
type ServiceDef struct {
	Slug        string
	Name        string
	Category    string
	Icon        string
	Color       string // brand hex color, e.g. "#4285F4"
	Description string
	AuthMethod  AuthMethod

	// OAuth2 only (empty/nil for APIKeyAuth services).
	AuthURL     string            // authorization endpoint
	TokenURL    string            // token exchange endpoint
	RevokeURL   string            // token revocation endpoint (optional)
	Scopes      []string          // space-joined into the scope query param
	ExtraParams map[string]string // additional query params appended to the auth URL
}

// Categories available for both built-in and custom services.
const (
	CategoryCalendar      = "Calendar"
	CategoryCommunication = "Communication"
	CategoryFinance       = "Finance"
	CategoryHealth        = "Health"
	CategoryBilling       = "Billing"
)

// registry is the single source of truth for built-in services.
// Seeded into the DB on first run; existing rows are never overwritten.
var registry = []ServiceDef{
	// ── Calendar ────────────────────────────────────────────────────────────
	{
		Slug:        "google-calendar",
		Name:        "Google Calendar",
		Category:    CategoryCalendar,
		Icon:        "google-calendar",
		Color:       "#4285F4",
		Description: "Sync and manage your Google Calendar events.",
		AuthMethod:  OAuth2Auth,
		AuthURL:     "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:    "https://oauth2.googleapis.com/token",
		RevokeURL:   "https://oauth2.googleapis.com/revoke",
		Scopes:      []string{"https://www.googleapis.com/auth/calendar.readonly"},
		ExtraParams: map[string]string{"access_type": "offline", "prompt": "consent"},
	},
	{
		Slug:        "outlook-calendar",
		Name:        "Outlook Calendar",
		Category:    CategoryCalendar,
		Icon:        "outlook",
		Color:       "#0078D4",
		Description: "Sync and manage your Microsoft Outlook Calendar.",
		AuthMethod:  OAuth2Auth,
		AuthURL:     "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
		TokenURL:    "https://login.microsoftonline.com/common/oauth2/v2.0/token",
		Scopes:      []string{"Calendars.Read", "offline_access"},
	},

	// ── Communication ────────────────────────────────────────────────────────
	{
		Slug:        "gmail",
		Name:        "Gmail",
		Category:    CategoryCommunication,
		Icon:        "gmail",
		Color:       "#EA4335",
		Description: "Read and filter your Gmail inbox.",
		AuthMethod:  OAuth2Auth,
		AuthURL:     "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:    "https://oauth2.googleapis.com/token",
		RevokeURL:   "https://oauth2.googleapis.com/revoke",
		Scopes:      []string{"https://www.googleapis.com/auth/gmail.readonly"},
		ExtraParams: map[string]string{"access_type": "offline", "prompt": "consent"},
	},
	{
		Slug:        "outlook-mail",
		Name:        "Outlook Mail",
		Category:    CategoryCommunication,
		Icon:        "outlook",
		Color:       "#0078D4",
		Description: "Read and filter your Outlook inbox.",
		AuthMethod:  OAuth2Auth,
		AuthURL:     "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
		TokenURL:    "https://login.microsoftonline.com/common/oauth2/v2.0/token",
		Scopes:      []string{"Mail.Read", "offline_access"},
	},
	{
		Slug:        "slack",
		Name:        "Slack",
		Category:    CategoryCommunication,
		Icon:        "slack",
		Color:       "#4A154B",
		Description: "Monitor and respond to Slack messages.",
		AuthMethod:  OAuth2Auth,
		AuthURL:     "https://slack.com/oauth/v2/authorize",
		TokenURL:    "https://slack.com/api/oauth.v2.access",
		RevokeURL:   "https://slack.com/api/auth.revoke",
		Scopes:      []string{"channels:read", "channels:history"},
	},
	{
		Slug:        "telegram",
		Name:        "Telegram",
		Category:    CategoryCommunication,
		Icon:        "telegram",
		Color:       "#26A5E4",
		Description: "Send and receive Telegram messages via bot.",
		AuthMethod:  APIKeyAuth,
	},

	// ── Finance ──────────────────────────────────────────────────────────────
	{
		Slug:        "plaid",
		Name:        "Plaid",
		Category:    CategoryFinance,
		Icon:        "plaid",
		Color:       "#01ACAE",
		Description: "Connect your bank accounts via Plaid.",
		AuthMethod:  APIKeyAuth,
	},
	{
		Slug:        "coinbase",
		Name:        "Coinbase",
		Category:    CategoryFinance,
		Icon:        "coinbase",
		Color:       "#0052FF",
		Description: "Monitor and manage your Coinbase portfolio.",
		AuthMethod:  APIKeyAuth,
	},

	// ── Health ───────────────────────────────────────────────────────────────
	{
		Slug:        "google-fit",
		Name:        "Google Fit",
		Category:    CategoryHealth,
		Icon:        "google-fit",
		Color:       "#34A853",
		Description: "Sync health and fitness data from Google Fit.",
		AuthMethod:  OAuth2Auth,
		AuthURL:     "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:    "https://oauth2.googleapis.com/token",
		RevokeURL:   "https://oauth2.googleapis.com/revoke",
		Scopes: []string{
			"https://www.googleapis.com/auth/fitness.activity.read",
			"https://www.googleapis.com/auth/fitness.sleep.read",
		},
		ExtraParams: map[string]string{"access_type": "offline", "prompt": "consent"},
	},
	{
		Slug:        "fitbit",
		Name:        "Fitbit",
		Category:    CategoryHealth,
		Icon:        "fitbit",
		Color:       "#00B0B9",
		Description: "Sync sleep, activity, and health data from Fitbit.",
		AuthMethod:  OAuth2Auth,
		AuthURL:     "https://www.fitbit.com/oauth2/authorize",
		TokenURL:    "https://api.fitbit.com/oauth2/token",
		RevokeURL:   "https://api.fitbit.com/oauth2/revoke",
		Scopes:      []string{"sleep", "activity", "heartrate"},
	},
	{
		Slug:        "oura",
		Name:        "Oura Ring",
		Category:    CategoryHealth,
		Icon:        "oura",
		Color:       "#2C2C2E",
		Description: "Sync sleep and recovery data from your Oura Ring.",
		AuthMethod:  APIKeyAuth,
	},
	{
		Slug:        "whoop",
		Name:        "WHOOP",
		Category:    CategoryHealth,
		Icon:        "whoop",
		Color:       "#3DFF8F",
		Description: "Sync strain, recovery, and sleep data from WHOOP.",
		AuthMethod:  OAuth2Auth,
		AuthURL:     "https://api.prod.whoop.com/oauth/oauth2/auth",
		TokenURL:    "https://api.prod.whoop.com/oauth/oauth2/token",
		Scopes:      []string{"read:recovery", "read:sleep", "read:workout"},
	},

	// ── Billing ──────────────────────────────────────────────────────────────
	{
		Slug:        "stripe",
		Name:        "Stripe",
		Category:    CategoryBilling,
		Icon:        "stripe",
		Color:       "#635BFF",
		Description: "Monitor Stripe payments and subscriptions.",
		AuthMethod:  APIKeyAuth,
	},
}
