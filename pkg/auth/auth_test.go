package auth

import (
	"database/sql"
	"log"
	"os"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// setupTestDB creates an in-memory SQLite database for testing.
// Tables are created by Initialize() via migrations, not here.
func setupTestDB() (*sql.DB, error) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	return testDB, nil
}

func TestMain(m *testing.M) {
	// Set up test database
	testDB, err := setupTestDB()
	if err != nil {
		log.Fatalf("Failed to set up test database: %v", err)
	}
	defer testDB.Close()

	// Initialize auth with test database
	if err := Initialize(testDB); err != nil {
		log.Fatalf("Failed to initialize auth: %v", err)
	}

	// Run tests
	code := m.Run()

	// Clean up
	testDB.Close()
	os.Exit(code)
}

func TestRegisterUser_NewOrganization(t *testing.T) {
	req := &SignupRequest{
		Email:        "john.doe@example.com",
		Password:     "securepassword123",
		FirstName:    "John",
		LastName:     "Doe",
		Organization: "Example Corp",
	}

	user, org, err := RegisterUser(req)
	if err != nil {
		t.Fatalf("Failed to register user: %v", err)
	}

	// Verify user data
	if user.Email != req.Email {
		t.Errorf("Expected email %s, got %s", req.Email, user.Email)
	}

	if user.FirstName != req.FirstName {
		t.Errorf("Expected first name %s, got %s", req.FirstName, user.FirstName)
	}

	if user.LastName != req.LastName {
		t.Errorf("Expected last name %s, got %s", req.LastName, user.LastName)
	}

	if user.Role != RoleOwner {
		t.Errorf("Expected role %s, got %s", RoleOwner, user.Role)
	}

	if !user.IsActive {
		t.Error("Expected user to be active")
	}

	if user.EmailVerified {
		t.Error("Expected user email to not be verified initially")
	}

	// Verify organization data
	if org.Name != req.Organization {
		t.Errorf("Expected organization name %s, got %s", req.Organization, org.Name)
	}

	if org.OwnerID != user.ID {
		t.Errorf("Expected organization owner ID to be user ID")
	}

	if !org.IsActive {
		t.Error("Expected organization to be active")
	}

	// Verify user belongs to the organization
	if user.OrganizationID != org.ID {
		t.Error("Expected user to belong to the created organization")
	}

	// Verify slug generation
	expectedSlug := "example-corp"
	if org.Slug != expectedSlug {
		t.Errorf("Expected slug %s, got %s", expectedSlug, org.Slug)
	}
}

func TestRegisterUser_ExistingOrganization(t *testing.T) {
	// First, create an organization with an owner
	ownerReq := &SignupRequest{
		Email:        "owner@company.com",
		Password:     "ownerpassword123",
		FirstName:    "Owner",
		LastName:     "Person",
		Organization: "Test Company",
	}

	owner, org, err := RegisterUser(ownerReq)
	if err != nil {
		t.Fatalf("Failed to register owner: %v", err)
	}

	// Now register a user to join the existing organization
	userReq := &SignupRequest{
		Email:          "user@company.com",
		Password:       "userpassword123",
		FirstName:      "Regular",
		LastName:       "User",
		OrganizationID: &org.ID,
	}

	user, joinedOrg, err := RegisterUser(userReq)
	if err != nil {
		t.Fatalf("Failed to register user to existing org: %v", err)
	}

	// Verify user data
	if user.Role != RoleUser {
		t.Errorf("Expected role %s, got %s", RoleUser, user.Role)
	}

	if user.OrganizationID != org.ID {
		t.Error("Expected user to belong to the existing organization")
	}

	// Verify organization data (should be the same)
	if joinedOrg.ID != org.ID {
		t.Error("Expected to join the existing organization")
	}

	if joinedOrg.OwnerID != owner.ID {
		t.Error("Expected organization owner to remain the same")
	}
}

func TestRegisterUser_DuplicateEmail(t *testing.T) {
	// Register a user first
	req1 := &SignupRequest{
		Email:        "duplicate@example.com",
		Password:     "password123",
		FirstName:    "First",
		LastName:     "User",
		Organization: "First Corp",
	}

	_, _, err := RegisterUser(req1)
	if err != nil {
		t.Fatalf("Failed to register first user: %v", err)
	}

	// Try to register another user with the same email
	req2 := &SignupRequest{
		Email:        "duplicate@example.com",
		Password:     "password456",
		FirstName:    "Second",
		LastName:     "User",
		Organization: "Second Corp",
	}

	_, _, err = RegisterUser(req2)
	if err == nil {
		t.Fatal("Expected error for duplicate email")
	}

	expectedError := "user with this email already exists, please login instead"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error message to contain '%s', got '%s'", expectedError, err.Error())
	}
}

func TestRegisterUser_Validation(t *testing.T) {
	tests := []struct {
		name        string
		req         *SignupRequest
		expectedErr string
	}{
		{
			name: "invalid email",
			req: &SignupRequest{
				Email:     "invalid-email",
				Password:  "password123",
				FirstName: "John",
				LastName:  "Doe",
			},
			expectedErr: "invalid email format",
		},
		{
			name: "short password",
			req: &SignupRequest{
				Email:     "test@example.com",
				Password:  "short",
				FirstName: "John",
				LastName:  "Doe",
			},
			expectedErr: "password must be at least 8 characters long",
		},
		{
			name: "missing first name",
			req: &SignupRequest{
				Email:     "test@example.com",
				Password:  "password123",
				FirstName: "",
				LastName:  "Doe",
			},
			expectedErr: "first name and last name are required",
		},
		{
			name: "missing last name",
			req: &SignupRequest{
				Email:     "test@example.com",
				Password:  "password123",
				FirstName: "John",
				LastName:  "",
			},
			expectedErr: "first name and last name are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := RegisterUser(tt.req)
			if err == nil {
				t.Fatalf("Expected error for %s", tt.name)
			}
			if !strings.Contains(err.Error(), tt.expectedErr) {
				t.Errorf("Expected error to contain '%s', got '%s'", tt.expectedErr, err.Error())
			}
		})
	}
}

