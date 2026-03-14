package auth

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	_ "modernc.org/sqlite"
)

type AuthNewTestSuite struct {
	suite.Suite
	testDB *sql.DB
}

func (suite *AuthNewTestSuite) SetupSuite() {
	var err error
	suite.testDB, err = sql.Open("sqlite", ":memory:")
	suite.Require().NoError(err)

	// Initialize auth system with test database
	err = Initialize(suite.testDB)
	suite.Require().NoError(err)

	// Create tables for testing
	suite.createTestTables()
}

func (suite *AuthNewTestSuite) TearDownSuite() {
	if suite.testDB != nil {
		suite.testDB.Close()
	}
}

func (suite *AuthNewTestSuite) SetupTest() {
	// Clean up data before each test
	suite.cleanupTestData()
}

func (suite *AuthNewTestSuite) createTestTables() {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS organizations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			slug TEXT NOT NULL UNIQUE,
			description TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			organization_id INTEGER NOT NULL,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			first_name TEXT,
			last_name TEXT,
			role TEXT NOT NULL DEFAULT 'user',
			is_active BOOLEAN DEFAULT true,
			email_verified BOOLEAN DEFAULT false,
			last_login DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (organization_id) REFERENCES organizations(id)
		)`,
		`CREATE TABLE IF NOT EXISTS user_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			session_token TEXT NOT NULL UNIQUE,
			expires_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_accessed DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
	}

	for _, table := range tables {
		_, err := suite.testDB.Exec(table)
		suite.Require().NoError(err)
	}
}

func (suite *AuthNewTestSuite) cleanupTestData() {
	tables := []string{"user_sessions", "users", "organizations"}
	for _, table := range tables {
		_, err := suite.testDB.Exec("DELETE FROM " + table)
		suite.Require().NoError(err)
	}
}

func (suite *AuthNewTestSuite) TestSignupUser_Success() {
	req := &RegisterRequest{
		Email:        "test@example.com",
		Password:     "password123",
		Name:         "Test User",
		Organization: "Test Company",
	}

	response, err := SignupUser(req, "127.0.0.1", "test-agent")

	suite.NoError(err)
	suite.NotNil(response)
	suite.NotEmpty(response.Token)
	suite.NotNil(response.User)
	suite.NotNil(response.Organization)
	suite.Equal("test@example.com", response.User.Email)
	suite.Equal("Test Company", response.Organization.Name)
	suite.Equal("admin", response.User.Role)
	suite.True(response.ExpiresAt.After(time.Now()))

	// Verify data was created atomically
	var userCount, orgCount int
	err = suite.testDB.QueryRow("SELECT COUNT(*) FROM users WHERE email = ?", req.Email).Scan(&userCount)
	suite.NoError(err)
	suite.Equal(1, userCount)

	err = suite.testDB.QueryRow("SELECT COUNT(*) FROM organizations WHERE name = ?", req.Organization).Scan(&orgCount)
	suite.NoError(err)
	suite.Equal(1, orgCount)
}

func (suite *AuthNewTestSuite) TestSignupUser_DuplicateEmail() {
	req := &RegisterRequest{
		Email:        "duplicate@example.com",
		Password:     "password123",
		Name:         "First User",
		Organization: "First Company",
	}

	// First signup should succeed
	_, err := SignupUser(req, "127.0.0.1", "test-agent")
	suite.NoError(err)

	// Second signup with same email should fail
	req2 := &RegisterRequest{
		Email:        "duplicate@example.com",
		Password:     "password456",
		Name:         "Second User",
		Organization: "Second Company",
	}

	_, err = SignupUser(req2, "127.0.0.1", "test-agent")
	suite.Error(err)

	// Verify only one organization was created (atomic rollback)
	var orgCount int
	err = suite.testDB.QueryRow("SELECT COUNT(*) FROM organizations").Scan(&orgCount)
	suite.NoError(err)
	suite.Equal(1, orgCount, "Should only have one organization after failed duplicate signup")
}

func (suite *AuthNewTestSuite) TestSignupUser_ValidationErrors() {
	testCases := []struct {
		name string
		req  *RegisterRequest
	}{
		{
			name: "missing email",
			req: &RegisterRequest{
				Password: "password123",
				Name:     "Test User",
			},
		},
		{
			name: "missing password",
			req: &RegisterRequest{
				Email: "test@example.com",
				Name:  "Test User",
			},
		},
		{
			name: "short password",
			req: &RegisterRequest{
				Email:    "test@example.com",
				Password: "123",
				Name:     "Test User",
			},
		},
		{
			name: "missing name",
			req: &RegisterRequest{
				Email:    "test@example.com",
				Password: "password123",
			},
		},
		{
			name: "invalid email",
			req: &RegisterRequest{
				Email:    "invalid-email",
				Password: "password123",
				Name:     "Test User",
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			_, err := SignupUser(tc.req, "127.0.0.1", "test-agent")
			suite.Error(err)
		})
	}
}

