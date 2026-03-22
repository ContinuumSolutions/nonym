package scanner

import (
	"database/sql"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	_ "modernc.org/sqlite"
)

// setupTestDB creates an in-memory SQLite database and initialises scanner tables.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Wire scanner package to the test DB.
	originalDB := db
	originalIsPostgres := isPostgres
	db = testDB
	isPostgres = false

	t.Cleanup(func() {
		db = originalDB
		isPostgres = originalIsPostgres
		testDB.Close()
	})

	if err := createTables(); err != nil {
		t.Fatalf("failed to create scanner tables: %v", err)
	}
	return testDB
}

// mockAuthMiddleware injects organization_id into fiber context.
func mockAuthMiddleware(orgID int) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return c.Next()
	}
}

// seedVendorConnection inserts a vendor connection and returns it.
func seedVendorConnection(t *testing.T, orgID int, vendor, status string) VendorConnection {
	t.Helper()
	now := time.Now()
	vc := VendorConnection{
		ID:          newID(),
		OrgID:       orgID,
		Vendor:      vendor,
		DisplayName: vendor,
		Status:      status,
		AuthType:    "api_key",
		Credentials: map[string]interface{}{"api_key": "sk-testkey12345678"},
		Settings:    map[string]interface{}{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := insertVendorConnection(&vc); err != nil {
		t.Fatalf("seedVendorConnection: %v", err)
	}
	return vc
}

// seedScan inserts a scan record and returns it.
func seedScan(t *testing.T, orgID int, status string) Scan {
	t.Helper()
	now := time.Now()
	s := Scan{
		ID:          newID(),
		OrgID:       orgID,
		VendorIDs:   []string{"sentry"},
		Status:      status,
		TriggeredBy: "manual",
		CreatedAt:   now,
	}
	if err := insertScan(&s); err != nil {
		t.Fatalf("seedScan: %v", err)
	}
	return s
}

// seedFinding inserts a finding record and returns it.
func seedFinding(t *testing.T, orgID int, scanID, vcID, vendor, dataType, riskLevel, status string) Finding {
	t.Helper()
	now := time.Now()
	f := Finding{
		ID:                 newID(),
		OrgID:              orgID,
		ScanID:             scanID,
		VendorConnectionID: vcID,
		Vendor:             vendor,
		DataType:           dataType,
		RiskLevel:          riskLevel,
		Title:              "Test finding",
		Description:        "Test description",
		Location:           "event.user.email",
		Endpoint:           "POST /login",
		Occurrences:        1,
		SampleMasked:       "t***@example.com",
		Status:             status,
		ComplianceImpact:   ComplianceFor(dataType),
		Fixes:              []Fix{},
		FirstSeenAt:        now,
		LastSeenAt:         now,
		CreatedAt:          now,
	}
	if err := insertFinding(&f); err != nil {
		t.Fatalf("seedFinding: %v", err)
	}
	return f
}
