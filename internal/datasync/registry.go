package datasync

// DefaultAdapters returns the full set of built-in service adapters.
// Mirrors the slugs defined in integrations/catalog.go.
func DefaultAdapters() []Adapter {
	return []Adapter{
		&GmailAdapter{},
		&GoogleCalendarAdapter{},
		&OutlookMailAdapter{},
		&OutlookCalendarAdapter{},
		&SlackAdapter{},
		&PlaidAdapter{},
		&StripeAdapter{},
		&OuraAdapter{},
		&FitbitAdapter{},
		&WhoopAdapter{},
	}
}
