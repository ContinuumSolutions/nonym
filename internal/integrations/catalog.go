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
	},
	{
		Slug:        "outlook-calendar",
		Name:        "Outlook Calendar",
		Category:    CategoryCalendar,
		Icon:        "outlook",
		Color:       "#0078D4",
		Description: "Sync and manage your Microsoft Outlook Calendar.",
		AuthMethod:  OAuth2Auth,
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
	},
	{
		Slug:        "outlook-mail",
		Name:        "Outlook Mail",
		Category:    CategoryCommunication,
		Icon:        "outlook",
		Color:       "#0078D4",
		Description: "Read and filter your Outlook inbox.",
		AuthMethod:  OAuth2Auth,
	},
	{
		Slug:        "slack",
		Name:        "Slack",
		Category:    CategoryCommunication,
		Icon:        "slack",
		Color:       "#4A154B",
		Description: "Monitor and respond to Slack messages.",
		AuthMethod:  OAuth2Auth,
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
	},
	{
		Slug:        "fitbit",
		Name:        "Fitbit",
		Category:    CategoryHealth,
		Icon:        "fitbit",
		Color:       "#00B0B9",
		Description: "Sync sleep, activity, and health data from Fitbit.",
		AuthMethod:  OAuth2Auth,
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
