package audit

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// ── DSAR Models ───────────────────────────────────────────────────────────────

// DsarType enumerates the GDPR request types.
type DsarType string

const (
	DsarTypeAccess        DsarType = "access"
	DsarTypeErasure       DsarType = "erasure"
	DsarTypeRectification DsarType = "rectification"
	DsarTypePortability   DsarType = "portability"
	DsarTypeObjection     DsarType = "objection"
)

// DsarStatus enumerates the lifecycle states of a DSAR.
type DsarStatus string

const (
	DsarStatusPending   DsarStatus = "pending"
	DsarStatusInReview  DsarStatus = "in_review"
	DsarStatusCompleted DsarStatus = "completed"
	DsarStatusRejected  DsarStatus = "rejected"
	DsarStatusExtended  DsarStatus = "extended"
)

// DsarRequest is the canonical DSAR record shape, matching the API contract.
type DsarRequest struct {
	ID                   string     `json:"id"`
	OrganizationID       int        `json:"organization_id,omitempty"`
	Reference            string     `json:"reference"`
	Type                 DsarType   `json:"type"`
	SubjectEmail         string     `json:"subject_email"`
	SubjectName          string     `json:"subject_name"`
	Status               DsarStatus `json:"status"`
	AssignedTo           string     `json:"assigned_to"`
	CreatedAt            time.Time  `json:"created_at"`
	DeadlineAt           time.Time  `json:"deadline_at"`
	ExtendedDeadlineAt   *time.Time `json:"extended_deadline_at"`
	CompletedAt          *time.Time `json:"completed_at"`
	Notes                string     `json:"notes"`
	TokenCount           int        `json:"token_count"`
	ProvidersInvolved    []string   `json:"providers_involved"`
	ErasureCertificateURL string    `json:"erasure_certificate_url"`
}

// DsarListResponse is returned by GET /api/v1/dsars.
type DsarListResponse struct {
	Requests       []DsarRequest `json:"requests"`
	Total          int           `json:"total"`
	OpenCount      int           `json:"open_count"`
	PendingCount   int           `json:"pending_count"`
	CompletedCount int           `json:"completed_count"`
	OverdueCount   int           `json:"overdue_count"`
}

// SubjectProfile is returned by GET /api/v1/subjects/lookup.
type SubjectProfile struct {
	ID               string       `json:"id"`
	Email            string       `json:"email"`
	Name             string       `json:"name"`
	FirstSeen        time.Time    `json:"first_seen"`
	TokenCount       int          `json:"token_count"`
	TransactionCount int          `json:"transaction_count"`
	Providers        []string     `json:"providers"`
	ActiveDsar       *DsarRequest `json:"active_dsar"`
}

// SubProcessorResult is a per-provider erasure outcome.
type SubProcessorResult struct {
	Provider    string     `json:"provider"`
	Status      string     `json:"status"`
	Method      string     `json:"method"`
	ConfirmedAt *time.Time `json:"confirmed_at"`
}

// ErasureResult is returned by POST /api/v1/subjects/{id}/erase.
type ErasureResult struct {
	ErasureID          string               `json:"erasure_id"`
	TokensRevoked      int                  `json:"tokens_revoked"`
	Timestamp          time.Time            `json:"timestamp"`
	CertificateURL     string               `json:"certificate_url"`
	SubProcessorResults []SubProcessorResult `json:"sub_processor_results"`
}

// ── DB Initialisation ─────────────────────────────────────────────────────────

