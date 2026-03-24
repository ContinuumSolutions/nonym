package scanner

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	db         *sql.DB
	isPostgres bool
)

// Initialize wires the scanner package to the shared database connection.
// Call this after audit.Initialize() and pass audit.GetDatabase().
// Tables are created automatically for both PostgreSQL and SQLite.
func Initialize(sharedDB *sql.DB, postgres bool) error {
	db = sharedDB
	isPostgres = postgres
	return createTables()
}

// formatQuery converts ? placeholders to $1, $2, … for PostgreSQL.
func formatQuery(query string) string {
	if !isPostgres {
		return query
	}
	count := 1
	result := ""
	for _, ch := range query {
		if ch == '?' {
			result += fmt.Sprintf("$%d", count)
			count++
		} else {
			result += string(ch)
		}
	}
	return result
}

func createTables() error {
	ts := "DATETIME"
	if isPostgres {
		ts = "TIMESTAMPTZ"
	}
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS vendor_connections (
			id            TEXT PRIMARY KEY,
			org_id        INTEGER NOT NULL,
			vendor        TEXT NOT NULL,
			display_name  TEXT NOT NULL DEFAULT '',
			status        TEXT NOT NULL DEFAULT 'disconnected',
			scan_status   TEXT NOT NULL DEFAULT 'idle',
			auth_type     TEXT NOT NULL DEFAULT 'api_key',
			credentials   TEXT NOT NULL DEFAULT '{}',
			settings      TEXT NOT NULL DEFAULT '{}',
			connected_at  %s,
			last_scan_at  %s,
			error_message TEXT,
			created_at    %s NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at    %s NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, vendor)
		)`, ts, ts, ts, ts),
		// Migration: add scan_status to existing tables (ignore error if column already exists).
		`ALTER TABLE vendor_connections ADD COLUMN scan_status TEXT NOT NULL DEFAULT 'idle'`,
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS scans (
			id             TEXT PRIMARY KEY,
			org_id         INTEGER NOT NULL,
			vendor_ids     TEXT NOT NULL DEFAULT '[]',
			status         TEXT NOT NULL DEFAULT 'pending',
			started_at     %s,
			completed_at   %s,
			findings_count INTEGER NOT NULL DEFAULT 0,
			error_message  TEXT,
			triggered_by   TEXT NOT NULL DEFAULT 'manual',
			created_at     %s NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`, ts, ts, ts),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS findings (
			id                   TEXT PRIMARY KEY,
			org_id               INTEGER NOT NULL,
			scan_id              TEXT NOT NULL,
			vendor_connection_id TEXT NOT NULL,
			vendor               TEXT NOT NULL,
			data_type            TEXT NOT NULL,
			risk_level           TEXT NOT NULL,
			title                TEXT NOT NULL,
			description          TEXT NOT NULL,
			location             TEXT,
			endpoint             TEXT,
			occurrences          INTEGER NOT NULL DEFAULT 1,
			sample_masked        TEXT,
			status               TEXT NOT NULL DEFAULT 'open',
			compliance_impact    TEXT NOT NULL DEFAULT '[]',
			fixes                TEXT NOT NULL DEFAULT '[]',
			first_seen_at        %s NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_seen_at         %s NOT NULL DEFAULT CURRENT_TIMESTAMP,
			resolved_at          %s,
			resolved_by          INTEGER,
			created_at           %s NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`, ts, ts, ts, ts),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS reports (
			id           TEXT PRIMARY KEY,
			org_id       INTEGER NOT NULL,
			framework    TEXT NOT NULL,
			time_range   TEXT NOT NULL,
			options      TEXT NOT NULL DEFAULT '{}',
			status       TEXT NOT NULL DEFAULT 'pending',
			file_url     TEXT,
			share_token  TEXT UNIQUE,
			generated_at %s,
			expires_at   %s,
			created_at   %s NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`, ts, ts, ts),
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			// Ignore "column already exists" errors from ALTER TABLE migrations.
			msg := err.Error()
			if strings.Contains(msg, "duplicate column") || strings.Contains(msg, "already exists") || strings.Contains(msg, "duplicate column name") {
				continue
			}
			return fmt.Errorf("scanner createTables: %w", err)
		}
	}
	return nil
}


// newID generates a UUID string.
func newID() string {
	return uuid.New().String()
}

// ── vendor_connections ────────────────────────────────────────────────────────

