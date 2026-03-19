package auth

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

var (
	authDB    *sql.DB
	jwtSecret []byte
)

// InitializeAuth sets up the authentication system
func InitializeAuth(database *sql.DB) error {
	authDB = database

	// Get JWT secret from environment or generate one
	secretEnv := os.Getenv("JWT_SECRET")
	if secretEnv != "" && secretEnv != "change-in-production" {
		jwtSecret = []byte(secretEnv)
	} else {
		// Generate a random secret for development
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return fmt.Errorf("failed to generate JWT secret: %w", err)
		}
		jwtSecret = secret
		log.Println("WARNING: Using randomly generated JWT secret. Set JWT_SECRET environment variable for production.")
	}

	log.Println("Authentication system initialized successfully")
	return nil
}

// isPostgreSQL checks if we're using PostgreSQL
func isPostgreSQL() bool {
	return os.Getenv("DB_HOST") != "" && os.Getenv("DB_NAME") != ""
}

// formatQuery converts ? placeholders to $1, $2, etc for PostgreSQL
func formatQuery(query string) string {
	if !isPostgreSQL() {
		return query
	}

	count := 1
	result := ""
	for _, r := range query {
		if r == '?' {
			result += fmt.Sprintf("$%d", count)
			count++
		} else {
			result += string(r)
		}
	}
	return result
}

// generateSlug creates a URL-friendly slug from text
func generateSlug(text string) string {
	slug := strings.ToLower(text)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")

	// Remove non-alphanumeric characters except hyphens
	reg := regexp.MustCompile(`[^a-z0-9\-]`)
	slug = reg.ReplaceAllString(slug, "")

	// Remove duplicate hyphens
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}

	// Trim hyphens from start and end
	slug = strings.Trim(slug, "-")

	if len(slug) == 0 {
		slug = "organization"
	}
	return slug
}

// checkUniqueSlug ensures slug is unique
func checkUniqueSlug(slug string) (string, error) {
	var count int
	query := formatQuery("SELECT COUNT(*) FROM organizations WHERE slug = ?")
	err := authDB.QueryRow(query, slug).Scan(&count)
	if err != nil {
		return "", err
	}

	originalSlug := slug
	counter := 1
	for count > 0 {
		slug = fmt.Sprintf("%s-%d", originalSlug, counter)
		err := authDB.QueryRow(query, slug).Scan(&count)
		if err != nil {
			return "", err
		}
		counter++
	}

	return slug, nil
}

// hashPassword hashes a password using bcrypt
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// verifyPassword verifies a password against its hash
func verifyPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

// generateJWTToken generates a JWT token for a user
func generateJWTToken(user *User) (string, time.Time, error) {
	expiresAt := time.Now().Add(24 * time.Hour) // 24 hour expiry

	claims := jwt.MapClaims{
		"user_id":         user.ID.String(),
		"email":           user.Email,
		"role":            string(user.Role),
		"organization_id": user.OrganizationID.String(),
		"exp":             expiresAt.Unix(),
		"iat":             time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiresAt, nil
}

// validateToken validates a JWT token and returns the user
func validateToken(tokenString string) (*User, error) {
	if authDB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Parse token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("token is not valid")
	}

	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Get user ID
	userIDStr, ok := claims["user_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid user ID in token")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID format in token")
	}

	// Get user from database
	user, err := getUserByID(userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	if !user.IsActive {
		return nil, fmt.Errorf("user account is disabled")
	}

	return user, nil
}

// getUserByID gets a user by ID
func getUserByID(userID uuid.UUID) (*User, error) {
	user := &User{}
	query := formatQuery(`
		SELECT id, email, password_hash, first_name, last_name, role,
		       organization_id, is_active, email_verified, last_login,
		       created_at, updated_at
		FROM users
		WHERE id = ? AND is_active = true
	`)

	var lastLogin sql.NullTime
	err := authDB.QueryRow(query, userID).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.FirstName,
		&user.LastName, &user.Role, &user.OrganizationID, &user.IsActive,
		&user.EmailVerified, &lastLogin, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}

	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	return user, nil
}

// getUserByEmail gets a user by email
func getUserByEmail(email string) (*User, error) {
	user := &User{}
	query := formatQuery(`
		SELECT id, email, password_hash, first_name, last_name, role,
		       organization_id, is_active, email_verified, last_login,
		       created_at, updated_at
		FROM users
		WHERE email = ? AND is_active = true
	`)

	var lastLogin sql.NullTime
	err := authDB.QueryRow(query, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.FirstName,
		&user.LastName, &user.Role, &user.OrganizationID, &user.IsActive,
		&user.EmailVerified, &lastLogin, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}

	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	return user, nil
}