// InitializeDsarTables creates the tables required for DSAR management.
func InitializeDsarTables() error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	ts := "DATETIME"
	if isPostgres {
		ts = "TIMESTAMPTZ"
	}

	stmts := []string{
		// Counter table for auto-generating DSAR-YYYY-NNN references.
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS dsar_sequence (
			year    INTEGER NOT NULL,
			next_seq INTEGER NOT NULL DEFAULT 1,
			PRIMARY KEY (year)
		)`),

		// Main DSAR table.
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS dsars (
			id                     TEXT PRIMARY KEY,
			organization_id        INTEGER NOT NULL,
			reference              TEXT NOT NULL,
			type                   TEXT NOT NULL,
			subject_email          TEXT NOT NULL,
			subject_name           TEXT NOT NULL DEFAULT '',
			status                 TEXT NOT NULL DEFAULT 'pending',
			assigned_to            TEXT NOT NULL DEFAULT '',
			created_at             %s NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deadline_at            %s NOT NULL,
			extended_deadline_at   %s,
			completed_at           %s,
			notes                  TEXT NOT NULL DEFAULT '',
			token_count            INTEGER NOT NULL DEFAULT 0,
			providers_involved     TEXT NOT NULL DEFAULT '[]',
			erasure_certificate_url TEXT NOT NULL DEFAULT ''
		)`, ts, ts, ts, ts),

		// Subject identity graph — maps subject email to first-seen info.
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS subjects (
			id              TEXT PRIMARY KEY,
			organization_id INTEGER NOT NULL,
			email           TEXT NOT NULL,
			name            TEXT NOT NULL DEFAULT '',
			first_seen      %s NOT NULL DEFAULT CURRENT_TIMESTAMP,
			token_count     INTEGER NOT NULL DEFAULT 0,
			UNIQUE(organization_id, email)
		)`, ts),
	}

	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			msg := err.Error()
			if strings.Contains(msg, "already exists") || strings.Contains(msg, "duplicate column") {
				continue
			}
			return fmt.Errorf("InitializeDsarTables: %w", err)
		}
	}
	return nil
}

// ── Reference Generator ───────────────────────────────────────────────────────

// nextDsarReference returns the next DSAR-YYYY-NNN reference for the current year.
// It uses an advisory lock-free sequence table (single-row upsert) which is
// safe for low-to-medium concurrency without true transactions.
func nextDsarReference() (string, error) {
	year := time.Now().Year()

	if isPostgres {
		var seq int
		err := db.QueryRow(
			`INSERT INTO dsar_sequence (year, next_seq) VALUES ($1, 2)
			 ON CONFLICT (year) DO UPDATE SET next_seq = dsar_sequence.next_seq + 1
			 RETURNING next_seq - 1`, year).Scan(&seq)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("DSAR-%d-%03d", year, seq), nil
	}

	// SQLite — no RETURNING, use a two-step approach inside a transaction.
	tx, err := db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback() //nolint:errcheck

	var seq int
	err = tx.QueryRow(`SELECT next_seq FROM dsar_sequence WHERE year = ?`, year).Scan(&seq)
	if err == sql.ErrNoRows {
		seq = 1
		if _, err2 := tx.Exec(`INSERT INTO dsar_sequence (year, next_seq) VALUES (?, ?)`, year, 2); err2 != nil {
			return "", err2
		}
	} else if err != nil {
		return "", err
	} else {
		if _, err2 := tx.Exec(`UPDATE dsar_sequence SET next_seq = next_seq + 1 WHERE year = ?`, year); err2 != nil {
			return "", err2
		}
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return fmt.Sprintf("DSAR-%d-%03d", year, seq), nil
}

// ── DB helpers ────────────────────────────────────────────────────────────────

func insertDsar(d *DsarRequest) error {
	piJSON := marshalDsarJSON(d.ProvidersInvolved)
	_, err := db.Exec(formatQuery(`
		INSERT INTO dsars
			(id, organization_id, reference, type, subject_email, subject_name, status,
			 assigned_to, created_at, deadline_at, extended_deadline_at, completed_at,
			 notes, token_count, providers_involved, erasure_certificate_url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`),
		d.ID, d.OrganizationID, d.Reference, string(d.Type), d.SubjectEmail, d.SubjectName,
		string(d.Status), d.AssignedTo, d.CreatedAt, d.DeadlineAt,
		nullableTime(d.ExtendedDeadlineAt), nullableTime(d.CompletedAt),
		d.Notes, d.TokenCount, piJSON, d.ErasureCertificateURL,
	)
	return err
}

func getDsar(orgID int, id string) (*DsarRequest, error) {
	row := db.QueryRow(formatQuery(`
		SELECT id, organization_id, reference, type, subject_email, subject_name, status,
		       assigned_to, created_at, deadline_at, extended_deadline_at, completed_at,
		       notes, token_count, providers_involved, erasure_certificate_url
		FROM dsars WHERE id = ? AND organization_id = ?
	`), id, orgID)
	return scanDsar(row)
}

func scanDsar(row interface{ Scan(...interface{}) error }) (*DsarRequest, error) {
	var d DsarRequest
	var piRaw string
	var extDeadline, completedAt sql.NullTime
	if err := row.Scan(
		&d.ID, &d.OrganizationID, &d.Reference, (*string)(&d.Type),
		&d.SubjectEmail, &d.SubjectName, (*string)(&d.Status), &d.AssignedTo,
		&d.CreatedAt, &d.DeadlineAt, &extDeadline, &completedAt,
		&d.Notes, &d.TokenCount, &piRaw, &d.ErasureCertificateURL,
	); err != nil {
		return nil, err
	}
	if extDeadline.Valid {
		t := extDeadline.Time
		d.ExtendedDeadlineAt = &t
	}
	if completedAt.Valid {
		t := completedAt.Time
		d.CompletedAt = &t
	}
	json.Unmarshal([]byte(piRaw), &d.ProvidersInvolved) //nolint:errcheck
	if d.ProvidersInvolved == nil {
		d.ProvidersInvolved = []string{}
	}
	return &d, nil
}

func listDsars(orgID, limit, offset int, status, dsarType string) ([]DsarRequest, int, error) {
	where := "organization_id = ?"
	args := []interface{}{orgID}

	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	if dsarType != "" {
		where += " AND type = ?"
		args = append(args, dsarType)
	}

	var total int
	if err := db.QueryRow(formatQuery("SELECT COUNT(*) FROM dsars WHERE "+where), args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, limit, offset)
	rows, err := db.Query(formatQuery(`
		SELECT id, organization_id, reference, type, subject_email, subject_name, status,
		       assigned_to, created_at, deadline_at, extended_deadline_at, completed_at,
		       notes, token_count, providers_involved, erasure_certificate_url
		FROM dsars WHERE `+where+` ORDER BY created_at DESC LIMIT ? OFFSET ?
	`), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []DsarRequest
	for rows.Next() {
		d, err := scanDsar(rows)
		if err != nil {
			continue
		}
		out = append(out, *d)
	}
	if out == nil {
		out = []DsarRequest{}
	}
	return out, total, nil
}

func dsarCounts(orgID int) (open, pending, completed, overdue int, err error) {
	now := time.Now()
	rows, err := db.Query(formatQuery(`
		SELECT status, deadline_at FROM dsars WHERE organization_id = ?
	`), orgID)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var deadline time.Time
		rows.Scan(&status, &deadline) //nolint:errcheck
		switch status {
		case "pending":
			pending++
			open++
		case "in_review", "extended":
			open++
		case "completed", "rejected":
			completed++
		}
		if status != "completed" && status != "rejected" && deadline.Before(now) {
			overdue++
		}
	}
	return
}

func upsertSubject(orgID int, email, name string) (string, error) {
	// Try to get existing subject.
	var id string
	err := db.QueryRow(formatQuery(
		`SELECT id FROM subjects WHERE organization_id = ? AND email = ?`),
		orgID, email).Scan(&id)
	if err == sql.ErrNoRows {
		id = "subj_" + uuid.New().String()
		_, err2 := db.Exec(formatQuery(
			`INSERT INTO subjects (id, organization_id, email, name, first_seen)
			 VALUES (?, ?, ?, ?, ?)`),
			id, orgID, email, name, time.Now())
		return id, err2
	}
	return id, err
}

func getSubjectByEmail(orgID int, email string) (*SubjectProfile, error) {
	var sp SubjectProfile
	err := db.QueryRow(formatQuery(
		`SELECT id, email, name, first_seen, token_count FROM subjects
		 WHERE organization_id = ? AND email = ?`),
		orgID, email).Scan(
		&sp.ID, &sp.Email, &sp.Name, &sp.FirstSeen, &sp.TokenCount)
	if err != nil {
		return nil, err
	}
	sp.Providers = []string{}
	return &sp, nil
}

func getSubjectByID(orgID int, id string) (*SubjectProfile, error) {
	var sp SubjectProfile
	err := db.QueryRow(formatQuery(
		`SELECT id, email, name, first_seen, token_count FROM subjects
		 WHERE organization_id = ? AND id = ?`),
		orgID, id).Scan(
		&sp.ID, &sp.Email, &sp.Name, &sp.FirstSeen, &sp.TokenCount)
	if err != nil {
		return nil, err
	}
	sp.Providers = []string{}
	return &sp, nil
}

func activeDsarForSubject(orgID int, email string) (*DsarRequest, error) {
	row := db.QueryRow(formatQuery(`
		SELECT id, organization_id, reference, type, subject_email, subject_name, status,
		       assigned_to, created_at, deadline_at, extended_deadline_at, completed_at,
		       notes, token_count, providers_involved, erasure_certificate_url
		FROM dsars
		WHERE organization_id = ? AND subject_email = ?
		  AND status NOT IN ('completed', 'rejected')
		ORDER BY created_at DESC LIMIT 1
	`), orgID, email)
	d, err := scanDsar(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return d, err
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// HandleListDsars handles GET /api/v1/dsars
func HandleListDsars(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok || orgID == 0 {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	limit := c.QueryInt("limit", 100)
	offset := c.QueryInt("offset", 0)
	if limit > 500 {
		limit = 500
	}

	dsars, total, err := listDsars(orgID, limit, offset, c.Query("status"), c.Query("type"))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch DSARs"})
	}

	open, pending, completed, overdue, err := dsarCounts(orgID)
	if err != nil {
		open, pending, completed, overdue = 0, 0, 0, 0
	}

	return c.JSON(DsarListResponse{
		Requests:       dsars,
		Total:          total,
		OpenCount:      open,
		PendingCount:   pending,
		CompletedCount: completed,
		OverdueCount:   overdue,
	})
}

// HandleCreateDsar handles POST /api/v1/dsars
func HandleCreateDsar(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok || orgID == 0 {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	var req struct {
		Type         string `json:"type"`
		SubjectEmail string `json:"subject_email"`
		SubjectName  string `json:"subject_name"`
		Notes        string `json:"notes"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	validTypes := map[string]bool{
		"access": true, "erasure": true, "rectification": true,
		"portability": true, "objection": true,
	}
	if !validTypes[req.Type] {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid type; must be access|erasure|rectification|portability|objection"})
	}
	if req.SubjectEmail == "" {
		return c.Status(400).JSON(fiber.Map{"error": "subject_email is required"})
	}

	ref, err := nextDsarReference()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate DSAR reference"})
	}

	now := time.Now().UTC()
	d := &DsarRequest{
		ID:                    "dsar_" + uuid.New().String(),
		OrganizationID:        orgID,
		Reference:             ref,
		Type:                  DsarType(req.Type),
		SubjectEmail:          req.SubjectEmail,
		SubjectName:           req.SubjectName,
		Status:                DsarStatusPending,
		CreatedAt:             now,
		DeadlineAt:            now.AddDate(0, 0, 30),
		Notes:                 req.Notes,
		ProvidersInvolved:     []string{},
		ErasureCertificateURL: "",
	}

	if err := insertDsar(d); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create DSAR"})
	}

	// Ensure subject exists in identity graph.
	upsertSubject(orgID, req.SubjectEmail, req.SubjectName) //nolint:errcheck

	return c.Status(201).JSON(d)
}

