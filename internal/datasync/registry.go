package datasync

// DefaultAdapters returns the full set of built-in service adapters.
// Mirrors the slugs defined in integrations/catalog.go.
//
// WhatsApp is NOT included here because it requires a webhook verify token at
// construction time. Create it with NewWhatsAppAdapter and append it manually,
// or use NewDefaultAdapters which does this automatically.
func DefaultAdapters() []Adapter {
	return []Adapter{
		&GmailAdapter{},
		&GoogleCalendarAdapter{},
		&OutlookMailAdapter{},
		&OutlookCalendarAdapter{},
		&SlackAdapter{},
		&ZohoMailAdapter{},
		&PlaidAdapter{},
		&StripeAdapter{},
		&OuraAdapter{},
		&FitbitAdapter{},
		&WhoopAdapter{},
		&NotionAdapter{},
		&IntaSendAdapter{},
	}
}

// NewDefaultAdapters returns DefaultAdapters plus a WhatsAppAdapter wired with
// the given verify token. The same *WhatsAppAdapter is returned separately so
// the caller can register its webhook routes on the HTTP router.
func NewDefaultAdapters(whatsappVerifyToken string) ([]Adapter, *WhatsAppAdapter) {
	wa := NewWhatsAppAdapter(whatsappVerifyToken)
	return append(DefaultAdapters(), wa), wa
}
