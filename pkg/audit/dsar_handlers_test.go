package audit

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	_ "modernc.org/sqlite"
)

// ── Test helpers ──────────────────────────────────────────────────────────────

func setupDsarTestDB(t *testing.T) {
	t.Helper()
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	origDB := db
	origPostgres := isPostgres
	db = testDB
	isPostgres = false

	t.Cleanup(func() {
		db = origDB
		isPostgres = origPostgres
		testDB.Close()
	})

	if err := InitializeDsarTables(); err != nil {
		t.Fatalf("InitializeDsarTables: %v", err)
	}
}

func dsarAuthMiddleware(orgID int) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return c.Next()
	}
}

func newDsarApp(t *testing.T) *fiber.App {
	t.Helper()
	setupDsarTestDB(t)
	app := fiber.New()
	app.Use(dsarAuthMiddleware(1))
	app.Get("/api/v1/dsars", HandleListDsars)
	app.Post("/api/v1/dsars", HandleCreateDsar)
	app.Get("/api/v1/dsars/:id", HandleGetDsar)
	app.Patch("/api/v1/dsars/:id", HandleUpdateDsar)
	app.Get("/api/v1/subjects/lookup", HandleSubjectLookup)
	app.Post("/api/v1/subjects/:id/export", HandleExportSubjectData)
	app.Post("/api/v1/subjects/:id/erase", HandleEraseSubject)
	return app
}

// seedDsar inserts a DSAR directly into the test DB.
func seedDsar(t *testing.T, orgID int, dsarType, status string) DsarRequest {
	t.Helper()
	ref := fmt.Sprintf("DSAR-TEST-%03d", time.Now().UnixNano()%1000)
	now := time.Now().UTC()
	d := &DsarRequest{
		ID:                    "dsar_" + ref,
		OrganizationID:        orgID,
		Reference:             ref,
		Type:                  DsarType(dsarType),
		SubjectEmail:          "alice@example.com",
		SubjectName:           "Alice Test",
		Status:                DsarStatus(status),
		CreatedAt:             now,
		DeadlineAt:            now.AddDate(0, 0, 30),
		Notes:                 "test note",
		ProvidersInvolved:     []string{"openai"},
		ErasureCertificateURL: "",
	}
	if err := insertDsar(d); err != nil {
		t.Fatalf("seedDsar: %v", err)
	}
	return *d
}

// seedSubject inserts a subject directly into the test DB.
func seedSubject(t *testing.T, orgID int, email, name string) SubjectProfile {
	t.Helper()
	id, err := upsertSubject(orgID, email, name)
	if err != nil {
		t.Fatalf("seedSubject: %v", err)
	}
	return SubjectProfile{ID: id, Email: email, Name: name}
}

// ── Unit tests — models & DB helpers ──────────────────────────────────────────

func TestNextDsarReference_Sequential(t *testing.T) {
	setupDsarTestDB(t)

	r1, err := nextDsarReference()
	if err != nil {
		t.Fatalf("nextDsarReference: %v", err)
	}
	r2, _ := nextDsarReference()
	r3, _ := nextDsarReference()

	year := time.Now().Year()
	if r1 != fmt.Sprintf("DSAR-%d-001", year) {
		t.Errorf("expected DSAR-%d-001, got %s", year, r1)
	}
	if r2 != fmt.Sprintf("DSAR-%d-002", year) {
		t.Errorf("expected DSAR-%d-002, got %s", year, r2)
	}
	if r3 != fmt.Sprintf("DSAR-%d-003", year) {
		t.Errorf("expected DSAR-%d-003, got %s", year, r3)
	}
}

