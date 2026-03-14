package auth

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

var (
	db        *sql.DB
	jwtSecret []byte
)

// isPostgreSQL checks if we're using PostgreSQL by checking environment variables
func isPostgreSQL() bool {
	return os.Getenv("DB_HOST") != "" && os.Getenv("DB_NAME") != ""
}

// Initialize sets up the authentication system
func Initialize(database *sql.DB) error {
	db = database

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

	// Create auth tables (skip if PostgreSQL as they're already created by schema)
	if !isPostgreSQL() {
		if err := createAuthTables(); err != nil {
			return fmt.Errorf("failed to create auth tables: %w", err)
		}
	}

	// Run migrations for schema updates (skip if PostgreSQL as schema is complete)
	if !isPostgreSQL() {
		if err := runAuthMigrations(); err != nil {
			return fmt.Errorf("failed to run auth migrations: %w", err)
		}
	}

	// Create indexes after migrations have run (skip if PostgreSQL as indexes are in schema)
	if !isPostgreSQL() {
		if err := createAuthIndexes(); err != nil {
			return fmt.Errorf("failed to create auth indexes: %w", err)
		}
	}

	// Create default admin user if none exists
	if err := createDefaultUser(); err != nil {
		log.Printf("Warning: Failed to create default user: %v", err)
	}

	log.Println("Authentication system initialized successfully")
	return nil
}

func createAuthTables() error {
	// Step 1: Create base tables without problematic columns
	baseQueries := []string{
		`CREATE TABLE IF NOT EXISTS organizations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			industry TEXT,
			size TEXT,
			country TEXT,
			description TEXT,
			owner_id INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			name TEXT NOT NULL,
			role TEXT DEFAULT 'user',
			organization_id INTEGER NOT NULL,
			active BOOLEAN DEFAULT true,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_login DATETIME,
			FOREIGN KEY (organization_id) REFERENCES organizations (id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS api_keys (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			key_hash TEXT NOT NULL,
			masked_key TEXT NOT NULL,
			permissions TEXT NOT NULL,
			user_id TEXT NOT NULL,
			organization_id INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME,
			status TEXT DEFAULT 'active',
			last_used DATETIME,
			FOREIGN KEY (organization_id) REFERENCES organizations (id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS provider_configs (
			user_id TEXT PRIMARY KEY,
			organization_id INTEGER NOT NULL,
			config_data TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (organization_id) REFERENCES organizations (id) ON DELETE CASCADE
		)`,
	}

	for _, query := range baseQueries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute base query %s: %w", query, err)
		}
	}

	return nil
}

func createAuthIndexes() error {
	// Step 2: Create indexes after migrations have run
	indexQueries := []string{
		`CREATE INDEX IF NOT EXISTS idx_organizations_slug ON organizations(slug)`,
		`CREATE INDEX IF NOT EXISTS idx_organizations_owner ON organizations(owner_id)`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
		`CREATE INDEX IF NOT EXISTS idx_users_active ON users(active)`,
		`CREATE INDEX IF NOT EXISTS idx_users_organization ON users(organization_id)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_organization ON api_keys(organization_id)`,
		`CREATE INDEX IF NOT EXISTS idx_provider_configs_organization ON provider_configs(organization_id)`,
	}

	for _, query := range indexQueries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute index query %s: %w", query, err)
		}
	}

	return nil
}