func TestLoginUser(t *testing.T) {
	// Register a user first
	signupReq := &SignupRequest{
		Email:        "login.test@example.com",
		Password:     "loginpassword123",
		FirstName:    "Login",
		LastName:     "Test",
		Organization: "Login Corp",
	}

	registeredUser, _, err := RegisterUser(signupReq)
	if err != nil {
		t.Fatalf("Failed to register user: %v", err)
	}

	// Test successful login
	loginReq := &LoginRequest{
		Email:    signupReq.Email,
		Password: signupReq.Password,
	}

	response, err := LoginUser(loginReq, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("Failed to login: %v", err)
	}

	// Verify response
	if response.Token == "" {
		t.Error("Expected token to be generated")
	}

	if response.User.ID != registeredUser.ID {
		t.Error("Expected user ID to match")
	}

	if response.User.Email != signupReq.Email {
		t.Errorf("Expected email %s, got %s", signupReq.Email, response.User.Email)
	}

	if response.Organization == nil {
		t.Error("Expected organization to be included")
	}

	// Test invalid password
	invalidReq := &LoginRequest{
		Email:    signupReq.Email,
		Password: "wrongpassword",
	}

	_, err = LoginUser(invalidReq, "127.0.0.1", "test-agent")
	if err == nil {
		t.Fatal("Expected error for invalid password")
	}

	if !strings.Contains(err.Error(), "invalid email or password") {
		t.Errorf("Expected 'invalid email or password' error, got %s", err.Error())
	}

	// Test non-existent email
	nonExistentReq := &LoginRequest{
		Email:    "nonexistent@example.com",
		Password: "password123",
	}

	_, err = LoginUser(nonExistentReq, "127.0.0.1", "test-agent")
	if err == nil {
		t.Fatal("Expected error for non-existent email")
	}

	if !strings.Contains(err.Error(), "invalid email or password") {
		t.Errorf("Expected 'invalid email or password' error, got %s", err.Error())
	}
}

func TestValidateToken(t *testing.T) {
	// Register and login a user first
	signupReq := &SignupRequest{
		Email:        "token.test@example.com",
		Password:     "tokenpassword123",
		FirstName:    "Token",
		LastName:     "Test",
		Organization: "Token Corp",
	}

	registeredUser, _, err := RegisterUser(signupReq)
	if err != nil {
		t.Fatalf("Failed to register user: %v", err)
	}

	loginReq := &LoginRequest{
		Email:    signupReq.Email,
		Password: signupReq.Password,
	}

	response, err := LoginUser(loginReq, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("Failed to login: %v", err)
	}

	// Test token validation
	user, err := ValidateToken(response.Token)
	if err != nil {
		t.Fatalf("Failed to validate token: %v", err)
	}

	if user.ID != registeredUser.ID {
		t.Error("Expected user ID to match")
	}

	if user.Email != signupReq.Email {
		t.Errorf("Expected email %s, got %s", signupReq.Email, user.Email)
	}

	// Test invalid token
	_, err = ValidateToken("invalid.token.here")
	if err == nil {
		t.Fatal("Expected error for invalid token")
	}
}

func TestGetUserProfile(t *testing.T) {
	// Register a user first
	signupReq := &SignupRequest{
		Email:        "profile.test@example.com",
		Password:     "profilepassword123",
		FirstName:    "Profile",
		LastName:     "Test",
		Organization: "Profile Corp",
	}

	registeredUser, _, err := RegisterUser(signupReq)
	if err != nil {
		t.Fatalf("Failed to register user: %v", err)
	}

	// Get user profile
	profile, err := GetUserProfile(registeredUser.ID)
	if err != nil {
		t.Fatalf("Failed to get user profile: %v", err)
	}

	// Verify profile data
	if profile.ID != registeredUser.ID {
		t.Error("Expected profile ID to match")
	}

	if profile.Email != signupReq.Email {
		t.Errorf("Expected email %s, got %s", signupReq.Email, profile.Email)
	}

	if profile.FullName != "Profile Test" {
		t.Errorf("Expected full name 'Profile Test', got %s", profile.FullName)
	}

	if profile.Organization == nil {
		t.Error("Expected organization to be included in profile")
	}

	// Test non-existent user
	_, err = GetUserProfile(999999) // Non-existent ID
	if err == nil {
		t.Fatal("Expected error for non-existent user")
	}
}

func TestSlugGeneration(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Test Company", "test-company"},
		{"My_Awesome Corp!", "my-awesome-corp"},
		{"Special-Characters@#$%", "special-characters"},
		{"   Trimmed Spaces   ", "trimmed-spaces"},
		{"Multiple---Hyphens", "multiple-hyphens"},
		{"", "organization"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := generateSlug(tt.input)
			if result != tt.expected {
				t.Errorf("generateSlug(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPasswordHashing(t *testing.T) {
	password := "testpassword123"

	// Hash password
	hash, err := hashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	if hash == password {
		t.Error("Expected hash to be different from password")
	}

	// Verify correct password
	if !verifyPassword(hash, password) {
		t.Error("Expected password verification to succeed")
	}

	// Verify incorrect password
	if verifyPassword(hash, "wrongpassword") {
		t.Error("Expected password verification to fail for wrong password")
	}
}