func insertVendorConnection(vc *VendorConnection) error {
	credJSON := marshalJSON(vc.Credentials)
	settJSON := marshalJSON(vc.Settings)
	if vc.ScanStatus == "" {
		vc.ScanStatus = "idle"
	}
	_, err := db.Exec(formatQuery(`
		INSERT INTO vendor_connections
			(id, org_id, vendor, display_name, status, scan_status, auth_type, credentials, settings, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(org_id, vendor) DO UPDATE SET
			display_name=EXCLUDED.display_name,
			status=EXCLUDED.status,
			scan_status=EXCLUDED.scan_status,
			auth_type=EXCLUDED.auth_type,
			credentials=EXCLUDED.credentials,
			settings=EXCLUDED.settings,
			updated_at=EXCLUDED.updated_at
	`),
		vc.ID, vc.OrgID, vc.Vendor, vc.DisplayName, vc.Status, vc.ScanStatus,
		vc.AuthType, credJSON, settJSON, vc.CreatedAt, vc.UpdatedAt)
	return err
}

const vcSelectCols = `id, org_id, vendor, display_name, status, scan_status, auth_type, credentials, settings,
	connected_at, last_scan_at, error_message, created_at, updated_at`

// scanVendorConnection reads one VendorConnection from a sql.Row.
func scanVendorConnection(row interface {
	Scan(...interface{}) error
}) (VendorConnection, error) {
	var vc VendorConnection
	var credRaw, settRaw string
	var connectedAt, lastScanAt sql.NullTime
	var errMsg sql.NullString
	if err := row.Scan(
		&vc.ID, &vc.OrgID, &vc.Vendor, &vc.DisplayName, &vc.Status, &vc.ScanStatus,
		&vc.AuthType, &credRaw, &settRaw,
		&connectedAt, &lastScanAt, &errMsg,
		&vc.CreatedAt, &vc.UpdatedAt,
	); err != nil {
		return vc, err
	}
	if err := json.Unmarshal([]byte(credRaw), &vc.Credentials); err != nil {
		log.Printf("scanner: scanVendorConnection credentials unmarshal error (id=%s): %v", vc.ID, err)
	}
	if err := json.Unmarshal([]byte(settRaw), &vc.Settings); err != nil {
		log.Printf("scanner: scanVendorConnection settings unmarshal error (id=%s): %v", vc.ID, err)
	}
	if connectedAt.Valid {
		t := connectedAt.Time
		vc.ConnectedAt = &t
	}
	if lastScanAt.Valid {
		t := lastScanAt.Time
		vc.LastScanAt = &t
	}
	vc.ErrorMessage = errMsg.String
	return vc, nil
}

