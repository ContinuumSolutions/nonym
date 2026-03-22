package scanner

import "fmt"

// Connector is the interface every vendor adapter must implement.
// Add new vendors by implementing this interface and calling Register() in an init().
type Connector interface {
	// Vendor returns the slug that matches VendorConnection.Vendor (e.g. "sentry").
	Vendor() string

	// FetchEvents calls the vendor API and returns a flat list of normalised
	// events ready for PII detection.  The slice may be empty but never nil on
	// a clean run.  Implementations should cap the number of events they fetch
	// to avoid unbounded scans (recommended: ≤ 500 events per call).
	FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error)
}

var registry = map[string]Connector{}

// Register adds a connector to the global registry.  Call this from init().
func Register(c Connector) {
	registry[c.Vendor()] = c
}

// connectorFor returns the registered connector for a vendor slug, or nil.
func connectorFor(vendor string) Connector {
	return registry[vendor]
}

// ErrNoConnector is returned when no real connector exists for a vendor and
// the caller needs to know (rather than silently falling back).
type ErrNoConnector struct{ Vendor string }

func (e ErrNoConnector) Error() string {
	return fmt.Sprintf("no connector registered for vendor %q", e.Vendor)
}
