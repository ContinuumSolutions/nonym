package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sovereignprivacy/gateway/pkg/auth/errors"
	"github.com/sovereignprivacy/gateway/pkg/auth/interfaces"
	"github.com/sovereignprivacy/gateway/pkg/auth/models"
)

// Organization operations continued...

func (r *authRepository) UpdateOrganization(ctx context.Context, id uuid.UUID, updates *models.UpdateOrganizationRequest) (*models.Organization, error) {
	setParts := []string{"updated_at = CURRENT_TIMESTAMP"}
	args := []interface{}{}

	if updates.Name != nil {
		setParts = append(setParts, "name = ?")
		args = append(args, *updates.Name)
	}
	if updates.Description != nil {
		setParts = append(setParts, "description = ?")
		args = append(args, *updates.Description)
	}
	if updates.Settings != nil {
		setParts = append(setParts, "settings = ?")
		if r.isPostgres {
			args = append(args, toJSONB(*updates.Settings))
		} else {
			args = append(args, toJSON(*updates.Settings))
		}
	}
	if updates.IsActive != nil {
		setParts = append(setParts, "is_active = ?")
		args = append(args, *updates.IsActive)
	}

	if len(args) == 0 {
		return r.GetOrganizationByID(ctx, id)
	}

	args = append(args, id)
	query := r.formatQuery(fmt.Sprintf("UPDATE organizations SET %s WHERE id = ?", joinString(setParts, ", ")))

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to update organization: %w", err)
	}

	return r.GetOrganizationByID(ctx, id)
}

func (r *authRepository) DeleteOrganization(ctx context.Context, id uuid.UUID) error {
	query := r.formatQuery("UPDATE organizations SET is_active = false, updated_at = CURRENT_TIMESTAMP WHERE id = ?")
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete organization: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return errors.ErrOrgNotFound
	}

	return nil
}

func (r *authRepository) ListOrganizations(ctx context.Context, limit, offset int) ([]*models.Organization, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	// Get total count
	countQuery := r.formatQuery("SELECT COUNT(*) FROM organizations WHERE is_active = true")
	var total int
	err := r.db.GetContext(ctx, &total, countQuery)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get organization count: %w", err)
	}

	// Get organizations
	query := r.formatQuery(`
		SELECT id, name, slug, description, settings, is_active, created_at, updated_at
		FROM organizations
		WHERE is_active = true
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`)

	rows, err := r.db.QueryxContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list organizations: %w", err)
	}
	defer rows.Close()

	var orgs []*models.Organization
	for rows.Next() {
		var org models.Organization
		var settingsJSON string

		err := rows.Scan(
			&org.ID,
			&org.Name,
			&org.Slug,
			&org.Description,
			&settingsJSON,
			&org.IsActive,
			&org.CreatedAt,
			&org.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan organization: %w", err)
		}

		// Parse settings JSON
		if err := fromJSON(settingsJSON, &org.Settings); err != nil {
			return nil, 0, fmt.Errorf("failed to parse organization settings: %w", err)
		}

		orgs = append(orgs, &org)
	}

	return orgs, total, nil
}

func (r *authRepository) GetOrganizationMembers(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*models.User, int, error) {
	return r.ListUsers(ctx, orgID, limit, offset)
}

func (r *authRepository) GetOrganizationStats(ctx context.Context, orgID uuid.UUID) (*models.OrganizationStats, error) {
	stats := &models.OrganizationStats{}

	// Get user counts
	userStatsQuery := r.formatQuery(`
		SELECT
			COUNT(*) as total_users,
			COUNT(CASE WHEN is_active = true THEN 1 END) as active_users,
			COUNT(CASE WHEN role IN ('admin', 'owner') THEN 1 END) as admin_users,
			COUNT(CASE WHEN email_verified = false THEN 1 END) as pending_users
		FROM users
		WHERE organization_id = ?
	`)

	err := r.db.QueryRowxContext(ctx, userStatsQuery, orgID).Scan(
		&stats.TotalUsers,
		&stats.ActiveUsers,
		&stats.AdminUsers,
		&stats.PendingUsers,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get user stats: %w", err)
	}

	// Get login events (last 24 hours)
	loginStatsQuery := r.formatQuery(`
		SELECT
			COUNT(CASE WHEN success = true AND type = 'login' THEN 1 END) as login_events,
			COUNT(CASE WHEN success = false AND type = 'login' THEN 1 END) as failed_logins
		FROM auth_events
		WHERE organization_id = ? AND created_at >= ?
	`)

	since := time.Now().Add(-24 * time.Hour)
	err = r.db.QueryRowxContext(ctx, loginStatsQuery, orgID, since).Scan(
		&stats.LoginEvents,
		&stats.FailedLogins,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get login stats: %w", err)
	}

	return stats, nil
}

