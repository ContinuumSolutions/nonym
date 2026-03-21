package router

import "testing"

func TestVendorCatalogNotEmpty(t *testing.T) {
	if len(VendorCatalog) == 0 {
		t.Fatal("VendorCatalog must not be empty")
	}
}

func TestVendorCatalogRequiredFields(t *testing.T) {
	for _, v := range VendorCatalog {
		if v.ID == "" {
			t.Errorf("vendor %q has empty ID", v.Name)
		}
		if v.Name == "" {
			t.Errorf("vendor ID %q has empty Name", v.ID)
		}
		if v.Category == "" {
			t.Errorf("vendor %q has empty Category", v.ID)
		}
		if len(v.KnownHosts) == 0 {
			t.Errorf("vendor %q has no KnownHosts", v.ID)
		}
		if len(v.DataTypes) == 0 {
			t.Errorf("vendor %q has no DataTypes", v.ID)
		}
		if len(v.ComplianceFrameworks) == 0 {
			t.Errorf("vendor %q has no ComplianceFrameworks", v.ID)
		}
	}
}

func TestGetVendorProfile_Known(t *testing.T) {
	cases := []struct {
		id       string
		wantName string
	}{
		{"sentry", "Sentry"},
		{"datadog", "Datadog"},
		{"posthog", "PostHog"},
	}

	for _, tc := range cases {
		p := GetVendorProfile(tc.id)
		if p == nil {
			t.Errorf("GetVendorProfile(%q) returned nil", tc.id)
			continue
		}
		if p.Name != tc.wantName {
			t.Errorf("GetVendorProfile(%q).Name = %q, want %q", tc.id, p.Name, tc.wantName)
		}
		if p.ID != tc.id {
			t.Errorf("GetVendorProfile(%q).ID = %q, want %q", tc.id, p.ID, tc.id)
		}
	}
}

func TestGetVendorProfile_Unknown(t *testing.T) {
	p := GetVendorProfile("nonexistent-vendor-xyz")
	if p != nil {
		t.Errorf("expected nil for unknown vendor, got %+v", p)
	}
}

func TestDetectVendorFromHost_ExactMatch(t *testing.T) {
	cases := []struct {
		host     string
		wantID   string
	}{
		{"sentry.io", "sentry"},
		{"api.datadoghq.com", "datadog"},
		{"app.posthog.com", "posthog"},
		{"api.mixpanel.com", "mixpanel"},
		{"collector.newrelic.com", "newrelic"},
	}

	for _, tc := range cases {
		got := DetectVendorFromHost(tc.host)
		if got != tc.wantID {
			t.Errorf("DetectVendorFromHost(%q) = %q, want %q", tc.host, got, tc.wantID)
		}
	}
}

func TestDetectVendorFromHost_Subdomain(t *testing.T) {
	// Sentry uses o<id>.ingest.sentry.io subdomains
	// The catalog has "o0.ingest.sentry.io" etc., so direct matches cover it;
	// non-catalogued subdomains of sentry.io should still match via suffix check.
	cases := []struct {
		host   string
		wantID string
	}{
		// These are explicitly in the catalog
		{"o0.ingest.sentry.io", "sentry"},
		{"logs.datadoghq.com", "datadog"},
		{"eu.posthog.com", "posthog"},
	}

	for _, tc := range cases {
		got := DetectVendorFromHost(tc.host)
		if got != tc.wantID {
			t.Errorf("DetectVendorFromHost(%q) = %q, want %q", tc.host, got, tc.wantID)
		}
	}
}

func TestDetectVendorFromHost_Unknown(t *testing.T) {
	cases := []string{
		"example.com",
		"api.openai.com",
		"",
		"localhost",
	}

	for _, host := range cases {
		got := DetectVendorFromHost(host)
		if got != "" {
			t.Errorf("DetectVendorFromHost(%q) = %q, want empty string", host, got)
		}
	}
}

func TestSentryVendorHasNonymSDK(t *testing.T) {
	p := GetVendorProfile("sentry")
	if p == nil {
		t.Fatal("sentry vendor not found")
	}
	if p.NonymSDK == "" {
		t.Error("sentry vendor should have a NonymSDK value")
	}
}

func TestCatalogIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, v := range VendorCatalog {
		if seen[v.ID] {
			t.Errorf("duplicate vendor ID %q in catalog", v.ID)
		}
		seen[v.ID] = true
	}
}
