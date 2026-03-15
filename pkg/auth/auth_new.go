package auth

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// SignupUser creates a new user with their own organization in an atomic transaction
func SignupUser(req *RegisterRequest, clientIP, userAgent string) (*LoginResponse, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Validate input
	if err := validateSignupRequest(req); err != nil {
		return nil, err
	}

	// Start atomic transaction
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Generate organization slug from name or email domain
	orgName := req.Organization
	if orgName == "" {
		// Extract domain from email as fallback
		emailParts := strings.Split(req.Email, "@")
		if len(emailParts) == 2 {
			domain := strings.Split(emailParts[1], ".")[0]
			orgName = strings.ToUpper(domain[:1]) + domain[1:]
		} else {
			orgName = "My Organization"
		}
	}
	orgSlug := generateOrgSlug(orgName)

	// Create organization
	var orgID string
	orgQuery := `INSERT INTO organizations (name, slug, description)
				 VALUES ($1, $2, $3) RETURNING id`
	err = tx.QueryRow(orgQuery, orgName, orgSlug, fmt.Sprintf("Organization for %s", req.Name)).
		Scan(&orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user as organization admin
	var userID string
	userQuery := `INSERT INTO users (organization_id, email, password_hash, first_name, last_name, role, is_active, email_verified)
				  VALUES ($1, $2, $3, $4, $5, 'admin', true, false) RETURNING id`

	firstName, lastName := splitName(req.Name)
	err = tx.QueryRow(userQuery, orgID, req.Email, string(passwordHash), firstName, lastName).
		Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Update organization with owner_id
	_, err = tx.Exec(`UPDATE organizations SET owner_id = $1 WHERE id = $2`, userID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to set organization owner: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Generate JWT token
	token, expiresAt, err := generateJWT(userID, orgID, req.Email, "admin")
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Create session record
	if err := createUserSession(userID, orgID, token, expiresAt, clientIP, userAgent); err != nil {
		log.Printf("Warning: failed to create session record: %v", err)
	}

	// Load complete user and organization data
	user, err := getUserByIDWithOrg(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to load user: %w", err)
	}

	// Update last login
	updateLastLogin(userID)

	log.Printf("New user registered: %s (ID: %s, Org: %s)", req.Email, userID, orgID)

	return &LoginResponse{
		Token:        token,
		ExpiresAt:    expiresAt,
		User:         user,
		Organization: user.Organization,
	}, nil
}

// AuthenticateUser validates credentials and returns authentication data
func AuthenticateUser(req *LoginRequest, clientIP, userAgent string) (*LoginResponse, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Validate input
	if req.Email == "" || req.Password == "" {
		return nil, fmt.Errorf("email and password are required")
	}

	// Get user with organization
	user, err := getUserByEmailWithOrg(req.Email)
	if err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}

	// Check if user is active
	if !user.Active {
		return nil, fmt.Errorf("account is disabled")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}

	// If organization ID is specified in request, validate user belongs to it
	if req.OrganizationID != nil && *req.OrganizationID != user.OrganizationID {
		return nil, fmt.Errorf("invalid organization")
	}

	// Generate JWT token
	token, expiresAt, err := generateJWT(strconv.Itoa(user.ID), strconv.Itoa(user.OrganizationID), user.Email, user.Role)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Create session record
	if err := createUserSession(strconv.Itoa(user.ID), strconv.Itoa(user.OrganizationID), token, expiresAt, clientIP, userAgent); err != nil {
		log.Printf("Warning: failed to create session record: %v", err)
	}

	// Update last login
	updateLastLogin(strconv.Itoa(user.ID))

	log.Printf("User logged in: %s (ID: %d, Org: %d)", user.Email, user.ID, user.OrganizationID)

	return &LoginResponse{
		Token:        token,
		ExpiresAt:    expiresAt,
		User:         user,
		Organization: user.Organization,
	}, nil
}

// Helper functions
func validateSignupRequest(req *RegisterRequest) error {
	if req.Email == "" {
		return fmt.Errorf("email is required")
	}
	if req.Password == "" {
		return fmt.Errorf("password is required")
	}
	if len(req.Password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if req.Name == "" && req.FirstName == "" {
		return fmt.Errorf("name is required")
	}

	// Validate email format
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(req.Email) {
		return fmt.Errorf("invalid email format")
	}

	return nil
}

func generateOrgSlug(name string) string {
	// Convert to lowercase and replace spaces/special chars with hyphens
	slug := strings.ToLower(name)
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	slug = reg.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")

	// Ensure maximum length
	if len(slug) > 50 {
		slug = slug[:50]
	}

	return slug
}

func splitName(fullName string) (string, string) {
	parts := strings.Fields(strings.TrimSpace(fullName))
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}

	firstName := parts[0]
	lastName := strings.Join(parts[1:], " ")
	return firstName, lastName
}

func generateJWT(userID, orgID string, email, role string) (string, time.Time, error) {
	if jwtSecret == nil {
		return "", time.Time{}, fmt.Errorf("JWT secret not initialized")
	}

	expiresAt := time.Now().Add(24 * time.Hour) // 24 hour expiry

	claims := jwt.MapClaims{
		"user_id":          userID,
		"organization_id":  orgID,
		"email":           email,
		"role":            role,
		"iat":             time.Now().Unix(),
		"exp":             expiresAt.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiresAt, nil
}

func createUserSession(userID, orgID string, token string, expiresAt time.Time, clientIP, userAgent string) error {
	query := `INSERT INTO user_sessions (user_id, session_token, expires_at, created_at, last_accessed)
			  VALUES ($1, $2, $3, $4, $5)`

	now := time.Now()
	_, err := db.Exec(query, userID, token, expiresAt, now, now)
	return err
}

func getUserByEmailWithOrg(email string) (*User, error) {
	user := &User{}

	query := `SELECT u.id, u.email, u.password_hash,
					 COALESCE(u.first_name || ' ' || u.last_name, u.first_name, u.last_name, '') as name,
					 u.role, u.organization_id, u.is_active, u.created_at, u.updated_at, u.last_login,
					 o.id, o.name, o.slug, o.description, o.created_at, o.updated_at
			  FROM users u
			  JOIN organizations o ON u.organization_id = o.id
			  WHERE u.email = $1 AND u.is_active = true`

	var lastLogin sql.NullTime
	org := &Organization{}

	err := db.QueryRow(query, email).Scan(
		&user.ID, &user.Email, &user.Password, &user.Name, &user.Role,
		&user.OrganizationID, &user.Active, &user.CreatedAt, &user.UpdatedAt, &lastLogin,
		&org.ID, &org.Name, &org.Slug, &org.Description, &org.CreatedAt, &org.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	user.Organization = org
	return user, nil
}

func getUserByIDWithOrg(userID string) (*User, error) {
	user := &User{}

	query := `SELECT u.id, u.email, u.password_hash,
					 COALESCE(u.first_name || ' ' || u.last_name, u.first_name, u.last_name, '') as name,
					 u.role, u.organization_id, u.is_active, u.created_at, u.updated_at, u.last_login,
					 o.id, o.name, o.slug, o.description, o.created_at, o.updated_at
			  FROM users u
			  JOIN organizations o ON u.organization_id = o.id
			  WHERE u.id = $1 AND u.is_active = true`

	var lastLogin sql.NullTime
	org := &Organization{}

	err := db.QueryRow(query, userID).Scan(
		&user.ID, &user.Email, &user.Password, &user.Name, &user.Role,
		&user.OrganizationID, &user.Active, &user.CreatedAt, &user.UpdatedAt, &lastLogin,
		&org.ID, &org.Name, &org.Slug, &org.Description, &org.CreatedAt, &org.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	user.Organization = org
	return user, nil
}

func updateLastLogin(userID string) {
	query := `UPDATE users SET last_login = $1 WHERE id = $2`
	_, err := db.Exec(query, time.Now(), userID)
	if err != nil {
		log.Printf("Warning: failed to update last login for user %s: %v", userID, err)
	}
}

// GetUserContext extracts user and organization from authentication context
func GetUserContext(c interface{}) (int, int, error) {
	// This would be called from fiber context in middleware
	// Implementation depends on how context is structured
	// For now, placeholder that should be implemented based on middleware
	return 0, 0, fmt.Errorf("not implemented - should extract from fiber context")
}

// FilterByOrganization ensures queries are scoped to user's organization
func FilterByOrganization(baseQuery string, orgID int) string {
	// Add organization filter to any query
	if strings.Contains(strings.ToLower(baseQuery), "where") {
		return baseQuery + fmt.Sprintf(" AND organization_id = %d", orgID)
	} else {
		return baseQuery + fmt.Sprintf(" WHERE organization_id = %d", orgID)
	}
}