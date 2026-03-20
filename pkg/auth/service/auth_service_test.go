package service

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ContinuumSolutions/nonym/pkg/auth/config"
	"github.com/ContinuumSolutions/nonym/pkg/auth/errors"
	"github.com/ContinuumSolutions/nonym/pkg/auth/interfaces"
	"github.com/ContinuumSolutions/nonym/pkg/auth/models"
	"github.com/ContinuumSolutions/nonym/pkg/auth/security"
	"github.com/ContinuumSolutions/nonym/pkg/auth/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock implementations for testing
type mockRepo struct {
	mock.Mock
}

func (m *mockRepo) CreateUser(ctx context.Context, req *models.CreateUserRequest) (*models.User, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *mockRepo) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *mockRepo) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	args := m.Called(ctx, email)
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *mockRepo) UpdateUser(ctx context.Context, id uuid.UUID, updates *models.UpdateUserRequest) (*models.User, error) {
	args := m.Called(ctx, id, updates)
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *mockRepo) DeleteUser(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockRepo) ListUsers(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*models.User, int, error) {
	args := m.Called(ctx, orgID, limit, offset)
	return args.Get(0).([]*models.User), args.Int(1), args.Error(2)
}

func (m *mockRepo) SetUserPassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	args := m.Called(ctx, userID, passwordHash)
	return args.Error(0)
}

func (m *mockRepo) UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *mockRepo) SetEmailVerified(ctx context.Context, userID uuid.UUID, verified bool) error {
	args := m.Called(ctx, userID, verified)
	return args.Error(0)
}

func (m *mockRepo) CreateOrganization(ctx context.Context, req *models.CreateOrganizationRequest) (*models.Organization, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*models.Organization), args.Error(1)
}

func (m *mockRepo) GetOrganizationByID(ctx context.Context, id uuid.UUID) (*models.Organization, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*models.Organization), args.Error(1)
}

func (m *mockRepo) GetOrganizationBySlug(ctx context.Context, slug string) (*models.Organization, error) {
	args := m.Called(ctx, slug)
	return args.Get(0).(*models.Organization), args.Error(1)
}

func (m *mockRepo) UpdateOrganization(ctx context.Context, id uuid.UUID, updates *models.UpdateOrganizationRequest) (*models.Organization, error) {
	args := m.Called(ctx, id, updates)
	return args.Get(0).(*models.Organization), args.Error(1)
}

func (m *mockRepo) DeleteOrganization(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockRepo) ListOrganizations(ctx context.Context, limit, offset int) ([]*models.Organization, int, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]*models.Organization), args.Int(1), args.Error(2)
}

func (m *mockRepo) GetOrganizationMembers(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*models.User, int, error) {
	args := m.Called(ctx, orgID, limit, offset)
	return args.Get(0).([]*models.User), args.Int(1), args.Error(2)
}

func (m *mockRepo) GetOrganizationStats(ctx context.Context, orgID uuid.UUID) (*models.OrganizationStats, error) {
	args := m.Called(ctx, orgID)
	return args.Get(0).(*models.OrganizationStats), args.Error(1)
}

func (m *mockRepo) CreateSession(ctx context.Context, session *interfaces.UserSession) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *mockRepo) GetSession(ctx context.Context, sessionID uuid.UUID) (*interfaces.UserSession, error) {
	args := m.Called(ctx, sessionID)
	return args.Get(0).(*interfaces.UserSession), args.Error(1)
}

func (m *mockRepo) DeleteSession(ctx context.Context, sessionID uuid.UUID) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *mockRepo) DeleteUserSessions(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *mockRepo) DeleteExpiredSessions(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *mockRepo) UpdateSessionActivity(ctx context.Context, sessionID uuid.UUID) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *mockRepo) CreateRefreshToken(ctx context.Context, token *interfaces.RefreshToken) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *mockRepo) GetRefreshToken(ctx context.Context, tokenHash string) (*interfaces.RefreshToken, error) {
	args := m.Called(ctx, tokenHash)
	return args.Get(0).(*interfaces.RefreshToken), args.Error(1)
}

