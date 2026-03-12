package auth

import (
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	_ "modernc.org/sqlite"
)

// AuthTestSuite is the test suite for auth package
type AuthTestSuite struct {
	suite.Suite
	db *sql.DB
}

func (suite *AuthTestSuite) SetupTest() {
	// Create in-memory database for each test
	testDB, err := sql.Open("sqlite", ":memory:")
	suite.Require().NoError(err)

	suite.db = testDB

	// Initialize auth system with test database
	err = Initialize(testDB)
	suite.Require().NoError(err)
}

func (suite *AuthTestSuite) TearDownTest() {
	if suite.db != nil {
		suite.db.Close()
	}
}

func TestAuthSuite(t *testing.T) {
	suite.Run(t, new(AuthTestSuite))
}

// Test initialization
func (suite *AuthTestSuite) TestInitialize() {
	// Should create tables and default user
	var tableCount int
	err := suite.db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master
		WHERE type='table' AND name IN ('users', 'api_keys', 'provider_configs', 'organizations')
	`).Scan(&tableCount)

	suite.NoError(err)
	suite.Equal(4, tableCount, "All required tables should be created")

	// Should create default admin user
	var userCount int
	err = suite.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
	suite.NoError(err)
	suite.GreaterOrEqual(userCount, 1, "Default user should be created")
}

func (suite *AuthTestSuite) TestInitializeWithJWTSecret() {
	// Test with JWT secret from environment
	originalSecret := os.Getenv("JWT_SECRET")
	defer os.Setenv("JWT_SECRET", originalSecret)

	os.Setenv("JWT_SECRET", "test-secret-key")

	testDB, err := sql.Open("sqlite", ":memory:")
	suite.Require().NoError(err)
	defer testDB.Close()

	err = Initialize(testDB)
	suite.NoError(err)
	suite.Equal([]byte("test-secret-key"), jwtSecret)
}

func (suite *AuthTestSuite) TestRegisterUser() {
	tests := []struct {
		name      string
		request   *RegisterRequest
		shouldErr bool
		errMsg    string
	}{
		{
			name: "valid registration",
			request: &RegisterRequest{
				Email:    "test@example.com",
				Password: "password123",
				Name:     "Test User",
			},
			shouldErr: false,
		},
		{
			name: "duplicate email",
			request: &RegisterRequest{
				Email:    "admin@gateway.local", // Default admin user email
				Password: "password123",
				Name:     "Duplicate User",
			},
			shouldErr: true,
			errMsg:    "user with this email already exists",
		},
		{
			name: "invalid email",
			request: &RegisterRequest{
				Email:    "invalid-email",
				Password: "password123",
				Name:     "Test User",
			},
			shouldErr: true,
			errMsg:    "invalid email format",
		},
		{
			name: "short password",
			request: &RegisterRequest{
				Email:    "short@example.com",
				Password: "123",
				Name:     "Test User",
			},
			shouldErr: true,
			errMsg:    "password must be at least 8 characters long",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			user, err := RegisterUser(tt.request)

			if tt.shouldErr {
				suite.Error(err)
				suite.Contains(err.Error(), tt.errMsg)
				suite.Nil(user)
			} else {
				suite.NoError(err)
				suite.NotNil(user)
				suite.Equal(tt.request.Email, user.Email)
				suite.Equal(tt.request.Name, user.Name)
				suite.Equal("user", user.Role)
				suite.True(user.Active)
				suite.NotZero(user.ID)
			}
		})
	}
}

func (suite *AuthTestSuite) TestLoginUser() {
	// First register a test user
	registerReq := &RegisterRequest{
		Email:    "login@test.com",
		Password: "testpass123",
		Name:     "Login Test User",
	}

	user, err := RegisterUser(registerReq)
	suite.Require().NoError(err)
	suite.Require().NotNil(user)

	tests := []struct {
		name      string
		request   *LoginRequest
		shouldErr bool
		errMsg    string
	}{
		{
			name: "valid login",
			request: &LoginRequest{
				Email:    "login@test.com",
				Password: "testpass123",
			},
			shouldErr: false,
		},
		{
			name: "invalid email",
			request: &LoginRequest{
				Email:    "nonexistent@test.com",
				Password: "testpass123",
			},
			shouldErr: true,
			errMsg:    "invalid email or password",
		},
		{
			name: "invalid password",
			request: &LoginRequest{
				Email:    "login@test.com",
				Password: "wrongpassword",
			},
			shouldErr: true,
			errMsg:    "invalid email or password",
		},
		{
			name: "default admin login",
			request: &LoginRequest{
				Email:    "admin@gateway.local",
				Password: "admin123",
			},
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			response, err := LoginUser(tt.request, "127.0.0.1", "test-user-agent")

			if tt.shouldErr {
				suite.Error(err)
				suite.Contains(err.Error(), tt.errMsg)
				suite.Nil(response)
			} else {
				suite.NoError(err)
				suite.NotNil(response)
				suite.NotEmpty(response.Token)
				suite.NotNil(response.User)
				suite.Equal(tt.request.Email, response.User.Email)
				suite.True(response.ExpiresAt.After(time.Now()))
			}
		})
	}
}

func (suite *AuthTestSuite) TestLoginUserInactive() {
	// Register and then deactivate a user
	registerReq := &RegisterRequest{
		Email:    "inactive@test.com",
		Password: "testpass123",
		Name:     "Inactive User",
	}

	user, err := RegisterUser(registerReq)
	suite.Require().NoError(err)

	// Deactivate user
	_, err = suite.db.Exec("UPDATE users SET active = false WHERE id = ?", user.ID)
	suite.Require().NoError(err)

	loginReq := &LoginRequest{
		Email:    "inactive@test.com",
		Password: "testpass123",
	}

	response, err := LoginUser(loginReq, "127.0.0.1", "test-user-agent")
	suite.Error(err)
	suite.Contains(err.Error(), "user account is disabled")
	suite.Nil(response)
}

func (suite *AuthTestSuite) TestValidateToken() {
	// Register and login a user to get a valid token
	registerReq := &RegisterRequest{
		Email:    "token@test.com",
		Password: "testpass123",
		Name:     "Token Test User",
	}

	user, err := RegisterUser(registerReq)
	suite.Require().NoError(err)

	loginReq := &LoginRequest{
		Email:    "token@test.com",
		Password: "testpass123",
	}

	loginResp, err := LoginUser(loginReq, "127.0.0.1", "test-user-agent")
	suite.Require().NoError(err)
	suite.Require().NotNil(loginResp)

	tests := []struct {
		name      string
		token     string
		shouldErr bool
		errMsg    string
	}{
		{
			name:      "valid token",
			token:     loginResp.Token,
			shouldErr: false,
		},
		{
			name:      "invalid token",
			token:     "invalid.token.here",
			shouldErr: true,
			errMsg:    "invalid token",
		},
		{
			name:      "empty token",
			token:     "",
			shouldErr: true,
			errMsg:    "invalid token",
		},
		{
			name:      "malformed token",
			token:     "not.a.jwt",
			shouldErr: true,
			errMsg:    "invalid token",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			validatedUser, err := ValidateToken(tt.token)

			if tt.shouldErr {
				suite.Error(err)
				suite.Contains(err.Error(), tt.errMsg)
				suite.Nil(validatedUser)
			} else {
				suite.NoError(err)
				suite.NotNil(validatedUser)
				suite.Equal(user.ID, validatedUser.ID)
				suite.Equal(user.Email, validatedUser.Email)
			}
		})
	}
}

func (suite *AuthTestSuite) TestValidateTokenInactiveUser() {
	// Register and login a user
	registerReq := &RegisterRequest{
		Email:    "deactivate@test.com",
		Password: "testpass123",
		Name:     "Deactivate Test User",
	}

	user, err := RegisterUser(registerReq)
	suite.Require().NoError(err)

	loginReq := &LoginRequest{
		Email:    "deactivate@test.com",
		Password: "testpass123",
	}

	loginResp, err := LoginUser(loginReq, "127.0.0.1", "test-user-agent")
	suite.Require().NoError(err)

	// Deactivate user after getting token
	_, err = suite.db.Exec("UPDATE users SET active = false WHERE id = ?", user.ID)
	suite.Require().NoError(err)

	// Token should now be invalid
	validatedUser, err := ValidateToken(loginResp.Token)
	suite.Error(err)
	suite.Contains(err.Error(), "user account is disabled")
	suite.Nil(validatedUser)
}

func (suite *AuthTestSuite) TestGetUserProfile() {
	// Register a user
	registerReq := &RegisterRequest{
		Email:    "profile@test.com",
		Password: "testpass123",
		Name:     "Profile Test User",
	}

	user, err := RegisterUser(registerReq)
	suite.Require().NoError(err)

	// Test getting profile
	profile, err := GetUserProfile(user.ID)
	suite.NoError(err)
	suite.NotNil(profile)
	suite.Equal(user.ID, profile.ID)
	suite.Equal(user.Email, profile.Email)
	suite.Equal(user.Name, profile.Name)
	suite.Equal(user.Role, profile.Role)
	suite.Equal(user.Active, profile.Active)

	// Test non-existent user
	profile, err = GetUserProfile(99999)
	suite.Error(err)
	suite.Nil(profile)
}

func (suite *AuthTestSuite) TestHashAndVerifyPassword() {
	password := "testpassword123"

	// Test hashing
	hash, err := hashPassword(password)
	suite.NoError(err)
	suite.NotEmpty(hash)
	suite.NotEqual(password, hash)

	// Test verification
	suite.True(verifyPassword(hash, password))
	suite.False(verifyPassword(hash, "wrongpassword"))
}

func (suite *AuthTestSuite) TestGenerateJWTToken() {
	user := &User{
		ID:    1,
		Email: "test@example.com",
		Role:  "user",
	}

	token, expiresAt, err := generateJWTToken(user)
	suite.NoError(err)
	suite.NotEmpty(token)
	suite.True(expiresAt.After(time.Now()))

	// Token should contain user info
	suite.Contains(token, ".")
	parts := len(strings.Split(token, "."))
	suite.Equal(3, parts, "JWT should have 3 parts")
}

func (suite *AuthTestSuite) TestGetUserByEmail() {
	// Register a user first
	registerReq := &RegisterRequest{
		Email:    "getemail@test.com",
		Password: "testpass123",
		Name:     "Get Email Test User",
	}

	registeredUser, err := RegisterUser(registerReq)
	suite.Require().NoError(err)

	// Test getting user by email
	user, err := getUserByEmail("getemail@test.com")
	suite.NoError(err)
	suite.NotNil(user)
	suite.Equal(registeredUser.ID, user.ID)
	suite.Equal(registeredUser.Email, user.Email)

	// Test non-existent email
	user, err = getUserByEmail("nonexistent@test.com")
	suite.Error(err)
	suite.Nil(user)
}

func (suite *AuthTestSuite) TestGetUserByID() {
	// Register a user first
	registerReq := &RegisterRequest{
		Email:    "getid@test.com",
		Password: "testpass123",
		Name:     "Get ID Test User",
	}

	registeredUser, err := RegisterUser(registerReq)
	suite.Require().NoError(err)

	// Test getting user by ID
	user, err := getUserByID(registeredUser.ID)
	suite.NoError(err)
	suite.NotNil(user)
	suite.Equal(registeredUser.ID, user.ID)
	suite.Equal(registeredUser.Email, user.Email)

	// Test non-existent ID
	user, err = getUserByID(99999)
	suite.Error(err)
	suite.Nil(user)
}

func (suite *AuthTestSuite) TestLogoutUser() {
	// For JWT-only implementation, logout should always succeed
	err := LogoutUser("any.token.here")
	suite.NoError(err)
}

// Benchmark tests
func BenchmarkHashPassword(b *testing.B) {
	password := "testpassword123"
	for i := 0; i < b.N; i++ {
		_, _ = hashPassword(password)
	}
}

func BenchmarkVerifyPassword(b *testing.B) {
	password := "testpassword123"
	hash, _ := hashPassword(password)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = verifyPassword(hash, password)
	}
}

func BenchmarkGenerateJWTToken(b *testing.B) {
	user := &User{
		ID:    1,
		Email: "test@example.com",
		Role:  "user",
	}

	// Initialize to set jwtSecret
	testDB, _ := sql.Open("sqlite", ":memory:")
	defer testDB.Close()
	Initialize(testDB)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = generateJWTToken(user)
	}
}

func BenchmarkValidateToken(b *testing.B) {
	// Setup
	testDB, _ := sql.Open("sqlite", ":memory:")
	defer testDB.Close()
	Initialize(testDB)

	user := &User{ID: 1, Email: "test@example.com", Role: "user"}
	token, _, _ := generateJWTToken(user)

	// Register the user in DB so validation works
	RegisterUser(&RegisterRequest{
		Email:    "test@example.com",
		Password: "password123",
		Name:     "Test User",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ValidateToken(token)
	}
}