func TestInsertAndGetDsar(t *testing.T) {
	setupDsarTestDB(t)

	now := time.Now().UTC().Truncate(time.Second)
	d := &DsarRequest{
		ID:                "dsar_unit_1",
		OrganizationID:    1,
		Reference:         "DSAR-2026-001",
		Type:              DsarTypeErasure,
		SubjectEmail:      "bob@example.com",
		SubjectName:       "Bob Smith",
		Status:            DsarStatusPending,
		CreatedAt:         now,
		DeadlineAt:        now.AddDate(0, 0, 30),
		Notes:             "unit test note",
		ProvidersInvolved: []string{"openai", "anthropic"},
	}
	if err := insertDsar(d); err != nil {
		t.Fatalf("insertDsar: %v", err)
	}

	got, err := getDsar(1, "dsar_unit_1")
	if err != nil {
		t.Fatalf("getDsar: %v", err)
	}
	if got.SubjectEmail != "bob@example.com" {
		t.Errorf("email mismatch: %s", got.SubjectEmail)
	}
	if got.Type != DsarTypeErasure {
		t.Errorf("type mismatch: %s", got.Type)
	}
	if len(got.ProvidersInvolved) != 2 {
		t.Errorf("providers mismatch: %v", got.ProvidersInvolved)
	}
}

func TestGetDsar_WrongOrg(t *testing.T) {
	setupDsarTestDB(t)
	seedDsar(t, 1, "access", "pending")

	_, err := getDsar(2, "dsar_unit_999") // different org
	if err == nil {
		t.Error("expected error for wrong org, got nil")
	}
}

func TestListDsars_EmptyAndPagination(t *testing.T) {
	setupDsarTestDB(t)

	dsars, total, err := listDsars(1, 10, 0, "", "")
	if err != nil {
		t.Fatalf("listDsars: %v", err)
	}
	if total != 0 || len(dsars) != 0 {
		t.Errorf("expected empty list, got %d/%d", len(dsars), total)
	}

	for i := 0; i < 5; i++ {
		seedDsar(t, 1, "access", "pending")
	}

	dsars, total, _ = listDsars(1, 3, 0, "", "")
	if total != 5 {
		t.Errorf("total: expected 5, got %d", total)
	}
	if len(dsars) != 3 {
		t.Errorf("page: expected 3, got %d", len(dsars))
	}
}

func TestListDsars_StatusFilter(t *testing.T) {
	setupDsarTestDB(t)
	seedDsar(t, 1, "access", "pending")
	seedDsar(t, 1, "erasure", "completed")
	seedDsar(t, 1, "access", "in_review")

	pending, total, _ := listDsars(1, 100, 0, "pending", "")
	if total != 1 || len(pending) != 1 {
		t.Errorf("pending filter: expected 1, got %d/%d", len(pending), total)
	}
}

func TestDsarCounts(t *testing.T) {
	setupDsarTestDB(t)
	seedDsar(t, 1, "access", "pending")
	seedDsar(t, 1, "erasure", "in_review")
	seedDsar(t, 1, "access", "completed")

	open, pending, completed, _, err := dsarCounts(1)
	if err != nil {
		t.Fatalf("dsarCounts: %v", err)
	}
	if open != 2 {
		t.Errorf("open: expected 2, got %d", open)
	}
	if pending != 1 {
		t.Errorf("pending: expected 1, got %d", pending)
	}
	if completed != 1 {
		t.Errorf("completed: expected 1, got %d", completed)
	}
}

func TestUpsertSubject_CreateAndIdempotent(t *testing.T) {
	setupDsarTestDB(t)

	id1, err := upsertSubject(1, "carol@example.com", "Carol")
	if err != nil {
		t.Fatalf("upsertSubject create: %v", err)
	}
	id2, err := upsertSubject(1, "carol@example.com", "Carol")
	if err != nil {
		t.Fatalf("upsertSubject idempotent: %v", err)
	}
	if id1 != id2 {
		t.Errorf("IDs should match: %s vs %s", id1, id2)
	}
}

// ── Functional tests — HTTP handlers ──────────────────────────────────────────

// HandleListDsars