func (m *mockRepo) DeleteRefreshToken(ctx context.Context, tokenHash string) error {
	args := m.Called(ctx, tokenHash)
	return args.Error(0)
}

func (m *mockRepo) DeleteUserRefreshTokens(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *mockRepo) DeleteExpiredRefreshTokens(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *mockRepo) LogAuthEvent(ctx context.Context, event *interfaces.AuthEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *mockRepo) GetAuthEvents(ctx context.Context, filter *interfaces.AuthEventFilter) ([]*interfaces.AuthEvent, int, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]*interfaces.AuthEvent), args.Int(1), args.Error(2)
}

func (m *mockRepo) CreatePasswordReset(ctx context.Context, reset *interfaces.PasswordReset) error {
	args := m.Called(ctx, reset)
	return args.Error(0)
}

func (m *mockRepo) GetPasswordReset(ctx context.Context, token string) (*interfaces.PasswordReset, error) {
	args := m.Called(ctx, token)
	return args.Get(0).(*interfaces.PasswordReset), args.Error(1)
}

func (m *mockRepo) DeletePasswordReset(ctx context.Context, token string) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *mockRepo) DeleteExpiredPasswordResets(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *mockRepo) WithTx(ctx context.Context, fn func(interfaces.AuthRepository) error) error {
	// For testing purposes, just call the function with the same mock
	return fn(m)
}

func (m *mockRepo) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockRepo) Close() error {
	args := m.Called()
	return args.Error(0)
}

// Mock password hasher
type mockHasher struct {
	mock.Mock
}

func (m *mockHasher) Hash(password string) (string, error) {
	args := m.Called(password)
	return args.String(0), args.Error(1)
}

func (m *mockHasher) Verify(password, hash string) error {
	args := m.Called(password, hash)
	return args.Error(0)
}

func (m *mockHasher) NeedsRehash(hash string) bool {
	args := m.Called(hash)
	return args.Bool(0)
}

// Mock token generator
type mockTokenGen struct {
	mock.Mock
}

func (m *mockTokenGen) GenerateAccessToken(user *models.User) (string, error) {
	args := m.Called(user)
	return args.String(0), args.Error(1)
}

func (m *mockTokenGen) GenerateRefreshToken(user *models.User) (string, error) {
	args := m.Called(user)
	return args.String(0), args.Error(1)
}

func (m *mockTokenGen) ValidateAccessToken(token string) (*interfaces.TokenClaims, error) {
	args := m.Called(token)
	return args.Get(0).(*interfaces.TokenClaims), args.Error(1)
}

func (m *mockTokenGen) ValidateRefreshToken(token string) (*interfaces.TokenClaims, error) {
	args := m.Called(token)
	return args.Get(0).(*interfaces.TokenClaims), args.Error(1)
}

func (m *mockTokenGen) RevokeToken(token string) error {
	args := m.Called(token)
	return args.Error(0)
}

// Mock audit logger
type mockAuditLogger struct {
	mock.Mock
}

func (m *mockAuditLogger) LogAuthEvent(ctx context.Context, event *interfaces.AuthEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *mockAuditLogger) LogSecurityEvent(ctx context.Context, userID *uuid.UUID, orgID *uuid.UUID, eventType string, details map[string]interface{}) error {
	args := m.Called(ctx, userID, orgID, eventType, details)
	return args.Error(0)
}

// Mock rate limiter
type mockRateLimiter struct {
	mock.Mock
}

func (m *mockRateLimiter) AllowLogin(ctx context.Context, identifier string) (bool, error) {
	args := m.Called(ctx, identifier)
	return args.Bool(0), args.Error(1)
}

func (m *mockRateLimiter) AllowRegistration(ctx context.Context, identifier string) (bool, error) {
	args := m.Called(ctx, identifier)
	return args.Bool(0), args.Error(1)
}

func (m *mockRateLimiter) AllowPasswordReset(ctx context.Context, identifier string) (bool, error) {
	args := m.Called(ctx, identifier)
	return args.Bool(0), args.Error(1)
}