func runAuthMigrations() error {
	// Migration 1: Add encrypted_key column if it doesn't exist
	_, err := db.Exec(`ALTER TABLE api_keys ADD COLUMN encrypted_key TEXT DEFAULT ''`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return fmt.Errorf("failed to add encrypted_key column: %w", err)
	}

	// Migration 2: Add slug column to organizations if it doesn't exist (without UNIQUE constraint)
	_, err = db.Exec(`ALTER TABLE organizations ADD COLUMN slug TEXT`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") && !strings.Contains(err.Error(), "already exists") {
		log.Printf("Migration warning - slug column: %v", err)
	}

	// Migration 3: Populate slug column with default values for existing organizations
	_, err = db.Exec(`UPDATE organizations SET slug = 'org-' || id WHERE slug IS NULL OR slug = ''`)
	if err != nil {
		log.Printf("Migration warning - slug population: %v", err)
	}

	// Migration 4: Add organization_id column to users if it doesn't exist
	_, err = db.Exec(`ALTER TABLE users ADD COLUMN organization_id INTEGER NOT NULL DEFAULT 1`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") && !strings.Contains(err.Error(), "already exists") {
		log.Printf("Migration warning - users organization_id column: %v", err)
	}

	// Migration 5: Add last_login column to users if it doesn't exist
	_, err = db.Exec(`ALTER TABLE users ADD COLUMN last_login DATETIME`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") && !strings.Contains(err.Error(), "already exists") {
		log.Printf("Migration warning - users last_login column: %v", err)
	}

	// Migration 6: Add missing columns to api_keys table
	apiKeyMigrations := []string{
		`ALTER TABLE api_keys ADD COLUMN masked_key TEXT DEFAULT ''`,
		`ALTER TABLE api_keys ADD COLUMN organization_id INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE api_keys ADD COLUMN expires_at DATETIME`,
		`ALTER TABLE api_keys ADD COLUMN status TEXT DEFAULT 'active'`,
		`ALTER TABLE api_keys ADD COLUMN last_used DATETIME`,
	}

	for _, migration := range apiKeyMigrations {
		_, err = db.Exec(migration)
		if err != nil && !strings.Contains(err.Error(), "duplicate column name") && !strings.Contains(err.Error(), "already exists") {
			log.Printf("Migration warning - api_keys: %v", err)
		}
	}

	return nil
}

func createDefaultUser() error {
	// Check if any users exist
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		return nil // Users already exist
	}

	// Create default admin user
	hashedPassword, err := hashPassword("admin123")
	if err != nil {
		return err
	}

	query := `INSERT INTO users (email, password, name, role, active)
			  VALUES (?, ?, ?, ?, ?)`

	_, err = db.Exec(query, "admin@gateway.local", hashedPassword, "Administrator", "admin", true)
	if err != nil {
		return err
	}

	log.Println("Created default admin user: admin@gateway.local / admin123")
	log.Println("IMPORTANT: Change the default password immediately in production!")
	return nil
}

// Organization management functions

// CreateOrganization creates a new organization
func CreateOrganization(req *OrganizationCreateRequest, ownerID int) (*Organization, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Generate a slug from the organization name
	slug := generateSlug(req.Name)

	// Check if slug already exists
	var existingID int
	err := db.QueryRow("SELECT id FROM organizations WHERE slug = ?", slug).Scan(&existingID)
	if err == nil {
		// Append a number to make it unique
		counter := 1
		for {
			newSlug := fmt.Sprintf("%s-%d", slug, counter)
			err := db.QueryRow("SELECT id FROM organizations WHERE slug = ?", newSlug).Scan(&existingID)
			if err != nil {
				slug = newSlug
				break
			}
			counter++
		}
	}

	query := `INSERT INTO organizations (name, slug, industry, size, country, description, owner_id)
			  VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id, created_at, updated_at`

	var org Organization
	err = db.QueryRow(query, req.Name, slug, req.Industry, req.Size, req.Country, req.Description, ownerID).
		Scan(&org.ID, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	org.Name = req.Name
	org.Slug = slug
	org.Industry = req.Industry
	org.Size = req.Size
	org.Country = req.Country
	org.Description = req.Description
	org.OwnerID = ownerID

	log.Printf("New organization created: %s (ID: %d, Owner: %d)", org.Name, org.ID, ownerID)
	return &org, nil
}

// GetOrganization retrieves an organization by ID
func GetOrganization(orgID int) (*Organization, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	org := &Organization{}
	query := `SELECT id, name, slug, '' as industry, '' as size, '' as country, description, 0 as owner_id, created_at, updated_at
			  FROM organizations WHERE id = $1`

	err := db.QueryRow(query, orgID).Scan(
		&org.ID, &org.Name, &org.Slug, &org.Industry, &org.Size,
		&org.Country, &org.Description, &org.OwnerID, &org.CreatedAt, &org.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("organization not found: %w", err)
	}

	return org, nil
}

// UpdateOrganization updates an organization
func UpdateOrganization(orgID int, req *OrganizationUpdateRequest) (*Organization, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Build dynamic update query
	updates := []string{}
	args := []interface{}{}

	if req.Name != "" {
		updates = append(updates, "name = ?")
		args = append(args, req.Name)
	}
	if req.Industry != "" {
		updates = append(updates, "industry = ?")
		args = append(args, req.Industry)
	}
	if req.Size != "" {
		updates = append(updates, "size = ?")
		args = append(args, req.Size)
	}
	if req.Country != "" {
		updates = append(updates, "country = ?")
		args = append(args, req.Country)
	}
	if req.Description != "" {
		updates = append(updates, "description = ?")
		args = append(args, req.Description)
	}

	if len(updates) == 0 {
		return GetOrganization(orgID)
	}

	updates = append(updates, "updated_at = CURRENT_TIMESTAMP")
	args = append(args, orgID)

	query := fmt.Sprintf("UPDATE organizations SET %s WHERE id = ?", strings.Join(updates, ", "))
	_, err := db.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to update organization: %w", err)
	}

	return GetOrganization(orgID)
}

