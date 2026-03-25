package scanner

import (
	"encoding/json"
	"time"
)

// VendorConnection represents an org's authenticated connection to an external vendor API.
type VendorConnection struct {
	ID               string                 `json:"id"`
	OrgID            int                    `json:"org_id"`
	Vendor           string                 `json:"vendor"`
	DisplayName      string                 `json:"display_name"`
	Status           string                 `json:"status"`          // connected | disconnected | error
	ScanStatus       string                 `json:"scan_status"`     // idle | scanning
	AuthType         string                 `json:"auth_type"`       // api_key | oauth
	Credentials      map[string]interface{} `json:"credentials"`     // masked — never expose raw secrets
	Settings         map[string]interface{} `json:"settings"`
	HostingRegion    string                 `json:"hosting_region"`  // US | EU | AU | "" (auto-detected or user-set)
	ConnectedAt      *time.Time             `json:"connected_at"`
	LastScanAt       *time.Time             `json:"last_scan_at"`
	ErrorMessage     string                 `json:"error_message,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

// Scan represents a scanning job across one or more vendor connections.
type Scan struct {
	ID            string     `json:"id"`
	OrgID         int        `json:"org_id"`
	VendorIDs     []string   `json:"vendor_ids"`
	Status        string     `json:"status"`          // pending | running | done | failed
	StartedAt     *time.Time `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	FindingsCount int        `json:"findings_count"`
	ErrorMessage  string     `json:"error_message,omitempty"`
	TriggeredBy   string     `json:"triggered_by"`    // manual | scheduled | onboarding
	CreatedAt     time.Time  `json:"created_at"`
}

// Finding represents a single detected PII or sensitive-data finding.
type Finding struct {
	ID                 string              `json:"id"`
	OrgID              int                 `json:"org_id"`
	ScanID             string              `json:"scan_id"`
	VendorConnectionID string              `json:"vendor_connection_id"`
	Vendor             string              `json:"vendor"`
	DataType           string              `json:"data_type"`    // email | phone | name | ip_address | api_key | token | financial | health
	RiskLevel          string              `json:"risk_level"`   // high | medium | low
	Title              string              `json:"title"`
	Description        string              `json:"description"`
	Location           string              `json:"location,omitempty"`
	Endpoint           string              `json:"endpoint,omitempty"`
	Occurrences        int                 `json:"occurrences"`
	SampleMasked       string              `json:"sample_masked,omitempty"`
	Status             string              `json:"status"`       // open | resolved | suppressed
	ComplianceImpact   []ComplianceImpact  `json:"compliance_impact"`
	Fixes              []Fix               `json:"fixes"`
	FirstSeenAt        time.Time           `json:"first_seen_at"`
	LastSeenAt         time.Time           `json:"last_seen_at"`
	ResolvedAt         *time.Time          `json:"resolved_at"`
	ResolvedBy         *int                `json:"resolved_by"`
	CreatedAt          time.Time           `json:"created_at"`
}

// ComplianceImpact maps a finding's data type to a compliance framework article.
type ComplianceImpact struct {
	Framework string `json:"framework"`
	Article   string `json:"article"`
	RiskLevel string `json:"risk_level"`
}

// Fix provides a code or config snippet to remediate a finding.
type Fix struct {
	Language    string `json:"language"`
	Code        string `json:"code"`
	Description string `json:"description"`
}

// Report represents a generated compliance report.
type Report struct {
	ID          string                 `json:"id"`
	OrgID       int                    `json:"org_id"`
	Framework   string                 `json:"framework"`   // GDPR | SOC2 | HIPAA | Custom
	TimeRange   string                 `json:"time_range"`
	Options     map[string]interface{} `json:"options"`
	Status      string                 `json:"status"`      // pending | generating | done | failed
	FileURL     string                 `json:"file_url,omitempty"`
	ShareToken  string                 `json:"share_token,omitempty"`
	GeneratedAt *time.Time             `json:"generated_at"`
	ExpiresAt   *time.Time             `json:"expires_at"`
	CreatedAt   time.Time              `json:"created_at"`
}

// ScannerOverview is returned by GET /api/v1/scanner/overview.
type ScannerOverview struct {
	VendorsConnected int                `json:"vendors_connected"`
	Findings         FindingCounts      `json:"findings"`
	RiskScore        int                `json:"risk_score"`
	Compliance       ComplianceSnapshot `json:"compliance"`
	LastScanAt       *time.Time         `json:"last_scan_at"`
}

// FindingCounts groups findings by risk level.
type FindingCounts struct {
	High   int `json:"high"`
	Medium int `json:"medium"`
	Low    int `json:"low"`
	Total  int `json:"total"`
}

// ComplianceSnapshot summarises framework coverage.
type ComplianceSnapshot struct {
	GDPR  FrameworkStatus `json:"GDPR"`
	SOC2  FrameworkStatus `json:"SOC2"`
	HIPAA FrameworkStatus `json:"HIPAA"`
}

// FrameworkStatus indicates how many findings affect a framework.
type FrameworkStatus struct {
	Violations int    `json:"violations"`
	Status     string `json:"status"` // ok | warning | critical
}

// DataFlows is returned by GET /api/v1/scanner/flows.
type DataFlows struct {
	Nodes []FlowNode `json:"nodes"`
	Edges []FlowEdge `json:"edges"`
}

// FlowNode represents an app or vendor in the data-flow graph.
type FlowNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"` // app | vendor
}

// FlowEdge connects two nodes and reports finding counts.
type FlowEdge struct {
	From          string `json:"from"`
	To            string `json:"to"`
	FindingsCount int    `json:"findings_count"`
	RiskLevel     string `json:"risk_level"`
}

// NormalizedEvent is a vendor-agnostic event ready for detection.
type NormalizedEvent struct {
	VendorID   string
	EventID    string
	Source     string // field path e.g. "event.user.email"
	Text       string
	Metadata   map[string]string
	RawPayload []byte

	// PreDetected carries results already identified by an upstream engine (e.g.
	// the proxy NER). When non-nil, Detect() is skipped and these are used directly.
	PreDetected []Detection
}

// marshalJSON is a helper for serialising slices to JSON strings for SQLite.
func marshalJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