// Session operations

func (r *authRepository) CreateSession(ctx context.Context, session *interfaces.UserSession) error {
	query := r.formatQuery(`
		INSERT INTO user_sessions (id, user_id, organization_id, session_token, ip_address, user_agent, expires_at, is_active)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)

	_, err := r.db.ExecContext(ctx, query,
		session.ID,
		session.UserID,
		session.OrganizationID,
		session.SessionToken,
		session.IPAddress,
		session.UserAgent,
		session.ExpiresAt,
		session.IsActive,
	)

	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

func (r *authRepository) GetSession(ctx context.Context, sessionID uuid.UUID) (*interfaces.UserSession, error) {
	var session interfaces.UserSession
	query := r.formatQuery(`
		SELECT id, user_id, organization_id, session_token, ip_address, user_agent,
		       expires_at, created_at, last_accessed_at, is_active
		FROM user_sessions
		WHERE id = ? AND is_active = true AND expires_at > CURRENT_TIMESTAMP
	`)

	err := r.db.GetContext(ctx, &session, query, sessionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NewAuthError(errors.ErrCodeInvalidToken, "Session not found")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return &session, nil
}

func (r *authRepository) DeleteSession(ctx context.Context, sessionID uuid.UUID) error {
	query := r.formatQuery("UPDATE user_sessions SET is_active = false WHERE id = ?")
	_, err := r.db.ExecContext(ctx, query, sessionID)
	return err
}

func (r *authRepository) DeleteUserSessions(ctx context.Context, userID uuid.UUID) error {
	query := r.formatQuery("UPDATE user_sessions SET is_active = false WHERE user_id = ?")
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}

func (r *authRepository) DeleteExpiredSessions(ctx context.Context) (int, error) {
	query := r.formatQuery("DELETE FROM user_sessions WHERE expires_at <= CURRENT_TIMESTAMP OR is_active = false")
	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(rowsAffected), nil
}

func (r *authRepository) UpdateSessionActivity(ctx context.Context, sessionID uuid.UUID) error {
	query := r.formatQuery("UPDATE user_sessions SET last_accessed_at = CURRENT_TIMESTAMP WHERE id = ?")
	_, err := r.db.ExecContext(ctx, query, sessionID)
	return err
}

// Token operations

func (r *authRepository) CreateRefreshToken(ctx context.Context, token *interfaces.RefreshToken) error {
	query := r.formatQuery(`
		INSERT INTO refresh_tokens (id, user_id, organization_id, token_hash, expires_at, ip_address, user_agent)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)

	_, err := r.db.ExecContext(ctx, query,
		token.ID,
		token.UserID,
		token.OrganizationID,
		token.TokenHash,
		token.ExpiresAt,
		token.IPAddress,
		token.UserAgent,
	)

	if err != nil {
		return fmt.Errorf("failed to create refresh token: %w", err)
	}

	return nil
}

func (r *authRepository) GetRefreshToken(ctx context.Context, tokenHash string) (*interfaces.RefreshToken, error) {
	var token interfaces.RefreshToken
	query := r.formatQuery(`
		SELECT id, user_id, organization_id, token_hash, expires_at, created_at,
		       last_used_at, ip_address, user_agent, is_revoked
		FROM refresh_tokens
		WHERE token_hash = ? AND is_revoked = false AND expires_at > CURRENT_TIMESTAMP
	`)

	err := r.db.GetContext(ctx, &token, query, tokenHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NewAuthError(errors.ErrCodeRefreshTokenInvalid, "Refresh token not found or expired")
		}
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}

	return &token, nil
}

func (r *authRepository) DeleteRefreshToken(ctx context.Context, tokenHash string) error {
	query := r.formatQuery("UPDATE refresh_tokens SET is_revoked = true WHERE token_hash = ?")
	_, err := r.db.ExecContext(ctx, query, tokenHash)
	return err
}

func (r *authRepository) DeleteUserRefreshTokens(ctx context.Context, userID uuid.UUID) error {
	query := r.formatQuery("UPDATE refresh_tokens SET is_revoked = true WHERE user_id = ?")
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}

func (r *authRepository) DeleteExpiredRefreshTokens(ctx context.Context) (int, error) {
	query := r.formatQuery("DELETE FROM refresh_tokens WHERE expires_at <= CURRENT_TIMESTAMP OR is_revoked = true")
	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(rowsAffected), nil
}

