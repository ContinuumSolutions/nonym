package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ContinuumSolutions/nonym/pkg/auth/config"
	"github.com/ContinuumSolutions/nonym/pkg/auth/errors"
	"github.com/ContinuumSolutions/nonym/pkg/auth/interfaces"
	"github.com/ContinuumSolutions/nonym/pkg/auth/models"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// sqlRepository implements the AuthRepository interface using standard database/sql
type sqlRepository struct {
	db         *sql.DB
	tx         *sql.Tx // For transaction mode
	config     *config.Config
	isPostgres bool
}

// NewSQL creates a new SQL-based AuthRepository instance
func NewSQL(cfg *config.Config) (interfaces.AuthRepository, error) {
	// Open database connection
	db, err := sql.Open(cfg.Database.Driver, cfg.GetDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)

	// Test connection
	if err := db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	repo := &sqlRepository{
		db:         db,
		config:     cfg,
		isPostgres: cfg.IsPostgreSQL(),
	}

	return repo, nil
}

// formatQuery converts ? placeholders to $1, $2, etc for PostgreSQL
func (r *sqlRepository) formatQuery(query string) string {
	if !r.isPostgres {
		return query
	}

	count := 1
	result := ""
	for _, char := range query {
		if char == '?' {
			result += fmt.Sprintf("$%d", count)
			count++
		} else {
			result += string(char)
		}
	}
	return result
}

// User operations

func (r *sqlRepository) CreateUser(ctx context.Context, req *models.CreateUserRequest) (*models.User, error) {
	if req.ID == uuid.Nil {
		req.ID = uuid.New()
	}

	query := r.formatQuery(`
		INSERT INTO users (id, organization_id, email, password_hash, first_name, last_name, role, is_active, email_verified)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING created_at, updated_at
	`)

	var user models.User
	var createdAt, updatedAt time.Time

	err := r.db.QueryRowContext(ctx, query,
		req.ID,
		req.OrganizationID,
		req.Email,
		req.PasswordHash,
		req.FirstName,
		req.LastName,
		string(req.Role),
		req.IsActive,
		req.EmailVerified,
	).Scan(&createdAt, &updatedAt)

	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, errors.ErrUserExists.WithCause(err)
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Populate the user object
	user.ID = req.ID
	user.OrganizationID = req.OrganizationID
	user.Email = req.Email
	user.PasswordHash = req.PasswordHash
	user.FirstName = req.FirstName
	user.LastName = req.LastName
	user.Role = req.Role
	user.IsActive = req.IsActive
	user.EmailVerified = req.EmailVerified
	user.CreatedAt = createdAt
	user.UpdatedAt = updatedAt

	return &user, nil
}

func (r *sqlRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	var lastLogin sql.NullTime

	query := r.formatQuery(`
		SELECT id, organization_id, email, password_hash, first_name, last_name,
		       role, is_active, email_verified, last_login, created_at, updated_at
		FROM users WHERE id = ? AND is_active = true
	`)

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.OrganizationID,
		&user.Email,
		&user.PasswordHash,
		&user.FirstName,
		&user.LastName,
		&user.Role,
		&user.IsActive,
		&user.EmailVerified,
		&lastLogin,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.ErrUserNotFound.WithCause(err)
		}
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}

	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	return &user, nil
}

func (r *sqlRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	var lastLogin sql.NullTime

	query := r.formatQuery(`
		SELECT id, organization_id, email, password_hash, first_name, last_name,
		       role, is_active, email_verified, last_login, created_at, updated_at
		FROM users WHERE email = ? AND is_active = true
	`)

	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.OrganizationID,
		&user.Email,
		&user.PasswordHash,
		&user.FirstName,
		&user.LastName,
		&user.Role,
		&user.IsActive,
		&user.EmailVerified,
		&lastLogin,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.ErrUserNotFound.WithCause(err)
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	return &user, nil
}

// Implement remaining interface methods...

func (r *sqlRepository) UpdateUser(ctx context.Context, id uuid.UUID, updates *models.UpdateUserRequest) (*models.User, error) {
	// Implementation here...
	return r.GetUserByID(ctx, id) // Simplified for now
}

func (r *sqlRepository) DeleteUser(ctx context.Context, id uuid.UUID) error {
	query := r.formatQuery("UPDATE users SET is_active = false, updated_at = CURRENT_TIMESTAMP WHERE id = ?")
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return errors.ErrUserNotFound
	}

	return nil
}

func (r *sqlRepository) ListUsers(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*models.User, int, error) {
	// Simplified implementation
	return []*models.User{}, 0, nil
}

func (r *sqlRepository) SetUserPassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	query := r.formatQuery("UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?")
	result, err := r.db.ExecContext(ctx, query, passwordHash, userID)
	if err != nil {
		return fmt.Errorf("failed to set user password: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return errors.ErrUserNotFound
	}

	return nil
}

func (r *sqlRepository) UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
	query := r.formatQuery("UPDATE users SET last_login = CURRENT_TIMESTAMP WHERE id = ?")
	_, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}
	return nil
}