func (suite *AuthNewTestSuite) TestAuthenticateUser_Success() {
	// First create a user
	signupReq := &RegisterRequest{
		Email:        "auth@example.com",
		Password:     "password123",
		Name:         "Auth User",
		Organization: "Auth Company",
	}

	signupResp, err := SignupUser(signupReq, "127.0.0.1", "test-agent")
	suite.NoError(err)

	// Now authenticate
	loginReq := &LoginRequest{
		Email:    "auth@example.com",
		Password: "password123",
	}

	response, err := AuthenticateUser(loginReq, "127.0.0.1", "test-agent")

	suite.NoError(err)
	suite.NotNil(response)
	suite.NotEmpty(response.Token)
	suite.NotNil(response.User)
	suite.NotNil(response.Organization)
	suite.Equal("auth@example.com", response.User.Email)
	suite.Equal(signupResp.Organization.ID, response.Organization.ID)
}

func (suite *AuthNewTestSuite) TestAuthenticateUser_InvalidCredentials() {
	// Create a user first
	signupReq := &RegisterRequest{
		Email:        "valid@example.com",
		Password:     "correctpassword",
		Name:         "Valid User",
		Organization: "Valid Company",
	}

	_, err := SignupUser(signupReq, "127.0.0.1", "test-agent")
	suite.NoError(err)

	testCases := []struct {
		name string
		req  *LoginRequest
	}{
		{
			name: "wrong password",
			req: &LoginRequest{
				Email:    "valid@example.com",
				Password: "wrongpassword",
			},
		},
		{
			name: "nonexistent email",
			req: &LoginRequest{
				Email:    "nonexistent@example.com",
				Password: "password123",
			},
		},
		{
			name: "missing email",
			req: &LoginRequest{
				Password: "password123",
			},
		},
		{
			name: "missing password",
			req: &LoginRequest{
				Email: "valid@example.com",
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			_, err := AuthenticateUser(tc.req, "127.0.0.1", "test-agent")
			suite.Error(err)
		})
	}
}

func (suite *AuthNewTestSuite) TestGenerateJWT() {
	userID := 1
	orgID := 2
	email := "test@example.com"
	role := "admin"

	token, expiresAt, err := generateJWT(userID, orgID, email, role)

	suite.NoError(err)
	suite.NotEmpty(token)
	suite.True(expiresAt.After(time.Now()))

	// Validate the token
	user, err := ValidateToken(token)
	suite.NoError(err)
	suite.Equal(userID, user.ID)
	suite.Equal(orgID, user.OrganizationID)
	suite.Equal(email, user.Email)
	suite.Equal(role, user.Role)
}

func (suite *AuthNewTestSuite) TestGenerateOrgSlug() {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple name",
			input:    "Test Company",
			expected: "test-company",
		},
		{
			name:     "special characters",
			input:    "Test & Company, Inc.",
			expected: "test-company-inc",
		},
		{
			name:     "long name",
			input:    "This Is A Very Long Company Name That Should Be Truncated",
			expected: "this-is-a-very-long-company-name-that-should-be-t",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result := generateOrgSlug(tc.input)
			suite.Equal(tc.expected, result)
		})
	}
}

func (suite *AuthNewTestSuite) TestSplitName() {
	testCases := []struct {
		input     string
		firstName string
		lastName  string
	}{
		{"John Doe", "John", "Doe"},
		{"John", "John", ""},
		{"John Michael Doe", "John", "Michael Doe"},
		{"", "", ""},
		{"   ", "", ""},
	}

	for _, tc := range testCases {
		firstName, lastName := splitName(tc.input)
		suite.Equal(tc.firstName, firstName)
		suite.Equal(tc.lastName, lastName)
	}
}

func (suite *AuthNewTestSuite) TestOrganizationIsolation() {
	// Create two users in different organizations
	user1Req := &RegisterRequest{
		Email:        "user1@company1.com",
		Password:     "password123",
		Name:         "User One",
		Organization: "Company One",
	}

	user2Req := &RegisterRequest{
		Email:        "user2@company2.com",
		Password:     "password123",
		Name:         "User Two",
		Organization: "Company Two",
	}

	resp1, err := SignupUser(user1Req, "127.0.0.1", "test-agent")
	suite.NoError(err)

	resp2, err := SignupUser(user2Req, "127.0.0.1", "test-agent")
	suite.NoError(err)

	// Verify they have different organization IDs
	suite.NotEqual(resp1.Organization.ID, resp2.Organization.ID)
	suite.Equal(resp1.User.OrganizationID, resp1.Organization.ID)
	suite.Equal(resp2.User.OrganizationID, resp2.Organization.ID)
}

func TestAuthNewSuite(t *testing.T) {
	suite.Run(t, new(AuthNewTestSuite))
}

// Additional helper tests
func TestValidateSignupRequest(t *testing.T) {
	testCases := []struct {
		name        string
		req         *RegisterRequest
		shouldError bool
	}{
		{
			name: "valid request",
			req: &RegisterRequest{
				Email:    "test@example.com",
				Password: "password123",
				Name:     "Test User",
			},
			shouldError: false,
		},
		{
			name: "missing email",
			req: &RegisterRequest{
				Password: "password123",
				Name:     "Test User",
			},
			shouldError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSignupRequest(tc.req)
			if tc.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}