func (m *mockRateLimiter) RecordFailedLogin(ctx context.Context, identifier string) error {
	args := m.Called(ctx, identifier)
	return args.Error(0)
}

func (m *mockRateLimiter) ClearFailedLogins(ctx context.Context, identifier string) error {
	args := m.Called(ctx, identifier)
	return args.Error(0)
}

// Mock email service
type mockEmailService struct {
	mock.Mock
}

func (m *mockEmailService) SendWelcomeEmail(ctx context.Context, user *models.User, org *models.Organization) error {
	args := m.Called(ctx, user, org)
	return args.Error(0)
}

func (m *mockEmailService) SendPasswordResetEmail(ctx context.Context, user *models.User, resetToken string) error {
	args := m.Called(ctx, user, resetToken)
	return args.Error(0)
}

func (m *mockEmailService) SendEmailVerificationEmail(ctx context.Context, user *models.User, verificationToken string) error {
	args := m.Called(ctx, user, verificationToken)
	return args.Error(0)
}

func (m *mockEmailService) SendOrganizationInviteEmail(ctx context.Context, invite *interfaces.InviteRequest) error {
	args := m.Called(ctx, invite)
	return args.Error(0)
}

// Test helper to create auth service with all mocks
type testMocks struct {
	service     *authService
	repo        *mockRepo
	hasher      *mockHasher
	tokenGen    *mockTokenGen
	rateLimiter *mockRateLimiter
	audit       *mockAuditLogger
	email       *mockEmailService
}

func createTestAuthServiceWithMocks() *testMocks {
	repo := &mockRepo{}
	hasher := &mockHasher{}
	tokenGen := &mockTokenGen{}
	validator := validation.New()
	audit := &mockAuditLogger{}
	rateLimiter := &mockRateLimiter{}
	email := &mockEmailService{}
	config := &config.Config{
		JWT: config.JWTConfig{
			SecretKey:            "test-secret-key-at-least-32-characters-long",
			AccessTokenExpiry:    15 * time.Minute,
			RefreshTokenExpiry:   24 * time.Hour,
			Issuer:              "test-issuer",
			Audience:            "test-audience",
			RefreshTokenRotation: false,
		},
	}

	service := &authService{
		repo:        repo,
		hasher:      hasher,
		tokenGen:    tokenGen,
		validator:   validator,
		audit:       audit,
		rateLimiter: rateLimiter,
		email:       email,
		config:      config,
	}

	return &testMocks{
		service:     service,
		repo:        repo,
		hasher:      hasher,
		tokenGen:    tokenGen,
		rateLimiter: rateLimiter,
		audit:       audit,
		email:       email,
	}
}