// generateSlug creates a URL-friendly slug from organization name
func generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")
	// Remove any non-alphanumeric characters except hyphens
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	slug = result.String()
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

// RegisterUser creates a new user account with organization context
func RegisterUser(req *RegisterRequest) (*User, *Organization, error) {
	if db == nil {
		return nil, nil, fmt.Errorf("database not initialized")
	}

	// Validate email format (basic check)
	if !strings.Contains(req.Email, "@") || len(req.Email) < 5 {
		return nil, nil, fmt.Errorf("invalid email format")
	}

	// Validate password strength
	if len(req.Password) < 8 {
		return nil, nil, fmt.Errorf("password must be at least 8 characters long")
	}

	// Check if user already exists
	var existingID int
	err := db.QueryRow("SELECT id FROM users WHERE email = ?", req.Email).Scan(&existingID)
	if err == nil {
		return nil, nil, fmt.Errorf("user with this email already exists")
	}

	// Hash password
	hashedPassword, err := hashPassword(req.Password)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to hash password: %w", err)
	}

	var organization *Organization
	var organizationID int

	// Handle organization logic
	if req.OrganizationID != nil {
		// User is joining existing organization
		org, err := GetOrganization(*req.OrganizationID)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid organization: %w", err)
		}
		organization = org
		organizationID = org.ID
	} else {
		// Create new organization
		orgName := req.Organization
		if orgName == "" {
			// Extract organization from email domain
			emailParts := strings.Split(req.Email, "@")
			if len(emailParts) > 1 {
				orgName = strings.Title(strings.Split(emailParts[1], ".")[0])
			} else {
				orgName = "My Organization"
			}
		}

		// Create organization first (temporary user ID 0, will update later)
		orgReq := &OrganizationCreateRequest{
			Name:        orgName,
			Industry:    "",
			Size:        "",
			Country:     "",
			Description: "",
		}

		// We need to create a temporary organization first, then update owner_id
		tempOrg, err := CreateOrganization(orgReq, 0)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create organization: %w", err)
		}
		organization = tempOrg
		organizationID = tempOrg.ID
	}

	// Determine role - if creating new org, user is admin
	userRole := "user"
	if req.OrganizationID == nil {
		userRole = "admin"
	}

	// Insert user
	query := `INSERT INTO users (email, password, name, role, organization_id, active)
			  VALUES (?, ?, ?, ?, ?, ?) RETURNING id, created_at, updated_at`

	var user User
	user.Email = req.Email
	user.Name = req.Name
	user.Role = userRole
	user.OrganizationID = organizationID
	user.Active = true

	err = db.QueryRow(query, req.Email, hashedPassword, req.Name, userRole, organizationID, true).
		Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create user: %w", err)
	}

	// If we created a new organization, update the owner_id
	if req.OrganizationID == nil {
		_, err = db.Exec("UPDATE organizations SET owner_id = ? WHERE id = ?", user.ID, organizationID)
		if err != nil {
			log.Printf("Warning: Failed to update organization owner: %v", err)
		} else {
			organization.OwnerID = user.ID
		}
	}

	log.Printf("New user registered: %s (ID: %d, Org: %d)", user.Email, user.ID, organizationID)
	return &user, organization, nil
}