func TestHandleListDsars_Empty(t *testing.T) {
	app := newDsarApp(t)

	req := httptest.NewRequest("GET", "/api/v1/dsars", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result DsarListResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Total != 0 {
		t.Errorf("expected total=0, got %d", result.Total)
	}
	if result.Requests == nil {
		t.Error("requests should not be nil")
	}
}

func TestHandleListDsars_WithRecords(t *testing.T) {
	app := newDsarApp(t)
	seedDsar(t, 1, "access", "pending")
	seedDsar(t, 1, "erasure", "completed")

	req := httptest.NewRequest("GET", "/api/v1/dsars", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result DsarListResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Total != 2 {
		t.Errorf("expected total=2, got %d", result.Total)
	}
	if result.PendingCount != 1 {
		t.Errorf("expected pending=1, got %d", result.PendingCount)
	}
	if result.CompletedCount != 1 {
		t.Errorf("expected completed=1, got %d", result.CompletedCount)
	}
}

func TestHandleListDsars_StatusFilter(t *testing.T) {
	app := newDsarApp(t)
	seedDsar(t, 1, "access", "pending")
	seedDsar(t, 1, "erasure", "in_review")

	req := httptest.NewRequest("GET", "/api/v1/dsars?status=pending", nil)
	resp, _ := app.Test(req, -1)

	var result DsarListResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Total != 1 {
		t.Errorf("expected 1 pending, got %d", result.Total)
	}
}

func TestHandleListDsars_TypeFilter(t *testing.T) {
	app := newDsarApp(t)
	seedDsar(t, 1, "access", "pending")
	seedDsar(t, 1, "erasure", "pending")

	req := httptest.NewRequest("GET", "/api/v1/dsars?type=erasure", nil)
	resp, _ := app.Test(req, -1)

	var result DsarListResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Total != 1 {
		t.Errorf("expected 1 erasure, got %d", result.Total)
	}
	if result.Requests[0].Type != DsarTypeErasure {
		t.Errorf("wrong type: %s", result.Requests[0].Type)
	}
}

// HandleCreateDsar

func TestHandleCreateDsar_Success(t *testing.T) {
	app := newDsarApp(t)

	body := map[string]string{
		"type":          "erasure",
		"subject_email": "dave@example.com",
		"subject_name":  "Dave Jones",
		"notes":         "Ref GDPR Art 17",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/dsars", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result DsarRequest
	json.NewDecoder(resp.Body).Decode(&result)

	if result.ID == "" {
		t.Error("id should not be empty")
	}
	if result.Reference == "" {
		t.Error("reference should not be empty")
	}
	if result.Status != DsarStatusPending {
		t.Errorf("status should be pending, got %s", result.Status)
	}
	if result.Type != DsarTypeErasure {
		t.Errorf("type mismatch: %s", result.Type)
	}
	if result.DeadlineAt.IsZero() {
		t.Error("deadline_at should be set")
	}
	// Deadline should be ~30 days from now.
	expectedDeadline := time.Now().AddDate(0, 0, 30)
	diff := result.DeadlineAt.Sub(expectedDeadline)
	if diff < -time.Minute || diff > time.Minute {
		t.Errorf("deadline_at unexpected: %v", result.DeadlineAt)
	}
}

func TestHandleCreateDsar_InvalidType(t *testing.T) {
	app := newDsarApp(t)

	body := map[string]string{
		"type":          "delete_everything",
		"subject_email": "eve@example.com",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/dsars", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestHandleCreateDsar_MissingEmail(t *testing.T) {
	app := newDsarApp(t)

	body := map[string]string{"type": "access"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/dsars", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestHandleCreateDsar_AutoReference(t *testing.T) {
	app := newDsarApp(t)

	createOne := func(email string) DsarRequest {
		body, _ := json.Marshal(map[string]string{
			"type": "access", "subject_email": email,
		})
		req := httptest.NewRequest("POST", "/api/v1/dsars", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)
		var d DsarRequest
		json.NewDecoder(resp.Body).Decode(&d)
		return d
	}

	d1 := createOne("f1@example.com")
	d2 := createOne("f2@example.com")

	if d1.Reference == d2.Reference {
		t.Errorf("references should differ: both=%s", d1.Reference)
	}
}

// HandleGetDsar

func TestHandleGetDsar_Found(t *testing.T) {
	app := newDsarApp(t)
	d := seedDsar(t, 1, "access", "pending")

	req := httptest.NewRequest("GET", "/api/v1/dsars/"+d.ID, nil)
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result DsarRequest
	json.NewDecoder(resp.Body).Decode(&result)
	if result.ID != d.ID {
		t.Errorf("id mismatch: expected %s, got %s", d.ID, result.ID)
	}
}

func TestHandleGetDsar_NotFound(t *testing.T) {
	app := newDsarApp(t)

	req := httptest.NewRequest("GET", "/api/v1/dsars/nonexistent", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// HandleUpdateDsar

func TestHandleUpdateDsar_Status(t *testing.T) {
	app := newDsarApp(t)
	d := seedDsar(t, 1, "access", "pending")

	body, _ := json.Marshal(map[string]string{"status": "in_review"})
	req := httptest.NewRequest("PATCH", "/api/v1/dsars/"+d.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result DsarRequest
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Status != DsarStatusInReview {
		t.Errorf("expected in_review, got %s", result.Status)
	}
}

func TestHandleUpdateDsar_Complete(t *testing.T) {
	app := newDsarApp(t)
	d := seedDsar(t, 1, "access", "in_review")

	body, _ := json.Marshal(map[string]string{"status": "completed"})
	req := httptest.NewRequest("PATCH", "/api/v1/dsars/"+d.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)

	var result DsarRequest
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Status != DsarStatusCompleted {
		t.Errorf("expected completed, got %s", result.Status)
	}
	if result.CompletedAt == nil {
		t.Error("completed_at should be set")
	}
}

func TestHandleUpdateDsar_Assign(t *testing.T) {
	app := newDsarApp(t)
	d := seedDsar(t, 1, "erasure", "pending")

	body, _ := json.Marshal(map[string]string{"assigned_to": "sarah@company.io"})
	req := httptest.NewRequest("PATCH", "/api/v1/dsars/"+d.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)

	var result DsarRequest
	json.NewDecoder(resp.Body).Decode(&result)
	if result.AssignedTo != "sarah@company.io" {
		t.Errorf("assignedTo mismatch: %s", result.AssignedTo)
	}
}

func TestHandleUpdateDsar_ExtendDeadline(t *testing.T) {
	app := newDsarApp(t)
	d := seedDsar(t, 1, "access", "pending")

	extended := time.Now().AddDate(0, 0, 90).UTC().Truncate(time.Second)
	body, _ := json.Marshal(map[string]interface{}{
		"extended_deadline_at": extended,
	})
	req := httptest.NewRequest("PATCH", "/api/v1/dsars/"+d.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)

	var result DsarRequest
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Status != DsarStatusExtended {
		t.Errorf("expected extended status, got %s", result.Status)
	}
	if result.ExtendedDeadlineAt == nil {
		t.Error("extended_deadline_at should be set")
	}
}

func TestHandleUpdateDsar_InvalidStatus(t *testing.T) {
	app := newDsarApp(t)
	d := seedDsar(t, 1, "access", "pending")

	body, _ := json.Marshal(map[string]string{"status": "explode"})
	req := httptest.NewRequest("PATCH", "/api/v1/dsars/"+d.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestHandleUpdateDsar_NotFound(t *testing.T) {
	app := newDsarApp(t)

	body, _ := json.Marshal(map[string]string{"status": "in_review"})
	req := httptest.NewRequest("PATCH", "/api/v1/dsars/ghost", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// HandleSubjectLookup

func TestHandleSubjectLookup_MissingEmail(t *testing.T) {
	app := newDsarApp(t)

	req := httptest.NewRequest("GET", "/api/v1/subjects/lookup", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestHandleSubjectLookup_NotFound(t *testing.T) {
	app := newDsarApp(t)

	// Unknown subject → returns zero-value profile (not 404 — subject can still file a DSAR)
	req := httptest.NewRequest("GET", "/api/v1/subjects/lookup?email=ghost@example.com", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var sp SubjectProfile
	json.NewDecoder(resp.Body).Decode(&sp)
	if sp.Email != "ghost@example.com" {
		t.Errorf("email mismatch: %s", sp.Email)
	}
	if sp.ActiveDsar != nil {
		t.Error("active_dsar should be nil for unknown subject")
	}
}

func TestHandleSubjectLookup_Found_WithActiveDsar(t *testing.T) {
	app := newDsarApp(t)
	seedSubject(t, 1, "alice@example.com", "Alice Test")
	seedDsar(t, 1, "erasure", "in_review")

	req := httptest.NewRequest("GET", "/api/v1/subjects/lookup?email=alice@example.com", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var sp SubjectProfile
	json.NewDecoder(resp.Body).Decode(&sp)
	if sp.Email != "alice@example.com" {
		t.Errorf("email: %s", sp.Email)
	}
	if sp.ActiveDsar == nil {
		t.Error("expected active DSAR, got nil")
	}
}

func TestHandleSubjectLookup_NoActiveDsar_WhenCompleted(t *testing.T) {
	app := newDsarApp(t)
	seedSubject(t, 1, "alice@example.com", "Alice Test")
	seedDsar(t, 1, "erasure", "completed")

	req := httptest.NewRequest("GET", "/api/v1/subjects/lookup?email=alice@example.com", nil)
	resp, _ := app.Test(req, -1)

	var sp SubjectProfile
	json.NewDecoder(resp.Body).Decode(&sp)
	if sp.ActiveDsar != nil {
		t.Error("completed DSAR should not appear as active")
	}
}

// HandleExportSubjectData

func TestHandleExportSubjectData_NotFound(t *testing.T) {
	app := newDsarApp(t)

	req := httptest.NewRequest("POST", "/api/v1/subjects/subj_ghost/export", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestHandleExportSubjectData_Found(t *testing.T) {
	app := newDsarApp(t)
	sp := seedSubject(t, 1, "export@example.com", "Export User")

	req := httptest.NewRequest("POST", "/api/v1/subjects/"+sp.ID+"/export", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["download_url"] == "" {
		t.Error("download_url should not be empty")
	}
}

// HandleEraseSubject

func TestHandleEraseSubject_NotFound(t *testing.T) {
	app := newDsarApp(t)

	req := httptest.NewRequest("POST", "/api/v1/subjects/subj_ghost/erase", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestHandleEraseSubject_Success(t *testing.T) {
	app := newDsarApp(t)
	sp := seedSubject(t, 1, "erase@example.com", "Erase User")
	seedDsar(t, 1, "erasure", "in_review")

	req := httptest.NewRequest("POST", "/api/v1/subjects/"+sp.ID+"/erase", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result ErasureResult
	json.NewDecoder(resp.Body).Decode(&result)
	if result.ErasureID == "" {
		t.Error("erasure_id should not be empty")
	}
	if result.CertificateURL == "" {
		t.Error("certificate_url should not be empty")
	}
	if result.Timestamp.IsZero() {
		t.Error("timestamp should be set")
	}
	if result.SubProcessorResults == nil {
		t.Error("sub_processor_results should not be nil")
	}
}

func TestHandleEraseSubject_MarksDsarCompleted(t *testing.T) {
	setupDsarTestDB(t)

	// Seed a subject and an active DSAR.
	subjectID, _ := upsertSubject(1, "erase2@example.com", "Erase Two")
	d := &DsarRequest{
		ID:             "dsar_erase_test",
		OrganizationID: 1,
		Reference:      "DSAR-ERASE-001",
		Type:           DsarTypeErasure,
		SubjectEmail:   "erase2@example.com",
		Status:         DsarStatusInReview,
		CreatedAt:      time.Now().UTC(),
		DeadlineAt:     time.Now().AddDate(0, 0, 30).UTC(),
		ProvidersInvolved: []string{"openai"},
	}
	insertDsar(d) //nolint:errcheck

	// Erase directly via HTTP.
	app := fiber.New()
	app.Use(dsarAuthMiddleware(1))
	app.Post("/api/v1/subjects/:id/erase", HandleEraseSubject)

	req := httptest.NewRequest("POST", "/api/v1/subjects/"+subjectID+"/erase", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify DSAR is now completed.
	updated, err := getDsar(1, "dsar_erase_test")
	if err != nil {
		t.Fatalf("getDsar after erase: %v", err)
	}
	if updated.Status != DsarStatusCompleted {
		t.Errorf("expected completed, got %s", updated.Status)
	}
	if updated.CompletedAt == nil {
		t.Error("completed_at should be set")
	}
	if updated.ErasureCertificateURL == "" {
		t.Error("erasure_certificate_url should be set after erase")
	}
}

// ── Auth guard tests ──────────────────────────────────────────────────────────

func TestDsarEndpoints_RequireAuth(t *testing.T) {
	setupDsarTestDB(t)
	app := fiber.New()
	// No auth middleware — organization_id will be missing
	app.Get("/api/v1/dsars", HandleListDsars)
	app.Post("/api/v1/dsars", HandleCreateDsar)
	app.Get("/api/v1/dsars/:id", HandleGetDsar)
	app.Patch("/api/v1/dsars/:id", HandleUpdateDsar)
	app.Get("/api/v1/subjects/lookup", HandleSubjectLookup)
	app.Post("/api/v1/subjects/:id/export", HandleExportSubjectData)
	app.Post("/api/v1/subjects/:id/erase", HandleEraseSubject)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/dsars"},
		{"POST", "/api/v1/dsars"},
		{"GET", "/api/v1/dsars/x"},
		{"PATCH", "/api/v1/dsars/x"},
		{"GET", "/api/v1/subjects/lookup"},
		{"POST", "/api/v1/subjects/x/export"},
		{"POST", "/api/v1/subjects/x/erase"},
	}
	for _, e := range endpoints {
		req := httptest.NewRequest(e.method, e.path, nil)
		resp, _ := app.Test(req, -1)
		if resp.StatusCode != 401 {
			t.Errorf("%s %s: expected 401, got %d", e.method, e.path, resp.StatusCode)
		}
	}
}

// ── OrgID isolation test ──────────────────────────────────────────────────────

func TestDsarOrgIsolation(t *testing.T) {
	setupDsarTestDB(t)

	// Seed DSAR for org 1.
	d := &DsarRequest{
		ID:             "dsar_org1",
		OrganizationID: 1,
		Reference:      "DSAR-2026-999",
		Type:           DsarTypeAccess,
		SubjectEmail:   "org1@example.com",
		Status:         DsarStatusPending,
		CreatedAt:      time.Now().UTC(),
		DeadlineAt:     time.Now().AddDate(0, 0, 30).UTC(),
		ProvidersInvolved: []string{},
	}
	insertDsar(d) //nolint:errcheck

	// Org 2 should not see org 1's DSAR.
	dsars, total, _ := listDsars(2, 100, 0, "", "")
	if total != 0 || len(dsars) != 0 {
		t.Errorf("org 2 should not see org 1 DSARs, got %d", total)
	}

	// Org 1 should see its own DSAR.
	dsars, total, _ = listDsars(1, 100, 0, "", "")
	if total != 1 {
		t.Errorf("org 1 should see 1 DSAR, got %d", total)
	}
}
