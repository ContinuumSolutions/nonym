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
	AuthURL      string            // authorization endpoint
	TokenURL     string            // token exchange endpoint
	RevokeURL    string            // token revocation endpoint (optional)
	Scopes       []string          // space-joined into the scope query param
	ExtraParams  map[string]string // additional query params appended to the auth URL
	NoPKCE       bool              // skip code_challenge/code_verifier (e.g. Notion)
	UseBasicAuth bool              // send client_id:client_secret as HTTP Basic auth on token exchange (e.g. Notion)
}

// Categories available for both built-in and custom services.
const (
	CategoryCalendar      = "Calendar"
	CategoryCommunication = "Communication"
	CategoryFinance       = "Finance"
	CategoryHealth        = "Health"
	CategoryBilling       = "Billing"
	CategoryProductivity  = "Productivity"
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
		Description: "Personal or work calendar used to track time commitments. Signals are upcoming or newly created events — meetings, appointments, calls, and reminders. Used to detect schedule conflicts and protect focused time.",
		AuthMethod:  OAuth2Auth,
		AuthURL:     "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:    "https://oauth2.googleapis.com/token",
		RevokeURL:   "https://oauth2.googleapis.com/revoke",
		Scopes:      []string{"https://www.googleapis.com/auth/calendar.events"},
		ExtraParams: map[string]string{"access_type": "offline", "prompt": "consent"},
	},
	{
		Slug:        "outlook-calendar",
		Name:        "Outlook Calendar",
		Category:    CategoryCalendar,
		Icon:        "outlook",
		Color:       "#0078D4",
		Description: "Microsoft work or personal calendar used to track time commitments. Signals are upcoming meetings, appointments, and scheduled events. Used to detect schedule conflicts and protect focused time.",
		AuthMethod:  OAuth2Auth,
		AuthURL:     "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
		TokenURL:    "https://login.microsoftonline.com/common/oauth2/v2.0/token",
		Scopes:      []string{"Calendars.ReadWrite", "offline_access"},
	},

	// ── Communication ────────────────────────────────────────────────────────
	{
		Slug:        "gmail",
		Name:        "Gmail",
		Category:    CategoryCommunication,
		Icon:        "gmail",
		Color:       "#EA4335",
		Description: "Personal or work email inbox. Signals are unread emails from real senders — client requests, invoices, proposals, follow-ups, and personal messages. Used to triage inbound communication and surface action items.",
		AuthMethod:  OAuth2Auth,
		AuthURL:     "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:    "https://oauth2.googleapis.com/token",
		RevokeURL:   "https://oauth2.googleapis.com/revoke",
		Scopes:      []string{"https://www.googleapis.com/auth/gmail.readonly", "https://www.googleapis.com/auth/gmail.modify"},
		ExtraParams: map[string]string{"access_type": "offline", "prompt": "consent"},
	},
	{
		Slug:        "outlook-mail",
		Name:        "Outlook Mail",
		Category:    CategoryCommunication,
		Icon:        "outlook",
		Color:       "#0078D4",
		Description: "Microsoft work email inbox. Signals are unread emails — business communications, client requests, invoices, project updates, and internal messages. Used to triage work communications and identify action items.",
		AuthMethod:  OAuth2Auth,
		AuthURL:     "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
		TokenURL:    "https://login.microsoftonline.com/common/oauth2/v2.0/token",
		Scopes:      []string{"Mail.ReadWrite", "offline_access"},
	},
	{
		Slug:        "slack",
		Name:        "Slack",
		Category:    CategoryCommunication,
		Icon:        "slack",
		Color:       "#4A154B",
		Description: "Team messaging platform used for work collaboration. Signals are messages from colleagues, channel updates, and direct requests — not a social network. Used to identify tasks, decisions, and asks from teammates.",
		AuthMethod:  OAuth2Auth,
		AuthURL:     "https://slack.com/oauth/v2/authorize",
		TokenURL:    "https://slack.com/api/oauth.v2.access",
		RevokeURL:   "https://slack.com/api/auth.revoke",
		Scopes:      []string{"channels:read", "channels:history"},
	},
	{
		Slug:        "zoho-mail",
		Name:        "Zoho Mail",
		Category:    CategoryCommunication,
		Icon:        "zoho-mail",
		Color:       "#E42527",
		Description: "Business email inbox used for client and professional communications. Signals are unread emails — client inquiries, business proposals, invoices, and follow-ups. Used to triage inbound business mail and surface action items.",
		AuthMethod:  OAuth2Auth,
		AuthURL:     "https://accounts.zoho.com/oauth/v2/auth",
		TokenURL:    "https://accounts.zoho.com/oauth/v2/token",
		RevokeURL:   "https://accounts.zoho.com/oauth/v2/token/revoke",
		Scopes:      []string{"ZohoMail.accounts.READ", "ZohoMail.messages.READ"},
	},
	{
		Slug:        "telegram",
		Name:        "Telegram",
		Category:    CategoryCommunication,
		Icon:        "telegram",
		Color:       "#26A5E4",
		Description: "Messaging app used for personal and professional conversations via a bot. Signals are direct messages — personal conversations, business inquiries, alerts, and group notifications. Not a social media feed.",
		AuthMethod:  APIKeyAuth,
	},

	// ── Finance ──────────────────────────────────────────────────────────────
	{
		Slug:        "plaid",
		Name:        "Plaid",
		Category:    CategoryFinance,
		Icon:        "plaid",
		Color:       "#01ACAE",
		Description: "Bank account aggregator that connects to real bank accounts. Signals are actual bank transactions — spending, deposits, transfers, and account activity. Used to monitor personal cash flow and financial health. Not a payment processor.",
		AuthMethod:  APIKeyAuth,
	},
	{
		Slug:        "coinbase",
		Name:        "Coinbase",
		Category:    CategoryFinance,
		Icon:        "coinbase",
		Color:       "#0052FF",
		Description: "Cryptocurrency exchange and investment account. Signals are crypto transactions — buys, sells, transfers, staking rewards, and portfolio changes. Used to track crypto holdings and trading activity. This is an investment account, not a payment gateway.",
		AuthMethod:  APIKeyAuth,
	},

	// ── Health ───────────────────────────────────────────────────────────────
	{
		Slug:        "google-fit",
		Name:        "Google Fit",
		Category:    CategoryHealth,
		Icon:        "google-fit",
		Color:       "#34A853",
		Description: "Google's fitness tracking platform. Signals are daily health metrics — steps taken, active minutes, calories burned, and sleep duration. Used to monitor physical activity levels and set wellness context for decisions. Not financial data.",
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
		Description: "Wearable fitness tracker. Signals are daily biometric data — sleep stages, resting heart rate, active minutes, and step counts. Used to monitor physical health and recovery quality. Not financial data.",
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
		Description: "Smart ring for health and sleep monitoring. Signals are daily readiness, sleep quality, and activity scores (0–100). Used to assess recovery and determine how prepared the user is for demanding work. Not financial data.",
		AuthMethod:  APIKeyAuth,
	},
	{
		Slug:        "whoop",
		Name:        "WHOOP",
		Category:    CategoryHealth,
		Icon:        "whoop",
		Color:       "#3DFF8F",
		Description: "Athletic performance wearable focused on recovery and strain. Signals are daily recovery percentage, strain score, and sleep performance. Used to assess physical readiness and adjust decision thresholds accordingly. Not financial data.",
		AuthMethod:  OAuth2Auth,
		AuthURL:     "https://api.prod.whoop.com/oauth/oauth2/auth",
		TokenURL:    "https://api.prod.whoop.com/oauth/oauth2/token",
		Scopes:      []string{"read:recovery", "read:sleep", "read:workout"},
	},

	// ── Productivity ─────────────────────────────────────────────────────────
	{
		Slug:        "notion",
		Name:        "Notion",
		Category:    CategoryProductivity,
		Icon:        "notion",
		Color:       "#000000",
		Description: "Personal knowledge base and productivity workspace. Signals are page updates, new database entries, and document changes. Used to monitor tasks, notes, and project progress. Not a communication or financial platform.",
		AuthMethod:  APIKeyAuth,
	},

	// ── Billing ──────────────────────────────────────────────────────────────
	{
		Slug:        "intasend",
		Name:        "IntaSend",
		Category:    CategoryFinance,
		Icon:        "intasend",
		Color:       "#0A6EBD",
		Description: "Payment gateway used to track income. Signals represent money received from clients and customers — incoming payments and wallet transactions. Not an investment platform.",
		AuthMethod:  APIKeyAuth,
	},
	{
		Slug:        "stripe",
		Name:        "Stripe",
		Category:    CategoryBilling,
		Icon:        "stripe",
		Color:       "#635BFF",
		Description: "Payment processing platform for business revenue. Signals are incoming customer payments, subscription charges, and refunds. Used to track business revenue and billing events. Not a bank account — transactions are customer payments only.",
		AuthMethod:  APIKeyAuth,
	},
}