// Backward compatibility function for simple tests.
// Sets up permissive defaults for rate limiter, audit, and email mocks so tests
// that only care about repo/hasher/tokenGen don't have to set up all dependencies.
func createTestAuthService() (*authService, *mockRepo, *mockHasher, *mockTokenGen) {
	mocks := createTestAuthServiceWithMocks()
	// Allow any rate limiter calls
	mocks.rateLimiter.On("AllowRegistration", mock.Anything, mock.Anything).Return(true, nil)
	mocks.rateLimiter.On("AllowLogin", mock.Anything, mock.Anything).Return(true, nil)
	mocks.rateLimiter.On("AllowPasswordReset", mock.Anything, mock.Anything).Return(true, nil)
	mocks.rateLimiter.On("RecordFailedLogin", mock.Anything, mock.Anything).Return(nil)
	mocks.rateLimiter.On("ClearFailedLogins", mock.Anything, mock.Anything).Return(nil)
	// Allow any audit events (async goroutine)
	mocks.audit.On("LogAuthEvent", mock.Anything, mock.Anything).Return(nil)
	mocks.audit.On("LogSecurityEvent", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	// Allow any email sends
	mocks.email.On("SendWelcomeEmail", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mocks.email.On("SendPasswordResetEmail", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mocks.email.On("SendEmailVerificationEmail", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mocks.email.On("SendOrganizationInviteEmail", mock.Anything, mock.Anything).Return(nil)
	return mocks.service, mocks.repo, mocks.hasher, mocks.tokenGen
}

// Helper to create test user
func createTestUser() *models.User {
	return &models.User{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Email:          "test@example.com",
		PasswordHash:   "hashed-password",
		FirstName:      "Test",
		LastName:       "User",
		Role:           models.RoleUser,
		IsActive:       true,
		EmailVerified:  true,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
}

// Helper to create test organization
func createTestOrganization() *models.Organization {
	return &models.Organization{
		ID:          uuid.New(),
		Name:        "Test Organization",
		Slug:        "test-org",
		Description: "Test organization",
		Settings: models.Settings{
			PasswordPolicy: models.DefaultPasswordPolicy(),
		},
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
}

func TestAuthService_Register(t *testing.T) {
	t.Run("successful registration with new organization", func(t *testing.T) {
		mocks := createTestAuthServiceWithMocks()
		ctx := context.Background()

		req := &models.RegisterRequest{
			Email:        "newuser@example.com",
			Password:     "SecurePassword123!",
			FirstName:    "New",
			LastName:     "User",
			Organization: "New Organization",
		}

		// Setup mocks
		mocks.rateLimiter.On("AllowRegistration", ctx, req.Email).Return(true, nil)
		mocks.audit.On("LogAuthEvent", ctx, mock.AnythingOfType("*interfaces.AuthEvent")).Return(nil)
		mocks.email.On("SendWelcomeEmail", mock.Anything, mock.AnythingOfType("*models.User"), mock.AnythingOfType("*models.Organization")).Return(nil)
		mocks.hasher.On("Hash", req.Password).Return("hashed-password", nil)
		mocks.repo.On("GetUserByEmail", ctx, req.Email).Return((*models.User)(nil), errors.ErrUserNotFound)

		newOrg := createTestOrganization()
		newUser := createTestUser()
		newUser.Email = req.Email
		newUser.FirstName = req.FirstName
		newUser.LastName = req.LastName
		newUser.Role = models.RoleOwner

		mocks.repo.On("CreateOrganization", ctx, mock.AnythingOfType("*models.CreateOrganizationRequest")).Return(newOrg, nil)
		mocks.repo.On("CreateUser", ctx, mock.AnythingOfType("*models.CreateUserRequest")).Return(newUser, nil)

		mocks.tokenGen.On("GenerateAccessToken", newUser).Return("access-token", nil)
		mocks.tokenGen.On("GenerateRefreshToken", newUser).Return("refresh-token", nil)

		// Execute
		response, err := mocks.service.Register(ctx, req)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, newUser, response.User)
		assert.Equal(t, newOrg, response.Organization)
		assert.Equal(t, "access-token", response.Token.AccessToken)
		assert.Equal(t, "refresh-token", response.Token.RefreshToken)
		assert.Equal(t, "Registration successful", response.Message)

		// Verify mocks
		mocks.hasher.AssertExpectations(t)
		mocks.repo.AssertExpectations(t)
		mocks.tokenGen.AssertExpectations(t)
	})

	t.Run("registration fails when user already exists", func(t *testing.T) {
		mocks := createTestAuthServiceWithMocks()
		ctx := context.Background()

		req := &models.RegisterRequest{
			Email:        "existing@example.com",
			Password:     "SecurePassword123!",
			FirstName:    "Existing",
			LastName:     "User",
			Organization: "Test Organization",
		}

		existingUser := createTestUser()
		existingUser.Email = req.Email

		// Setup mocks
		mocks.rateLimiter.On("AllowRegistration", ctx, req.Email).Return(true, nil)
		mocks.repo.On("GetUserByEmail", ctx, req.Email).Return(existingUser, nil)
		mocks.audit.On("LogAuthEvent", mock.Anything, mock.AnythingOfType("*interfaces.AuthEvent")).Return(nil)

		// Execute
		response, err := mocks.service.Register(ctx, req)

		// Assert
		assert.Nil(t, response)
		assert.Equal(t, errors.ErrUserExists, err)

		// Verify mocks
		mocks.repo.AssertExpectations(t)
		mocks.rateLimiter.AssertExpectations(t)
	})

	t.Run("registration fails with invalid password", func(t *testing.T) {
		service, _, _, _ := createTestAuthService()
		ctx := context.Background()

		req := &models.RegisterRequest{
			Email:        "test@example.com",
			Password:     "weak",
			FirstName:    "Test",
			LastName:     "User",
			Organization: "Test Organization",
		}

		// Execute
		response, err := service.Register(ctx, req)

		// Assert
		assert.Nil(t, response)
		assert.Error(t, err)
		var validationErr *errors.ValidationErrors
		require.True(t, stderrors.As(err, &validationErr))
	})
}

func TestAuthService_Login(t *testing.T) {
	t.Run("successful login", func(t *testing.T) {
		service, repo, hasher, tokenGen := createTestAuthService()
		ctx := context.Background()

		req := &models.LoginRequest{
			Email:    "test@example.com",
			Password: "TestPassword123!",
		}

		user := createTestUser()
		user.Email = req.Email
		org := createTestOrganization()
		org.ID = user.OrganizationID

		// Setup mocks
		repo.On("GetUserByEmail", ctx, req.Email).Return(user, nil)
		hasher.On("Verify", req.Password, user.PasswordHash).Return(nil)
		repo.On("GetOrganizationByID", ctx, user.OrganizationID).Return(org, nil)
		hasher.On("NeedsRehash", user.PasswordHash).Return(false)
		repo.On("UpdateLastLogin", ctx, user.ID).Return(nil)

		tokenGen.On("GenerateAccessToken", user).Return("access-token", nil)
		tokenGen.On("GenerateRefreshToken", user).Return("refresh-token", nil)

		// Execute
		response, err := service.Login(ctx, req)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, user, response.User)
		assert.Equal(t, org, response.Organization)
		assert.Equal(t, "access-token", response.Token.AccessToken)
		assert.Equal(t, "refresh-token", response.Token.RefreshToken)
		assert.Equal(t, "Login successful", response.Message)

		// Verify mocks
		repo.AssertExpectations(t)
		hasher.AssertExpectations(t)
		tokenGen.AssertExpectations(t)
	})

	t.Run("login fails with invalid credentials", func(t *testing.T) {
		service, repo, hasher, _ := createTestAuthService()
		ctx := context.Background()

		req := &models.LoginRequest{
			Email:    "test@example.com",
			Password: "wrongpassword",
		}

		user := createTestUser()
		user.Email = req.Email

		// Setup mocks
		repo.On("GetUserByEmail", ctx, req.Email).Return(user, nil)
		hasher.On("Verify", req.Password, user.PasswordHash).Return(errors.NewAuthError(errors.ErrCodeInvalidCredentials, "invalid password"))

		// Execute
		response, err := service.Login(ctx, req)

		// Assert
		assert.Nil(t, response)
		assert.Equal(t, errors.ErrInvalidCredentials, err)

		// Verify mocks
		repo.AssertExpectations(t)
		hasher.AssertExpectations(t)
	})

	t.Run("login fails when user is inactive", func(t *testing.T) {
		service, repo, _, _ := createTestAuthService()
		ctx := context.Background()

		req := &models.LoginRequest{
			Email:    "test@example.com",
			Password: "TestPassword123!",
		}

		user := createTestUser()
		user.Email = req.Email
		user.IsActive = false

		// Setup mocks
		repo.On("GetUserByEmail", ctx, req.Email).Return(user, nil)

		// Execute
		response, err := service.Login(ctx, req)

		// Assert
		assert.Nil(t, response)
		assert.Equal(t, errors.ErrUserInactive, err)

		// Verify mocks
		repo.AssertExpectations(t)
	})
}

func TestAuthService_ChangePassword(t *testing.T) {
	t.Run("successful password change", func(t *testing.T) {
		service, repo, hasher, _ := createTestAuthService()
		ctx := context.Background()

		userID := uuid.New()
		req := &models.ChangePasswordRequest{
			CurrentPassword: "oldpassword",
			NewPassword:     "NewPassword123!",
		}

		user := createTestUser()
		user.ID = userID

		// Setup mocks
		repo.On("GetUserByID", ctx, userID).Return(user, nil)
		hasher.On("Verify", req.CurrentPassword, user.PasswordHash).Return(nil)
		hasher.On("Hash", req.NewPassword).Return("new-hashed-password", nil)
		repo.On("SetUserPassword", ctx, userID, "new-hashed-password").Return(nil)
		repo.On("DeleteUserSessions", ctx, userID).Return(nil)
		repo.On("DeleteUserRefreshTokens", ctx, userID).Return(nil)

		// Execute
		err := service.ChangePassword(ctx, userID, req)

		// Assert
		require.NoError(t, err)

		// Verify mocks
		repo.AssertExpectations(t)
		hasher.AssertExpectations(t)
	})

	t.Run("password change fails with invalid current password", func(t *testing.T) {
		service, repo, hasher, _ := createTestAuthService()
		ctx := context.Background()

		userID := uuid.New()
		req := &models.ChangePasswordRequest{
			CurrentPassword: "wrongpassword",
			NewPassword:     "NewPassword123!",
		}

		user := createTestUser()
		user.ID = userID

		// Setup mocks
		repo.On("GetUserByID", ctx, userID).Return(user, nil)
		hasher.On("Verify", req.CurrentPassword, user.PasswordHash).Return(errors.NewAuthError(errors.ErrCodeInvalidCredentials, "invalid password"))

		// Execute
		err := service.ChangePassword(ctx, userID, req)

		// Assert
		assert.Equal(t, errors.ErrInvalidCredentials, err)

		// Verify mocks
		repo.AssertExpectations(t)
		hasher.AssertExpectations(t)
	})
}

func TestAuthService_ValidateToken(t *testing.T) {
	t.Run("successful token validation", func(t *testing.T) {
		service, repo, _, tokenGen := createTestAuthService()
		ctx := context.Background()

		token := "valid-token"
		userID := uuid.New()
		claims := &interfaces.TokenClaims{
			UserID: userID,
		}
		user := createTestUser()
		user.ID = userID

		// Setup mocks
		tokenGen.On("ValidateAccessToken", token).Return(claims, nil)
		repo.On("GetUserByID", ctx, userID).Return(user, nil)

		// Execute
		result, err := service.ValidateToken(ctx, token)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, user, result)

		// Verify mocks
		tokenGen.AssertExpectations(t)
		repo.AssertExpectations(t)
	})

	t.Run("token validation fails with invalid token", func(t *testing.T) {
		service, _, _, tokenGen := createTestAuthService()
		ctx := context.Background()

		token := "invalid-token"

		// Setup mocks
		tokenGen.On("ValidateAccessToken", token).Return((*interfaces.TokenClaims)(nil), errors.ErrInvalidToken)

		// Execute
		result, err := service.ValidateToken(ctx, token)

		// Assert
		assert.Nil(t, result)
		assert.Equal(t, errors.ErrInvalidToken, err)

		// Verify mocks
		tokenGen.AssertExpectations(t)
	})

	t.Run("token validation fails when user is inactive", func(t *testing.T) {
		service, repo, _, tokenGen := createTestAuthService()
		ctx := context.Background()

		token := "valid-token"
		userID := uuid.New()
		claims := &interfaces.TokenClaims{
			UserID: userID,
		}
		user := createTestUser()
		user.ID = userID
		user.IsActive = false

		// Setup mocks
		tokenGen.On("ValidateAccessToken", token).Return(claims, nil)
		repo.On("GetUserByID", ctx, userID).Return(user, nil)

		// Execute
		result, err := service.ValidateToken(ctx, token)

		// Assert
		assert.Nil(t, result)
		assert.Equal(t, errors.ErrUserInactive, err)

		// Verify mocks
		tokenGen.AssertExpectations(t)
		repo.AssertExpectations(t)
	})
}

func TestAuthService_HealthCheck(t *testing.T) {
	t.Run("healthy service", func(t *testing.T) {
		service, repo, _, _ := createTestAuthService()
		ctx := context.Background()

		// Setup mocks
		repo.On("Ping", ctx).Return(nil)

		// Execute
		response, err := service.HealthCheck(ctx)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "healthy", response.Status)
		assert.Equal(t, "1.0.0", response.Version)
		assert.Equal(t, "up", response.Services["database"])
		assert.Equal(t, "up", response.Services["rate_limiter"])
		assert.Equal(t, "up", response.Services["email"])
		assert.Equal(t, "up", response.Services["audit"])

		// Verify mocks
		repo.AssertExpectations(t)
	})

	t.Run("unhealthy service due to database", func(t *testing.T) {
		service, repo, _, _ := createTestAuthService()
		ctx := context.Background()

		// Setup mocks
		repo.On("Ping", ctx).Return(errors.ErrInternalError)

		// Execute
		response, err := service.HealthCheck(ctx)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "unhealthy", response.Status)
		assert.Equal(t, "down", response.Services["database"])

		// Verify mocks
		repo.AssertExpectations(t)
	})
}

func TestNewAuthService(t *testing.T) {
	t.Run("creates auth service with all dependencies", func(t *testing.T) {
		repo := &mockRepo{}
		hasher := security.NewPasswordHasher(nil)
		tokenGen := &mockTokenGen{}
		validator := validation.New()
		audit := &mockAuditLogger{}
		rateLimiter := &mockRateLimiter{}
		email := &mockEmailService{}
		config := &config.Config{}

		// Execute
		service := NewAuthService(repo, hasher, tokenGen, validator, audit, rateLimiter, email, config)

		// Assert
		assert.NotNil(t, service)
		assert.Implements(t, (*interfaces.AuthService)(nil), service)
	})
}

// Additional comprehensive tests for edge cases and error handling
func TestAuthService_EdgeCases(t *testing.T) {
	t.Run("register with existing organization", func(t *testing.T) {
		service, repo, hasher, tokenGen := createTestAuthService()
		ctx := context.Background()

		orgID := uuid.New()
		req := &models.RegisterRequest{
			Email:          "newuser@example.com",
			Password:       "SecurePassword123!",
			FirstName:      "New",
			LastName:       "User",
			OrganizationID: &orgID,
		}

		// Setup mocks
		hasher.On("Hash", req.Password).Return("hashed-password", nil)
		repo.On("GetUserByEmail", ctx, req.Email).Return((*models.User)(nil), errors.ErrUserNotFound)

		existingOrg := createTestOrganization()
		existingOrg.ID = orgID
		newUser := createTestUser()
		newUser.Email = req.Email
		newUser.Role = models.RoleUser
		newUser.OrganizationID = orgID

		repo.On("GetOrganizationByID", ctx, orgID).Return(existingOrg, nil)
		repo.On("CreateUser", ctx, mock.AnythingOfType("*models.CreateUserRequest")).Return(newUser, nil)

		tokenGen.On("GenerateAccessToken", newUser).Return("access-token", nil)
		tokenGen.On("GenerateRefreshToken", newUser).Return("refresh-token", nil)

		// Execute
		response, err := service.Register(ctx, req)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, models.RoleUser, response.User.Role) // Should be user, not owner
		assert.Equal(t, orgID, response.User.OrganizationID)

		// Verify mocks
		repo.AssertExpectations(t)
		hasher.AssertExpectations(t)
		tokenGen.AssertExpectations(t)
	})

	t.Run("refresh token with expired token", func(t *testing.T) {
		service, _, _, tokenGen := createTestAuthService()
		ctx := context.Background()

		req := &models.RefreshTokenRequest{
			RefreshToken: "expired-token",
		}

		// Setup mocks
		tokenGen.On("ValidateRefreshToken", req.RefreshToken).Return((*interfaces.TokenClaims)(nil), errors.ErrTokenExpired)

		// Execute
		response, err := service.RefreshToken(ctx, req)

		// Assert
		assert.Nil(t, response)
		assert.Equal(t, errors.ErrTokenExpired, err)

		// Verify mocks
		tokenGen.AssertExpectations(t)
	})
}