func listVendorConnections(orgID int, statusFilter string) ([]VendorConnection, error) {
	q := `SELECT ` + vcSelectCols + ` FROM vendor_connections WHERE org_id = ?`
	args := []interface{}{orgID}
	if statusFilter != "" {
		q += " AND status = ?"
		args = append(args, statusFilter)
	}
	q += " ORDER BY created_at DESC"

	rows, err := db.Query(formatQuery(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []VendorConnection
	for rows.Next() {
		vc, err := scanVendorConnection(rows)
		if err != nil {
			log.Printf("scanner: listVendorConnections scan error: %v", err)
			continue
		}
		out = append(out, vc)
	}
	if out == nil {
		out = []VendorConnection{}
	}
	return out, nil
}

func getVendorConnection(orgID int, id string) (*VendorConnection, error) {
	row := db.QueryRow(formatQuery(
		`SELECT `+vcSelectCols+` FROM vendor_connections WHERE id = ? AND org_id = ?`,
	), id, orgID)
	vc, err := scanVendorConnection(row)
	if err != nil {
		return nil, err
	}
	return &vc, nil
}

// getVendorConnectionByVendor returns the first connection for the given org and vendor slug.
func getVendorConnectionByVendor(orgID int, vendor string) (*VendorConnection, error) {
	row := db.QueryRow(formatQuery(
		`SELECT `+vcSelectCols+` FROM vendor_connections WHERE org_id = ? AND vendor = ? LIMIT 1`,
	), orgID, vendor)
	vc, err := scanVendorConnection(row)
	if err != nil {
		return nil, err
	}
	return &vc, nil
}

func deleteVendorConnection(orgID int, id string) error {
	_, err := db.Exec(formatQuery(
		`DELETE FROM vendor_connections WHERE id = ? AND org_id = ?`), id, orgID)
	return err
}

func updateVendorConnectionStatus(id, status, errMsg string, connectedAt, lastScanAt *time.Time) error {
	if db == nil {
		return nil
	}
	_, err := db.Exec(formatQuery(`
		UPDATE vendor_connections
		SET status = ?, error_message = ?, connected_at = ?, last_scan_at = ?, updated_at = ?
		WHERE id = ?
	`), status, errMsg, connectedAt, lastScanAt, time.Now(), id)
	return err
}

func updateVendorScanStatus(id, scanStatus string) error {
	if db == nil {
		return nil
	}
	_, err := db.Exec(formatQuery(`
		UPDATE vendor_connections SET scan_status = ?, updated_at = ? WHERE id = ?
	`), scanStatus, time.Now(), id)
	return err
}


// ── scans ─────────────────────────────────────────────────────────────────────

func insertScan(s *Scan) error {
	vidsJSON := marshalJSON(s.VendorIDs)
	_, err := db.Exec(formatQuery(`
		INSERT INTO scans (id, org_id, vendor_ids, status, triggered_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`), s.ID, s.OrgID, vidsJSON, s.Status, s.TriggeredBy, s.CreatedAt)
	return err
}

func listScans(orgID, limit, offset int) ([]Scan, error) {
	rows, err := db.Query(formatQuery(`
		SELECT id, org_id, vendor_ids, status, started_at, completed_at,
		       findings_count, error_message, triggered_by, created_at
		FROM scans WHERE org_id = ?
		ORDER BY created_at DESC LIMIT ? OFFSET ?
	`), orgID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

func getScan(orgID int, id string) (*Scan, error) {
	rows, err := db.Query(formatQuery(`
		SELECT id, org_id, vendor_ids, status, started_at, completed_at,
		       findings_count, error_message, triggered_by, created_at
		FROM scans WHERE id = ? AND org_id = ?
	`), id, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	scans, err := scanRows(rows)
	if err != nil || len(scans) == 0 {
		return nil, fmt.Errorf("scan not found")
	}
	return &scans[0], nil
}

func scanRows(rows *sql.Rows) ([]Scan, error) {
	var out []Scan
	for rows.Next() {
		var s Scan
		var vidsRaw string
		var startedAt, completedAt sql.NullTime
		var errMsg sql.NullString
		if err := rows.Scan(
			&s.ID, &s.OrgID, &vidsRaw, &s.Status,
			&startedAt, &completedAt,
			&s.FindingsCount, &errMsg, &s.TriggeredBy, &s.CreatedAt,
		); err != nil {
			log.Printf("scanner: scanRows scan error: %v", err)
			continue
		}
		if err := json.Unmarshal([]byte(vidsRaw), &s.VendorIDs); err != nil {
			log.Printf("scanner: scanRows vendor_ids unmarshal error (id=%s): %v", s.ID, err)
		}
		if s.VendorIDs == nil {
			s.VendorIDs = []string{}
		}
		if startedAt.Valid {
			t := startedAt.Time
			s.StartedAt = &t
		}
		if completedAt.Valid {
			t := completedAt.Time
			s.CompletedAt = &t
		}
		s.ErrorMessage = errMsg.String
		out = append(out, s)
	}
	if out == nil {
		out = []Scan{}
	}
	return out, nil
}

func updateScanStatus(id, status string, findingsCount int, startedAt, completedAt *time.Time, errMsg string) error {
	if db == nil {
		return nil
	}
	_, err := db.Exec(formatQuery(`
		UPDATE scans
		SET status = ?, findings_count = ?, started_at = ?, completed_at = ?, error_message = ?
		WHERE id = ?
	`), status, findingsCount, startedAt, completedAt, errMsg, id)
	return err
}

// ── findings ──────────────────────────────────────────────────────────────────

type FindingFilter struct {
	OrgID     int
	Vendor    string
	RiskLevel string
	DataType  string
	Status    string
	Since     *time.Time // only return findings last seen at or after this time
	Limit     int
	Offset    int
}

func insertFinding(f *Finding) error {
	if db == nil {
		return nil
	}
	ciJSON := marshalJSON(f.ComplianceImpact)
	fixJSON := marshalJSON(f.Fixes)
	_, err := db.Exec(formatQuery(`
		INSERT INTO findings
			(id, org_id, scan_id, vendor_connection_id, vendor, data_type, risk_level,
			 title, description, location, endpoint, occurrences, sample_masked, status,
			 compliance_impact, fixes, first_seen_at, last_seen_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`),
		f.ID, f.OrgID, f.ScanID, f.VendorConnectionID, f.Vendor, f.DataType, f.RiskLevel,
		f.Title, f.Description, f.Location, f.Endpoint, f.Occurrences, f.SampleMasked, f.Status,
		ciJSON, fixJSON, f.FirstSeenAt, f.LastSeenAt, f.CreatedAt)
	return err
}

// deduplicateFinding increments occurrences if an identical open finding exists;
// returns the existing finding ID if deduped, empty string if new.
func deduplicateFinding(orgID int, vendor, dataType, location, endpoint string) (string, error) {
	var id string
	err := db.QueryRow(formatQuery(`
		SELECT id FROM findings
		WHERE org_id = ? AND vendor = ? AND data_type = ? AND location = ? AND endpoint = ? AND status = 'open'
		LIMIT 1
	`), orgID, vendor, dataType, location, endpoint).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	now := time.Now()
	_, err = db.Exec(formatQuery(`
		UPDATE findings SET occurrences = occurrences + 1, last_seen_at = ? WHERE id = ?
	`), now, id)
	return id, err
}

func listFindings(f FindingFilter) ([]Finding, error) {
	q := `SELECT id, org_id, scan_id, vendor_connection_id, vendor, data_type, risk_level,
	             title, description, location, endpoint, occurrences, sample_masked, status,
	             compliance_impact, fixes, first_seen_at, last_seen_at, resolved_at, resolved_by, created_at
	      FROM findings WHERE org_id = ?`
	args := []interface{}{f.OrgID}
	if f.Vendor != "" {
		q += " AND vendor = ?"
		args = append(args, f.Vendor)
	}
	if f.RiskLevel != "" {
		q += " AND risk_level = ?"
		args = append(args, f.RiskLevel)
	}
	if f.DataType != "" {
		q += " AND data_type = ?"
		args = append(args, f.DataType)
	}
	if f.Status != "" {
		q += " AND status = ?"
		args = append(args, f.Status)
	}
	if f.Since != nil {
		q += " AND last_seen_at >= ?"
		args = append(args, *f.Since)
	}
	q += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, f.Limit, f.Offset)

	rows, err := db.Query(formatQuery(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return findingRows(rows)
}

func getFinding(orgID int, id string) (*Finding, error) {
	rows, err := db.Query(formatQuery(`
		SELECT id, org_id, scan_id, vendor_connection_id, vendor, data_type, risk_level,
		       title, description, location, endpoint, occurrences, sample_masked, status,
		       compliance_impact, fixes, first_seen_at, last_seen_at, resolved_at, resolved_by, created_at
		FROM findings WHERE id = ? AND org_id = ?
	`), id, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	findings, err := findingRows(rows)
	if err != nil || len(findings) == 0 {
		return nil, fmt.Errorf("finding not found")
	}
	return &findings[0], nil
}

func findingRows(rows *sql.Rows) ([]Finding, error) {
	var out []Finding
	for rows.Next() {
		var f Finding
		var ciRaw, fixRaw string
		var resolvedAt sql.NullTime
		var resolvedBy sql.NullInt64
		if err := rows.Scan(
			&f.ID, &f.OrgID, &f.ScanID, &f.VendorConnectionID, &f.Vendor,
			&f.DataType, &f.RiskLevel, &f.Title, &f.Description,
			&f.Location, &f.Endpoint, &f.Occurrences, &f.SampleMasked, &f.Status,
			&ciRaw, &fixRaw, &f.FirstSeenAt, &f.LastSeenAt,
			&resolvedAt, &resolvedBy, &f.CreatedAt,
		); err != nil {
			continue
		}
		json.Unmarshal([]byte(ciRaw), &f.ComplianceImpact)
		json.Unmarshal([]byte(fixRaw), &f.Fixes)
		if f.ComplianceImpact == nil {
			f.ComplianceImpact = []ComplianceImpact{}
		}
		if f.Fixes == nil {
			f.Fixes = []Fix{}
		}
		if resolvedAt.Valid {
			t := resolvedAt.Time
			f.ResolvedAt = &t
		}
		if resolvedBy.Valid {
			v := int(resolvedBy.Int64)
			f.ResolvedBy = &v
		}
		out = append(out, f)
	}
	if out == nil {
		out = []Finding{}
	}
	return out, nil
}

func patchFinding(orgID int, id, status string, resolvedBy *int) error {
	var resolvedAt interface{}
	var resolvedByVal interface{}
	if status == "resolved" {
		t := time.Now()
		resolvedAt = t
		if resolvedBy != nil {
			resolvedByVal = *resolvedBy
		}
	}
	_, err := db.Exec(formatQuery(`
		UPDATE findings SET status = ?, resolved_at = ?, resolved_by = ?
		WHERE id = ? AND org_id = ?
	`), status, resolvedAt, resolvedByVal, id, orgID)
	return err
}

// findingCounts returns high/medium/low/total counts of open findings.
func findingCounts(orgID int) (FindingCounts, error) {
	var fc FindingCounts
	rows, err := db.Query(formatQuery(`
		SELECT risk_level, COUNT(*) FROM findings
		WHERE org_id = ? AND status = 'open'
		GROUP BY risk_level
	`), orgID)
	if err != nil {
		return fc, err
	}
	defer rows.Close()
	for rows.Next() {
		var level string
		var count int
		rows.Scan(&level, &count)
		switch level {
		case "high":
			fc.High = count
		case "medium":
			fc.Medium = count
		case "low":
			fc.Low = count
		}
		fc.Total += count
	}
	return fc, nil
}

// getOrgName returns the organisation's display name for the given ID.
// Returns an empty string on any error so callers can degrade gracefully.
func getOrgName(orgID int) string {
	if db == nil {
		return ""
	}
	var name string
	if err := db.QueryRow(
		formatQuery(`SELECT name FROM organizations WHERE id = ?`), orgID,
	).Scan(&name); err != nil {
		return ""
	}
	return name
}

// ── reports ───────────────────────────────────────────────────────────────────

func insertReport(r *Report) error {
	optJSON := marshalJSON(r.Options)
	_, err := db.Exec(formatQuery(`
		INSERT INTO reports (id, org_id, framework, time_range, options, status, share_token, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`), r.ID, r.OrgID, r.Framework, r.TimeRange, optJSON, r.Status, r.ShareToken, r.ExpiresAt, r.CreatedAt)
	return err
}

func listReports(orgID int) ([]Report, error) {
	rows, err := db.Query(formatQuery(`
		SELECT id, org_id, framework, time_range, options, status, file_url, share_token,
		       generated_at, expires_at, created_at
		FROM reports WHERE org_id = ?
		ORDER BY created_at DESC
	`), orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return reportRows(rows)
}

func getReport(orgID int, id string) (*Report, error) {
	rows, err := db.Query(formatQuery(`
		SELECT id, org_id, framework, time_range, options, status, file_url, share_token,
		       generated_at, expires_at, created_at
		FROM reports WHERE id = ? AND org_id = ?
	`), id, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reports, err := reportRows(rows)
	if err != nil || len(reports) == 0 {
		return nil, fmt.Errorf("report not found")
	}
	return &reports[0], nil
}

func getReportByShareToken(token string) (*Report, error) {
	rows, err := db.Query(formatQuery(`
		SELECT id, org_id, framework, time_range, options, status, file_url, share_token,
		       generated_at, expires_at, created_at
		FROM reports WHERE share_token = ?
	`), token)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reports, err := reportRows(rows)
	if err != nil || len(reports) == 0 {
		return nil, fmt.Errorf("report not found")
	}
	return &reports[0], nil
}

func reportRows(rows *sql.Rows) ([]Report, error) {
	var out []Report
	for rows.Next() {
		var r Report
		var optRaw string
		var fileURL, shareToken sql.NullString
		var generatedAt, expiresAt sql.NullTime
		if err := rows.Scan(
			&r.ID, &r.OrgID, &r.Framework, &r.TimeRange, &optRaw,
			&r.Status, &fileURL, &shareToken,
			&generatedAt, &expiresAt, &r.CreatedAt,
		); err != nil {
			continue
		}
		json.Unmarshal([]byte(optRaw), &r.Options)
		r.FileURL = fileURL.String
		r.ShareToken = shareToken.String
		if generatedAt.Valid {
			t := generatedAt.Time
			r.GeneratedAt = &t
		}
		if expiresAt.Valid {
			t := expiresAt.Time
			r.ExpiresAt = &t
		}
		out = append(out, r)
	}
	if out == nil {
		out = []Report{}
	}
	return out, nil
}

func updateReport(id, status, fileURL, shareToken string, generatedAt, expiresAt *time.Time) error {
	if db == nil {
		return nil
	}
	_, err := db.Exec(formatQuery(`
		UPDATE reports
		SET status = ?, file_url = ?, share_token = ?, generated_at = ?, expires_at = ?
		WHERE id = ?
	`), status, fileURL, shareToken, generatedAt, expiresAt, id)
	return err
}