// LoginUser authenticates a user and returns a JWT token
func LoginUser(req *LoginRequest, clientIP, userAgent string) (*LoginResponse, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Get user by email
	user, err := getUserByEmail(req.Email)
	if err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}

	// Check if user is active
	if !user.Active {
		return nil, fmt.Errorf("user account is disabled")
	}

	// Verify password
	if !verifyPassword(user.Password, req.Password) {
		return nil, fmt.Errorf("invalid email or password")
	}

	// Get user's organization
	organization, err := GetOrganization(user.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to load organization: %w", err)
	}

	// Set organization in user object
	user.Organization = organization

	// Generate JWT token with organization context
	token, expiresAt, err := generateJWTToken(user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Update last login
	db.Exec("UPDATE users SET last_login = CURRENT_TIMESTAMP WHERE id = ?", user.ID)

	log.Printf("User logged in: %s (ID: %d, Org: %d)", user.Email, user.ID, user.OrganizationID)

	return &LoginResponse{
		Token:        token,
		ExpiresAt:    expiresAt,
		User:         user,
		Organization: organization,
	}, nil
}

// ValidateToken validates a JWT token and returns the user (simplified)
func ValidateToken(tokenString string) (*User, error) {
	if db == nil {
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
	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid user ID in token")
	}
	userID := int(userIDFloat)

	// Get organization ID
	orgIDFloat, ok := claims["organization_id"].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid organization ID in token")
	}
	organizationID := int(orgIDFloat)

	// Get user with organization context
	user, err := getUserByID(userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	if !user.Active {
		return nil, fmt.Errorf("user account is disabled")
	}

	// Verify user belongs to the organization in token
	if user.OrganizationID != organizationID {
		return nil, fmt.Errorf("invalid organization context")
	}

	return user, nil
}

// LogoutUser invalidates a user's session
func LogoutUser(tokenString string) error {
	// For JWT-only auth, there's nothing to invalidate server-side
	// In a real implementation, you'd maintain a blacklist
	return nil
}

// Helper functions

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func verifyPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

func generateJWTToken(user *User) (string, time.Time, error) {
	expiresAt := time.Now().Add(24 * time.Hour) // 24 hour expiry

	claims := jwt.MapClaims{
		"user_id":         user.ID,
		"email":           user.Email,
		"role":            user.Role,
		"organization_id": user.OrganizationID,
		"exp":             expiresAt.Unix(),
		"iat":             time.Now().Unix(),
	}

	// Add organization slug if available
	if user.Organization != nil {
		claims["organization_slug"] = user.Organization.Slug
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiresAt, nil
}

func getUserByEmail(email string) (*User, error) {
	user := &User{}
	query := `SELECT id, email, password_hash, COALESCE(first_name || ' ' || last_name, first_name, last_name, '') as name, role, organization_id, is_active, created_at, updated_at, last_login
			  FROM users WHERE email = $1 AND is_active = true`

	var lastLogin sql.NullTime
	err := db.QueryRow(query, email).Scan(
		&user.ID, &user.Email, &user.Password, &user.Name, &user.Role,
		&user.OrganizationID, &user.Active, &user.CreatedAt, &user.UpdatedAt, &lastLogin,
	)

	if err != nil {
		return nil, err
	}

	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	return user, nil
}

func getUserByID(id int) (*User, error) {
	user := &User{}
	query := `SELECT id, email, password_hash, COALESCE(first_name || ' ' || last_name, first_name, last_name, '') as name, role, organization_id, is_active, created_at, updated_at, last_login
			  FROM users WHERE id = $1 AND is_active = true`

	var lastLogin sql.NullTime
	err := db.QueryRow(query, id).Scan(
		&user.ID, &user.Email, &user.Password, &user.Name, &user.Role,
		&user.OrganizationID, &user.Active, &user.CreatedAt, &user.UpdatedAt, &lastLogin,
	)

	if err != nil {
		return nil, err
	}

	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	return user, nil
}

// GetUserProfile returns a user profile without sensitive information
func GetUserProfile(userID int) (*UserProfile, error) {
	user, err := getUserByID(userID)
	if err != nil {
		return nil, err
	}

	// Get organization details
	organization, err := GetOrganization(user.OrganizationID)
	if err != nil {
		log.Printf("Warning: Failed to load organization for user %d: %v", userID, err)
		// Continue without organization data
	}

	return &UserProfile{
		ID:             user.ID,
		Email:          user.Email,
		Name:           user.Name,
		Role:           user.Role,
		OrganizationID: user.OrganizationID,
		Active:         user.Active,
		CreatedAt:      user.CreatedAt,
		LastLogin:      user.LastLogin,
		Organization:   organization,
	}, nil
}