// Audit operations

func (r *authRepository) LogAuthEvent(ctx context.Context, event *interfaces.AuthEvent) error {
	query := r.formatQuery(`
		INSERT INTO auth_events (id, type, user_id, organization_id, ip_address, user_agent, success, error_reason, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)

	_, err := r.db.ExecContext(ctx, query,
		event.ID,
		event.Type,
		event.UserID,
		event.OrganizationID,
		event.IPAddress,
		event.UserAgent,
		event.Success,
		event.ErrorReason,
		event.Metadata,
	)

	if err != nil {
		return fmt.Errorf("failed to log auth event: %w", err)
	}

	return nil
}

func (r *authRepository) GetAuthEvents(ctx context.Context, filter *interfaces.AuthEventFilter) ([]*interfaces.AuthEvent, int, error) {
	whereParts := []string{"1=1"}
	args := []interface{}{}

	if filter.UserID != nil {
		whereParts = append(whereParts, "user_id = ?")
		args = append(args, *filter.UserID)
	}
	if filter.OrganizationID != nil {
		whereParts = append(whereParts, "organization_id = ?")
		args = append(args, *filter.OrganizationID)
	}
	if filter.Type != "" {
		whereParts = append(whereParts, "type = ?")
		args = append(args, filter.Type)
	}
	if filter.Success != nil {
		whereParts = append(whereParts, "success = ?")
		args = append(args, *filter.Success)
	}
	if filter.IPAddress != "" {
		whereParts = append(whereParts, "ip_address = ?")
		args = append(args, filter.IPAddress)
	}
	if filter.StartDate != nil {
		whereParts = append(whereParts, "created_at >= ?")
		args = append(args, *filter.StartDate)
	}
	if filter.EndDate != nil {
		whereParts = append(whereParts, "created_at <= ?")
		args = append(args, *filter.EndDate)
	}

	whereClause := joinString(whereParts, " AND ")

	// Get total count
	countQuery := r.formatQuery(fmt.Sprintf("SELECT COUNT(*) FROM auth_events WHERE %s", whereClause))
	var total int
	err := r.db.GetContext(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get auth events count: %w", err)
	}

	// Get events
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	query := r.formatQuery(fmt.Sprintf(`
		SELECT id, type, user_id, organization_id, ip_address, user_agent, success, error_reason, metadata, created_at
		FROM auth_events
		WHERE %s
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, whereClause))

	args = append(args, limit, offset)

	var events []*interfaces.AuthEvent
	err = r.db.SelectContext(ctx, &events, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get auth events: %w", err)
	}

	return events, total, nil
}

// Password reset operations

func (r *authRepository) CreatePasswordReset(ctx context.Context, reset *interfaces.PasswordReset) error {
	query := r.formatQuery(`
		INSERT INTO password_resets (id, user_id, token, expires_at, ip_address)
		VALUES (?, ?, ?, ?, ?)
	`)

	_, err := r.db.ExecContext(ctx, query,
		reset.ID,
		reset.UserID,
		reset.Token,
		reset.ExpiresAt,
		reset.IPAddress,
	)

	if err != nil {
		return fmt.Errorf("failed to create password reset: %w", err)
	}

	return nil
}

func (r *authRepository) GetPasswordReset(ctx context.Context, token string) (*interfaces.PasswordReset, error) {
	var reset interfaces.PasswordReset
	query := r.formatQuery(`
		SELECT id, user_id, token, expires_at, created_at, used_at, ip_address
		FROM password_resets
		WHERE token = ? AND expires_at > CURRENT_TIMESTAMP AND used_at IS NULL
	`)

	err := r.db.GetContext(ctx, &reset, query, token)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NewAuthError(errors.ErrCodeInvalidToken, "Password reset token not found or expired")
		}
		return nil, fmt.Errorf("failed to get password reset: %w", err)
	}

	return &reset, nil
}

func (r *authRepository) DeletePasswordReset(ctx context.Context, token string) error {
	query := r.formatQuery("UPDATE password_resets SET used_at = CURRENT_TIMESTAMP WHERE token = ?")
	_, err := r.db.ExecContext(ctx, query, token)
	return err
}

func (r *authRepository) DeleteExpiredPasswordResets(ctx context.Context) (int, error) {
	query := r.formatQuery("DELETE FROM password_resets WHERE expires_at <= CURRENT_TIMESTAMP OR used_at IS NOT NULL")
	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(rowsAffected), nil
}