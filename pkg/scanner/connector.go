package scanner

import "fmt"

// Connector is the interface every vendor adapter must implement.
// Add new vendors by implementing this interface and calling Register() in an init().
type Connector interface {
	// Vendor returns the slug that matches VendorConnection.Vendor (e.g. "sentry").
	Vendor() string

	// TestConnection verifies that the credentials are valid by making a
	// lightweight API call where possible. The result is used to update the
	// vendor connection status. Credential format validation belongs here —
	// testConnection in vendor_connections.go is a thin dispatcher only.
	TestConnection(vc *VendorConnection) ConnectionResult

	// FetchEvents calls the vendor API and returns a flat list of normalised
	// events ready for PII detection. The slice may be empty but never nil on
	// a clean run. Implementations should cap the number of events they fetch
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

// RegionDetector is an optional interface a Connector may implement to derive
// the vendor's hosting region from its credentials or settings.  The returned
// string should be a canonical region code: "US", "EU", "AU", "US-FED", etc.
// Return "" if the region cannot be determined from the available credentials.
// Connectors that are single-region (always US, globally distributed, etc.)
// should return the fixed value rather than implementing this interface so that
// a future catalogue default can override it without code changes.
type RegionDetector interface {
	DetectRegion(vc *VendorConnection) string
}

// detectRegion calls the connector's DetectRegion if it implements RegionDetector,
// otherwise returns "".
func detectRegion(vc *VendorConnection) string {
	if c := connectorFor(vc.Vendor); c != nil {
		if rd, ok := c.(RegionDetector); ok {
			return rd.DetectRegion(vc)
		}
	}
	return ""
}

// ErrNoConnector is returned when no real connector exists for a vendor and
// the caller needs to know (rather than silently falling back).
type ErrNoConnector struct{ Vendor string }

func (e ErrNoConnector) Error() string {
	return fmt.Sprintf("no connector registered for vendor %q", e.Vendor)
}

// credStr returns the first non-empty string credential value for any of the
// given keys. Used by connector TestConnection and FetchEvents implementations
// to avoid repeating the same credential-extraction loop.
func credStr(vc *VendorConnection, keys ...string) string {
	for _, k := range keys {
		if v, ok := vc.Credentials[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}
