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
	_ "modernc.org/sqlite"
)

var (
	db        *sql.DB
	jwtSecret []byte
)

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

	// Create auth tables
	if err := createAuthTables(); err != nil {
		return fmt.Errorf("failed to create auth tables: %w", err)
	}

	// Run migrations for schema updates
	if err := runAuthMigrations(); err != nil {
		return fmt.Errorf("failed to run auth migrations: %w", err)
	}

	// Create default admin user if none exists
	if err := createDefaultUser(); err != nil {
		log.Printf("Warning: Failed to create default user: %v", err)
	}

	log.Println("Authentication system initialized successfully")
	return nil
}

func createAuthTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			name TEXT NOT NULL,
			role TEXT DEFAULT 'user',
			active BOOLEAN DEFAULT true,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_login DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
		`CREATE INDEX IF NOT EXISTS idx_users_active ON users(active)`,
		`CREATE TABLE IF NOT EXISTS api_keys (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			key_hash TEXT NOT NULL,
			encrypted_key TEXT NOT NULL,
			masked_key TEXT NOT NULL,
			permissions TEXT NOT NULL,
			user_id TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME,
			status TEXT DEFAULT 'active',
			last_used DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id)`,
		`CREATE TABLE IF NOT EXISTS provider_configs (
			user_id TEXT PRIMARY KEY,
			config_data TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS organizations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			industry TEXT,
			size TEXT,
			country TEXT,
			description TEXT,
			owner_id INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (owner_id) REFERENCES users (id)
		)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query %s: %w", query, err)
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

// RegisterUser creates a new user account
func RegisterUser(req *RegisterRequest) (*User, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Validate email format (basic check)
	if !strings.Contains(req.Email, "@") || len(req.Email) < 5 {
		return nil, fmt.Errorf("invalid email format")
	}

	// Validate password strength
	if len(req.Password) < 8 {
		return nil, fmt.Errorf("password must be at least 8 characters long")
	}

	// Check if user already exists
	var existingID int
	err := db.QueryRow("SELECT id FROM users WHERE email = ?", req.Email).Scan(&existingID)
	if err == nil {
		return nil, fmt.Errorf("user with this email already exists")
	}

	// Hash password
	hashedPassword, err := hashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Insert user
	query := `INSERT INTO users (email, password, name, role, active)
			  VALUES (?, ?, ?, ?, ?) RETURNING id, created_at, updated_at`

	var user User
	user.Email = req.Email
	user.Name = req.Name
	user.Role = "user"
	user.Active = true

	err = db.QueryRow(query, req.Email, hashedPassword, req.Name, "user", true).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	log.Printf("New user registered: %s (ID: %d)", user.Email, user.ID)
	return &user, nil
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

	// Generate JWT token
	token, expiresAt, err := generateJWTToken(user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Update last login
	db.Exec("UPDATE users SET last_login = CURRENT_TIMESTAMP WHERE id = ?", user.ID)

	log.Printf("User logged in: %s (ID: %d)", user.Email, user.ID)

	return &LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      user,
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

	// Get user directly - skip session validation for now
	user, err := getUserByID(userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	if !user.Active {
		return nil, fmt.Errorf("user account is disabled")
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
		"user_id": user.ID,
		"email":   user.Email,
		"role":    user.Role,
		"exp":     expiresAt.Unix(),
		"iat":     time.Now().Unix(),
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
	query := `SELECT id, email, password, name, role, active, created_at, updated_at, last_login
			  FROM users WHERE email = ? AND active = true`

	var lastLogin sql.NullTime
	err := db.QueryRow(query, email).Scan(
		&user.ID, &user.Email, &user.Password, &user.Name, &user.Role,
		&user.Active, &user.CreatedAt, &user.UpdatedAt, &lastLogin,
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
	query := `SELECT id, email, password, name, role, active, created_at, updated_at, last_login
			  FROM users WHERE id = ? AND active = true`

	var lastLogin sql.NullTime
	err := db.QueryRow(query, id).Scan(
		&user.ID, &user.Email, &user.Password, &user.Name, &user.Role,
		&user.Active, &user.CreatedAt, &user.UpdatedAt, &lastLogin,
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

	return &UserProfile{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Role:      user.Role,
		Active:    user.Active,
		CreatedAt: user.CreatedAt,
		LastLogin: user.LastLogin,
	}, nil
}