func (r *sqlRepository) SetEmailVerified(ctx context.Context, userID uuid.UUID, verified bool) error {
	query := r.formatQuery("UPDATE users SET email_verified = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?")
	_, err := r.db.ExecContext(ctx, query, verified, userID)
	if err != nil {
		return fmt.Errorf("failed to set email verified: %w", err)
	}
	return nil
}

// Organization operations - simplified implementations

func (r *sqlRepository) CreateOrganization(ctx context.Context, req *models.CreateOrganizationRequest) (*models.Organization, error) {
	org := &models.Organization{
		ID:          uuid.New(),
		Name:        req.Name,
		Slug:        generateSlug(req.Name),
		Description: req.Description,
		Settings:    req.Settings,
		IsActive:    true,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	return org, nil // Simplified
}

func (r *sqlRepository) GetOrganizationByID(ctx context.Context, id uuid.UUID) (*models.Organization, error) {
	return &models.Organization{ID: id}, nil // Simplified
}

func (r *sqlRepository) GetOrganizationBySlug(ctx context.Context, slug string) (*models.Organization, error) {
	return &models.Organization{Slug: slug}, nil // Simplified
}

func (r *sqlRepository) UpdateOrganization(ctx context.Context, id uuid.UUID, updates *models.UpdateOrganizationRequest) (*models.Organization, error) {
	return r.GetOrganizationByID(ctx, id)
}

func (r *sqlRepository) DeleteOrganization(ctx context.Context, id uuid.UUID) error {
	return nil // Simplified
}

func (r *sqlRepository) ListOrganizations(ctx context.Context, limit, offset int) ([]*models.Organization, int, error) {
	return []*models.Organization{}, 0, nil // Simplified
}

func (r *sqlRepository) GetOrganizationMembers(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*models.User, int, error) {
	return []*models.User{}, 0, nil // Simplified
}

func (r *sqlRepository) GetOrganizationStats(ctx context.Context, orgID uuid.UUID) (*models.OrganizationStats, error) {
	return &models.OrganizationStats{}, nil // Simplified
}

// Session operations - simplified

func (r *sqlRepository) CreateSession(ctx context.Context, session *interfaces.UserSession) error {
	return nil // Simplified
}

func (r *sqlRepository) GetSession(ctx context.Context, sessionID uuid.UUID) (*interfaces.UserSession, error) {
	return &interfaces.UserSession{}, nil // Simplified
}

func (r *sqlRepository) DeleteSession(ctx context.Context, sessionID uuid.UUID) error {
	return nil // Simplified
}

func (r *sqlRepository) DeleteUserSessions(ctx context.Context, userID uuid.UUID) error {
	return nil // Simplified
}

func (r *sqlRepository) DeleteExpiredSessions(ctx context.Context) (int, error) {
	return 0, nil // Simplified
}

func (r *sqlRepository) UpdateSessionActivity(ctx context.Context, sessionID uuid.UUID) error {
	return nil // Simplified
}

// Token operations - simplified

func (r *sqlRepository) CreateRefreshToken(ctx context.Context, token *interfaces.RefreshToken) error {
	return nil // Simplified
}

func (r *sqlRepository) GetRefreshToken(ctx context.Context, tokenHash string) (*interfaces.RefreshToken, error) {
	return &interfaces.RefreshToken{}, nil // Simplified
}

func (r *sqlRepository) DeleteRefreshToken(ctx context.Context, tokenHash string) error {
	return nil // Simplified
}

func (r *sqlRepository) DeleteUserRefreshTokens(ctx context.Context, userID uuid.UUID) error {
	return nil // Simplified
}

func (r *sqlRepository) DeleteExpiredRefreshTokens(ctx context.Context) (int, error) {
	return 0, nil // Simplified
}

// Audit operations - simplified

func (r *sqlRepository) LogAuthEvent(ctx context.Context, event *interfaces.AuthEvent) error {
	return nil // Simplified
}

func (r *sqlRepository) GetAuthEvents(ctx context.Context, filter *interfaces.AuthEventFilter) ([]*interfaces.AuthEvent, int, error) {
	return []*interfaces.AuthEvent{}, 0, nil // Simplified
}

// Password reset operations - simplified

func (r *sqlRepository) CreatePasswordReset(ctx context.Context, reset *interfaces.PasswordReset) error {
	return nil // Simplified
}

func (r *sqlRepository) GetPasswordReset(ctx context.Context, token string) (*interfaces.PasswordReset, error) {
	return &interfaces.PasswordReset{}, nil // Simplified
}

func (r *sqlRepository) DeletePasswordReset(ctx context.Context, token string) error {
	return nil // Simplified
}

func (r *sqlRepository) DeleteExpiredPasswordResets(ctx context.Context) (int, error) {
	return 0, nil // Simplified
}

// Transaction support

func (r *sqlRepository) WithTx(ctx context.Context, fn func(interfaces.AuthRepository) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p) // Re-throw panic after rollback
		} else if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	// Create a new repository instance with the transaction
	txRepo := &sqlRepository{
		db:         r.db,
		tx:         tx,
		config:     r.config,
		isPostgres: r.isPostgres,
	}

	err = fn(txRepo)
	return err
}

// Health check
func (r *sqlRepository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

// Close database connection
func (r *sqlRepository) Close() error {
	return r.db.Close()
}