// HandleGetDsar handles GET /api/v1/dsars/:id
func HandleGetDsar(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok || orgID == 0 {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	d, err := getDsar(orgID, c.Params("id"))
	if err == sql.ErrNoRows || d == nil {
		return c.Status(404).JSON(fiber.Map{"error": "DSAR not found"})
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch DSAR"})
	}
	return c.JSON(d)
}

// HandleUpdateDsar handles PATCH /api/v1/dsars/:id
func HandleUpdateDsar(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok || orgID == 0 {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	id := c.Params("id")
	existing, err := getDsar(orgID, id)
	if err == sql.ErrNoRows || existing == nil {
		return c.Status(404).JSON(fiber.Map{"error": "DSAR not found"})
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch DSAR"})
	}

	var patch struct {
		Status             *string    `json:"status"`
		AssignedTo         *string    `json:"assigned_to"`
		Notes              *string    `json:"notes"`
		ExtendedDeadlineAt *time.Time `json:"extended_deadline_at"`
	}
	if err := c.BodyParser(&patch); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	validStatuses := map[string]bool{
		"pending": true, "in_review": true, "completed": true, "rejected": true, "extended": true,
	}

	if patch.Status != nil {
		if !validStatuses[*patch.Status] {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid status"})
		}
		existing.Status = DsarStatus(*patch.Status)
		if *patch.Status == "completed" || *patch.Status == "rejected" {
			t := time.Now().UTC()
			existing.CompletedAt = &t
		}
	}
	if patch.AssignedTo != nil {
		existing.AssignedTo = *patch.AssignedTo
	}
	if patch.Notes != nil {
		existing.Notes = *patch.Notes
	}
	if patch.ExtendedDeadlineAt != nil {
		existing.ExtendedDeadlineAt = patch.ExtendedDeadlineAt
		existing.Status = DsarStatusExtended
	}

	piJSON := marshalDsarJSON(existing.ProvidersInvolved)
	_, err = db.Exec(formatQuery(`
		UPDATE dsars
		SET status = ?, assigned_to = ?, notes = ?, extended_deadline_at = ?, completed_at = ?,
		    providers_involved = ?
		WHERE id = ? AND organization_id = ?
	`), string(existing.Status), existing.AssignedTo, existing.Notes,
		nullableTime(existing.ExtendedDeadlineAt), nullableTime(existing.CompletedAt),
		piJSON, id, orgID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update DSAR"})
	}

	return c.JSON(existing)
}

// HandleSubjectLookup handles GET /api/v1/subjects/lookup?email=...
func HandleSubjectLookup(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok || orgID == 0 {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	email := c.Query("email")
	if email == "" {
		return c.Status(400).JSON(fiber.Map{"error": "email query parameter is required"})
	}

	sp, err := getSubjectByEmail(orgID, email)
	if err == sql.ErrNoRows {
		// Subject not found in identity graph but may still submit a DSAR.
		sp = &SubjectProfile{
			ID:        "",
			Email:     email,
			Name:      "",
			FirstSeen: time.Time{},
			Providers: []string{},
		}
	} else if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to lookup subject"})
	}

	// Attach active DSAR if any.
	activeDsar, _ := activeDsarForSubject(orgID, email)
	sp.ActiveDsar = activeDsar

	return c.JSON(sp)
}

// HandleExportSubjectData handles POST /api/v1/subjects/:id/export
func HandleExportSubjectData(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok || orgID == 0 {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	subjectID := c.Params("id")
	sp, err := getSubjectByID(orgID, subjectID)
	if err == sql.ErrNoRows || sp == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Subject not found"})
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch subject"})
	}

	// Generate a short-lived signed URL placeholder.
	// In production this would upload a PDF to object storage.
	downloadURL := fmt.Sprintf("/api/v1/subjects/%s/export/download?token=%s",
		subjectID, uuid.New().String())

	return c.JSON(fiber.Map{"download_url": downloadURL})
}

// HandleEraseSubject handles POST /api/v1/subjects/:id/erase
func HandleEraseSubject(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok || orgID == 0 {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	subjectID := c.Params("id")
	sp, err := getSubjectByID(orgID, subjectID)
	if err == sql.ErrNoRows || sp == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Subject not found"})
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch subject"})
	}

	now := time.Now().UTC()
	erasureID := "erasure_" + uuid.New().String()

	// Revoke tokens: mark token_count as 0, update subject record.
	tokensRevoked := sp.TokenCount
	db.Exec(formatQuery(`UPDATE subjects SET token_count = 0 WHERE id = ? AND organization_id = ?`), //nolint:errcheck
		subjectID, orgID)

	// Build sub-processor results based on DSARs for this subject.
	results := buildSubProcessorResults(orgID, sp.Email, now)

	// Generate certificate URL placeholder.
	certURL := fmt.Sprintf("/api/v1/erasure-certificates/%s?org=%d", erasureID, orgID)

	// Mark any active DSARs as completed.
	db.Exec(formatQuery(`
		UPDATE dsars SET status = 'completed', completed_at = ?,
		    erasure_certificate_url = ?
		WHERE organization_id = ? AND subject_email = ?
		  AND status NOT IN ('completed', 'rejected')
	`), now, certURL, orgID, sp.Email) //nolint:errcheck

	return c.JSON(ErasureResult{
		ErasureID:           erasureID,
		TokensRevoked:       tokensRevoked,
		Timestamp:           now,
		CertificateURL:      certURL,
		SubProcessorResults: results,
	})
}

// buildSubProcessorResults derives the provider-level erasure outcomes from the
// providers_involved column of any DSAR linked to the subject.
func buildSubProcessorResults(orgID int, email string, now time.Time) []SubProcessorResult {
	rows, err := db.Query(formatQuery(`
		SELECT providers_involved FROM dsars
		WHERE organization_id = ? AND subject_email = ?
	`), orgID, email)
	if err != nil {
		return []SubProcessorResult{}
	}
	defer rows.Close()

	seen := map[string]bool{}
	var results []SubProcessorResult

	for rows.Next() {
		var piRaw string
		rows.Scan(&piRaw) //nolint:errcheck
		var providers []string
		json.Unmarshal([]byte(piRaw), &providers) //nolint:errcheck
		for _, p := range providers {
			if seen[p] {
				continue
			}
			seen[p] = true
			t := now
			results = append(results, SubProcessorResult{
				Provider:    p,
				Status:      "not_applicable",
				Method:      "tokenized",
				ConfirmedAt: &t,
			})
		}
	}
	if results == nil {
		results = []SubProcessorResult{}
	}
	return results
}

// ── Utility ───────────────────────────────────────────────────────────────────

// marshalDsarJSON marshals a value to JSON string, returning "[]" on error.
func marshalDsarJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// nullableTime converts a *time.Time to a value safe for database storage.
func nullableTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return *t
}
