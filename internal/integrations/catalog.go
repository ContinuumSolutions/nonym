package integrations

// ServiceDef describes a built-in service in the registry.
// To add a new service, append one struct literal to registry below — nothing else needs to change.
type ServiceDef struct {
	Slug        string
	Name        string
	Category    string
	Icon        string
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
		Description: "Sync and manage your Google Calendar events.",
		AuthMethod:  OAuth2Auth,
	},
	{
		Slug:        "outlook-calendar",
		Name:        "Outlook Calendar",
		Category:    CategoryCalendar,
		Icon:        "outlook",
		Description: "Sync and manage your Microsoft Outlook Calendar.",
		AuthMethod:  OAuth2Auth,
	},

	// ── Communication ────────────────────────────────────────────────────────
	{
		Slug:        "gmail",
		Name:        "Gmail",
		Category:    CategoryCommunication,
		Icon:        "gmail",
		Description: "Read and filter your Gmail inbox.",
		AuthMethod:  OAuth2Auth,
	},
	{
		Slug:        "outlook-mail",
		Name:        "Outlook Mail",
		Category:    CategoryCommunication,
		Icon:        "outlook",
		Description: "Read and filter your Outlook inbox.",
		AuthMethod:  OAuth2Auth,
	},
	{
		Slug:        "slack",
		Name:        "Slack",
		Category:    CategoryCommunication,
		Icon:        "slack",
		Description: "Monitor and respond to Slack messages.",
		AuthMethod:  OAuth2Auth,
	},
	{
		Slug:        "telegram",
		Name:        "Telegram",
		Category:    CategoryCommunication,
		Icon:        "telegram",
		Description: "Send and receive Telegram messages via bot.",
		AuthMethod:  APIKeyAuth,
	},

	// ── Finance ──────────────────────────────────────────────────────────────
	{
		Slug:        "plaid",
		Name:        "Plaid",
		Category:    CategoryFinance,
		Icon:        "plaid",
		Description: "Connect your bank accounts via Plaid.",
		AuthMethod:  APIKeyAuth,
	},
	{
		Slug:        "coinbase",
		Name:        "Coinbase",
		Category:    CategoryFinance,
		Icon:        "coinbase",
		Description: "Monitor and manage your Coinbase portfolio.",
		AuthMethod:  APIKeyAuth,
	},

	// ── Health ───────────────────────────────────────────────────────────────
	{
		Slug:        "google-fit",
		Name:        "Google Fit",
		Category:    CategoryHealth,
		Icon:        "google-fit",
		Description: "Sync health and fitness data from Google Fit.",
		AuthMethod:  OAuth2Auth,
	},
	{
		Slug:        "fitbit",
		Name:        "Fitbit",
		Category:    CategoryHealth,
		Icon:        "fitbit",
		Description: "Sync sleep, activity, and health data from Fitbit.",
		AuthMethod:  OAuth2Auth,
	},
	{
		Slug:        "oura",
		Name:        "Oura Ring",
		Category:    CategoryHealth,
		Icon:        "oura",
		Description: "Sync sleep and recovery data from your Oura Ring.",
		AuthMethod:  APIKeyAuth,
	},
	{
		Slug:        "whoop",
		Name:        "WHOOP",
		Category:    CategoryHealth,
		Icon:        "whoop",
		Description: "Sync strain, recovery, and sleep data from WHOOP.",
		AuthMethod:  OAuth2Auth,
	},

	// ── Billing ──────────────────────────────────────────────────────────────
	{
		Slug:        "stripe",
		Name:        "Stripe",
		Category:    CategoryBilling,
		Icon:        "stripe",
		Description: "Monitor Stripe payments and subscriptions.",
		AuthMethod:  APIKeyAuth,
	},
}
