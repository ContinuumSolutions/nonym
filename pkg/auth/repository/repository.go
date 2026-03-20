package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/ContinuumSolutions/nonym/pkg/auth/config"
	"github.com/ContinuumSolutions/nonym/pkg/auth/errors"
	"github.com/ContinuumSolutions/nonym/pkg/auth/interfaces"
	"github.com/ContinuumSolutions/nonym/pkg/auth/models"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// authRepository implements the AuthRepository interface
type authRepository struct {
	db         *sqlx.DB
	tx         *sqlx.Tx // For transaction mode
	config     *config.Config
	isPostgres bool
}

// getDB returns the appropriate database connection (tx if in transaction, db otherwise)
// Note: This method was replaced with direct tx/db access in query methods for better type safety

// New creates a new AuthRepository instance
func New(cfg *config.Config) (interfaces.AuthRepository, error) {
	// Open database connection
	db, err := sqlx.Connect(cfg.Database.Driver, cfg.GetDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	repo := &authRepository{
		db:         db,
		config:     cfg,
		isPostgres: cfg.IsPostgreSQL(),
	}

	return repo, nil
}

// formatQuery converts ? placeholders to $1, $2, etc for PostgreSQL
func (r *authRepository) formatQuery(query string) string {
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

func (r *authRepository) CreateUser(ctx context.Context, req *models.CreateUserRequest) (*models.User, error) {
	if req.ID == uuid.Nil {
		req.ID = uuid.New()
	}

	query := r.formatQuery(`
		INSERT INTO users (id, organization_id, email, password_hash, first_name, last_name, role, is_active, email_verified)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, created_at, updated_at
	`)

	var user models.User
	var returnedUserID uuid.UUID
	var createdAt, updatedAt time.Time
	var err error

	if r.tx != nil {
		// In transaction, use the transaction's QueryRowxContext
		err = r.tx.QueryRowxContext(ctx, query,
			req.ID,
			req.OrganizationID,
			req.Email,
			req.PasswordHash,
			req.FirstName,
			req.LastName,
			string(req.Role),
			req.IsActive,
			req.EmailVerified,
		).Scan(&returnedUserID, &createdAt, &updatedAt)
	} else {
		// Not in transaction, use the DB's QueryRowxContext
		err = r.db.QueryRowxContext(ctx, query,
			req.ID,
			req.OrganizationID,
			req.Email,
			req.PasswordHash,
			req.FirstName,
			req.LastName,
			string(req.Role),
			req.IsActive,
			req.EmailVerified,
		).Scan(&returnedUserID, &createdAt, &updatedAt)
	}

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

func (r *authRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	query := r.formatQuery(`
		SELECT id, organization_id, email, password_hash, first_name, last_name,
		       role, is_active, email_verified, last_login, created_at, updated_at
		FROM users WHERE id = ? AND is_active = true
	`)

	err := r.db.GetContext(ctx, &user, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.ErrUserNotFound.WithCause(err)
		}
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}

	return &user, nil
}

func (r *authRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	query := r.formatQuery(`
		SELECT id, organization_id, email, password_hash, first_name, last_name,
		       role, is_active, email_verified, last_login, created_at, updated_at
		FROM users WHERE email = ? AND is_active = true
	`)

	err := r.db.GetContext(ctx, &user, query, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.ErrUserNotFound.WithCause(err)
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	return &user, nil
}

func (r *authRepository) UpdateUser(ctx context.Context, id uuid.UUID, updates *models.UpdateUserRequest) (*models.User, error) {
	setParts := []string{"updated_at = CURRENT_TIMESTAMP"}
	args := []interface{}{}

	if updates.FirstName != nil {
		setParts = append(setParts, "first_name = ?")
		args = append(args, *updates.FirstName)
	}
	if updates.LastName != nil {
		setParts = append(setParts, "last_name = ?")
		args = append(args, *updates.LastName)
	}
	if updates.Role != nil {
		setParts = append(setParts, "role = ?")
		args = append(args, string(*updates.Role))
	}
	if updates.IsActive != nil {
		setParts = append(setParts, "is_active = ?")
		args = append(args, *updates.IsActive)
	}

	if len(args) == 0 {
		return r.GetUserByID(ctx, id)
	}

	args = append(args, id)
	query := r.formatQuery(fmt.Sprintf("UPDATE users SET %s WHERE id = ?", joinString(setParts, ", ")))

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return r.GetUserByID(ctx, id)
}

func (r *authRepository) DeleteUser(ctx context.Context, id uuid.UUID) error {
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

func (r *authRepository) ListUsers(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*models.User, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	// Get total count
	countQuery := r.formatQuery("SELECT COUNT(*) FROM users WHERE organization_id = ? AND is_active = true")
	var total int
	err := r.db.GetContext(ctx, &total, countQuery, orgID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get user count: %w", err)
	}

	// Get users
	query := r.formatQuery(`
		SELECT id, organization_id, email, password_hash, first_name, last_name,
		       role, is_active, email_verified, last_login, created_at, updated_at
		FROM users
		WHERE organization_id = ? AND is_active = true
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`)

	var users []*models.User
	err = r.db.SelectContext(ctx, &users, query, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}

	return users, total, nil
}

func (r *authRepository) SetUserPassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
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

func (r *authRepository) UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
	query := r.formatQuery("UPDATE users SET last_login = CURRENT_TIMESTAMP WHERE id = ?")
	_, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}
	return nil
}

func (r *authRepository) SetEmailVerified(ctx context.Context, userID uuid.UUID, verified bool) error {
	query := r.formatQuery("UPDATE users SET email_verified = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?")
	_, err := r.db.ExecContext(ctx, query, verified, userID)
	if err != nil {
		return fmt.Errorf("failed to set email verified: %w", err)
	}
	return nil
}

// Organization operations

func (r *authRepository) CreateOrganization(ctx context.Context, req *models.CreateOrganizationRequest) (*models.Organization, error) {
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

	// Ensure unique slug
	for {
		exists, err := r.slugExists(ctx, org.Slug)
		if err != nil {
			return nil, fmt.Errorf("failed to check slug existence: %w", err)
		}
		if !exists {
			break
		}
		org.Slug = generateUniqueSlug(org.Slug)
	}

	var settingsJSON string
	if r.isPostgres {
		// PostgreSQL uses JSONB
		settingsJSON = toJSONB(org.Settings)
	} else {
		// SQLite uses TEXT
		settingsJSON = toJSON(org.Settings)
	}

	query := r.formatQuery(`
		INSERT INTO organizations (id, name, slug, description, settings, is_active)
		VALUES (?, ?, ?, ?, ?, ?)
		RETURNING id, created_at, updated_at
	`)

	var returnedID uuid.UUID
	err := r.db.QueryRowxContext(ctx, query,
		org.ID,
		org.Name,
		org.Slug,
		org.Description,
		settingsJSON,
		org.IsActive,
	).Scan(&returnedID, &org.CreatedAt, &org.UpdatedAt)

	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, errors.ErrOrgExists.WithCause(err)
		}
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	return org, nil
}

func (r *authRepository) GetOrganizationByID(ctx context.Context, id uuid.UUID) (*models.Organization, error) {
	var org models.Organization
	var settingsJSON string

	query := r.formatQuery(`
		SELECT id, name, slug, description, settings, is_active, created_at, updated_at
		FROM organizations WHERE id = ?
	`)

	err := r.db.QueryRowxContext(ctx, query, id).Scan(
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
		if err == sql.ErrNoRows {
			return nil, errors.ErrOrgNotFound.WithCause(err)
		}
		return nil, fmt.Errorf("failed to get organization by ID: %w", err)
	}

	// Parse settings JSON
	if err := fromJSON(settingsJSON, &org.Settings); err != nil {
		return nil, fmt.Errorf("failed to parse organization settings: %w", err)
	}

	return &org, nil
}

func (r *authRepository) GetOrganizationBySlug(ctx context.Context, slug string) (*models.Organization, error) {
	var org models.Organization
	var settingsJSON string

	query := r.formatQuery(`
		SELECT id, name, slug, description, settings, is_active, created_at, updated_at
		FROM organizations WHERE slug = ?
	`)

	err := r.db.QueryRowxContext(ctx, query, slug).Scan(
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
		if err == sql.ErrNoRows {
			return nil, errors.ErrOrgNotFound.WithCause(err)
		}
		return nil, fmt.Errorf("failed to get organization by slug: %w", err)
	}

	// Parse settings JSON
	if err := fromJSON(settingsJSON, &org.Settings); err != nil {
		return nil, fmt.Errorf("failed to parse organization settings: %w", err)
	}

	return &org, nil
}

// Additional helper methods...

// Health check
func (r *authRepository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

// Close database connection
func (r *authRepository) Close() error {
	return r.db.Close()
}

// Transaction support
func (r *authRepository) WithTx(ctx context.Context, fn func(interfaces.AuthRepository) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
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
	txRepo := &authRepository{
		db:         r.db,
		tx:         tx,
		config:     r.config,
		isPostgres: r.isPostgres,
	}

	err = fn(txRepo)
	return err
}