// getOrganizationByID gets an organization by ID
func getOrganizationByID(orgID uuid.UUID) (*Organization, error) {
	org := &Organization{}
	query := formatQuery(`
		SELECT id, name, slug, description, owner_id, is_active, created_at, updated_at
		FROM organizations
		WHERE id = ? AND is_active = true
	`)

	err := authDB.QueryRow(query, orgID).Scan(
		&org.ID, &org.Name, &org.Slug, &org.Description,
		&org.OwnerID, &org.IsActive, &org.CreatedAt, &org.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("organization not found")
		}
		return nil, err
	}

	return org, nil
}

// SignupUser handles user signup with atomic database transactions
func SignupUser(req *SignupRequest) (*User, *Organization, error) {
	if authDB == nil {
		return nil, nil, fmt.Errorf("database not initialized")
	}

	// Validate email format
	if !strings.Contains(req.Email, "@") || len(req.Email) < 5 {
		return nil, nil, fmt.Errorf("invalid email format")
	}

	// Validate password strength
	if len(req.Password) < 8 {
		return nil, nil, fmt.Errorf("password must be at least 8 characters long")
	}

	// Validate names
	if strings.TrimSpace(req.FirstName) == "" || strings.TrimSpace(req.LastName) == "" {
		return nil, nil, fmt.Errorf("first name and last name are required")
	}

	// Start atomic database transaction
	tx, err := authDB.Begin()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start transaction: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r) // Re-throw panic after rollback
		} else if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	// Step a: Check if user exists with email
	var existingUserID uuid.UUID
	checkQuery := formatQuery("SELECT id FROM users WHERE email = ?")
	checkErr := tx.QueryRow(checkQuery, req.Email).Scan(&existingUserID)
	if checkErr == nil {
		return nil, nil, fmt.Errorf("user with this email already exists, please login instead")
	}

	// Hash password
	hashedPassword, err := hashPassword(req.Password)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to hash password: %w", err)
	}

	var user *User
	var organization *Organization

	if req.OrganizationID != nil {
		// User is joining existing organization
		// Verify organization exists
		orgQuery := formatQuery("SELECT id, name, slug, description, owner_id, is_active, created_at, updated_at FROM organizations WHERE id = ? AND is_active = true")
		organization = &Organization{}
		err = tx.QueryRow(orgQuery, *req.OrganizationID).Scan(
			&organization.ID, &organization.Name, &organization.Slug,
			&organization.Description, &organization.OwnerID, &organization.IsActive,
			&organization.CreatedAt, &organization.UpdatedAt,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid organization: %w", err)
		}

		// Create user with regular role
		user = &User{
			ID:             uuid.New(),
			Email:          req.Email,
			PasswordHash:   hashedPassword,
			FirstName:      strings.TrimSpace(req.FirstName),
			LastName:       strings.TrimSpace(req.LastName),
			Role:           RoleUser,
			OrganizationID: *req.OrganizationID,
			IsActive:       true,
			EmailVerified:  false,
		}
	} else {
		// Step b: Create new organization and mark user as owner
		orgName := req.Organization
		if orgName == "" {
			// Extract organization from email domain
			emailParts := strings.Split(req.Email, "@")
			if len(emailParts) > 1 {
				domain := strings.Split(emailParts[1], ".")[0]
				orgName = strings.Title(domain) + " Organization"
			} else {
				orgName = "My Organization"
			}
		}

		// Generate unique slug
		slug := generateSlug(orgName)
		uniqueSlug, err := checkUniqueSlugTx(tx, slug)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to generate unique slug: %w", err)
		}

		// Create organization
		organization = &Organization{
			ID:          uuid.New(),
			Name:        orgName,
			Slug:        uniqueSlug,
			Description: fmt.Sprintf("Organization for %s %s", req.FirstName, req.LastName),
			IsActive:    true,
		}

		orgInsertQuery := formatQuery(`
			INSERT INTO organizations (id, name, slug, description, owner_id, is_active, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		`)

		// Create user as organization owner
		user = &User{
			ID:             uuid.New(),
			Email:          req.Email,
			PasswordHash:   hashedPassword,
			FirstName:      strings.TrimSpace(req.FirstName),
			LastName:       strings.TrimSpace(req.LastName),
			Role:           RoleOwner,
			OrganizationID: organization.ID,
			IsActive:       true,
			EmailVerified:  false,
		}

		// Set owner_id to user ID
		organization.OwnerID = user.ID

		// Insert organization
		_, err = tx.Exec(orgInsertQuery,
			organization.ID, organization.Name, organization.Slug,
			organization.Description, organization.OwnerID, organization.IsActive,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create organization: %w", err)
		}

		// Get the created_at and updated_at timestamps
		timeQuery := formatQuery("SELECT created_at, updated_at FROM organizations WHERE id = ?")
		err = tx.QueryRow(timeQuery, organization.ID).Scan(&organization.CreatedAt, &organization.UpdatedAt)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get organization timestamps: %w", err)
		}
	}

	// Insert user
	userInsertQuery := formatQuery(`
		INSERT INTO users (id, email, password_hash, first_name, last_name, role,
		                   organization_id, is_active, email_verified, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)

	_, err = tx.Exec(userInsertQuery,
		user.ID, user.Email, user.PasswordHash, user.FirstName,
		user.LastName, string(user.Role), user.OrganizationID,
		user.IsActive, user.EmailVerified,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Get the created_at and updated_at timestamps for user
	userTimeQuery := formatQuery("SELECT created_at, updated_at FROM users WHERE id = ?")
	err = tx.QueryRow(userTimeQuery, user.ID).Scan(&user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user timestamps: %w", err)
	}

	// Step c: Update the team account (this is implicitly done by creating the user)

	// Step d: TODO later - Send email to user (only after transaction commit)
	// This will be implemented in a separate function for future use

	log.Printf("New user registered: %s (ID: %s, Org: %s, Role: %s)",
		user.Email, user.ID, user.OrganizationID, user.Role)

	return user, organization, nil
}

// checkUniqueSlugTx checks for unique slug within a transaction
func checkUniqueSlugTx(tx *sql.Tx, slug string) (string, error) {
	var count int
	query := formatQuery("SELECT COUNT(*) FROM organizations WHERE slug = ?")
	err := tx.QueryRow(query, slug).Scan(&count)
	if err != nil {
		return "", err
	}

	originalSlug := slug
	counter := 1
	for count > 0 {
		slug = fmt.Sprintf("%s-%d", originalSlug, counter)
		err := tx.QueryRow(query, slug).Scan(&count)
		if err != nil {
			return "", err
		}
		counter++
	}

	return slug, nil
}

// LoginUser authenticates a user and returns a JWT token
func LoginUser(req *LoginRequest, clientIP, userAgent string) (*LoginResponse, error) {
	if authDB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Get user by email
	user, err := getUserByEmail(req.Email)
	if err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}

	// Check if user is active
	if !user.IsActive {
		return nil, fmt.Errorf("user account is disabled")
	}

	// Verify password
	if !verifyPassword(user.PasswordHash, req.Password) {
		return nil, fmt.Errorf("invalid email or password")
	}

	// Get user's organization
	organization, err := getOrganizationByID(user.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to load organization: %w", err)
	}

	// Set organization in user object
	user.Organization = organization

	// Generate JWT token
	token, expiresAt, err := generateJWTToken(user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Update last login
	updateLoginQuery := formatQuery("UPDATE users SET last_login = CURRENT_TIMESTAMP WHERE id = ?")
	authDB.Exec(updateLoginQuery, user.ID)

	log.Printf("User logged in: %s (ID: %s, Org: %s)", user.Email, user.ID, user.OrganizationID)

	return &LoginResponse{
		Token:        token,
		ExpiresAt:    expiresAt,
		User:         user.ToProfile(),
		Organization: organization,
	}, nil
}

// ValidateToken validates a JWT token and returns the user
func ValidateToken(tokenString string) (*User, error) {
	return validateToken(tokenString)
}

// LogoutUser invalidates a user's session (for future implementation)
func LogoutUser(tokenString string) error {
	// For JWT-only auth, there's nothing to invalidate server-side
	// In a real implementation, you'd maintain a blacklist
	return nil
}

// GetUserProfile returns a user profile
func GetUserProfile(userID uuid.UUID) (*UserProfile, error) {
	user, err := getUserByID(userID)
	if err != nil {
		return nil, err
	}

	// Get organization details
	organization, err := getOrganizationByID(user.OrganizationID)
	if err != nil {
		log.Printf("Warning: Failed to load organization for user %s: %v", userID, err)
		// Continue without organization data
	} else {
		user.Organization = organization
	}

	return user.ToProfile(), nil
}

// SendWelcomeEmail is a placeholder function for future email implementation
func SendWelcomeEmail(user *User, organization *Organization) error {
	// TODO: Implement email sending functionality
	// This should only be called after the database transaction is committed
	log.Printf("TODO: Send welcome email to %s for organization %s", user.Email, organization.Name)
	return nil
}