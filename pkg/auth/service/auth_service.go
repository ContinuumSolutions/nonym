package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ContinuumSolutions/nonym/pkg/auth/config"
	"github.com/ContinuumSolutions/nonym/pkg/auth/errors"
	"github.com/ContinuumSolutions/nonym/pkg/auth/interfaces"
	"github.com/ContinuumSolutions/nonym/pkg/auth/models"
	"github.com/ContinuumSolutions/nonym/pkg/auth/security"
)

// authService implements the AuthService interface
type authService struct {
	repo         interfaces.AuthRepository
	hasher       interfaces.PasswordHasher
	tokenGen     interfaces.TokenGenerator
	validator    interfaces.Validator
	audit        interfaces.AuditLogger
	rateLimiter  interfaces.RateLimiter
	email        interfaces.EmailService
	config       *config.Config
}

// NewAuthService creates a new auth service instance
func NewAuthService(
	repo interfaces.AuthRepository,
	hasher interfaces.PasswordHasher,
	tokenGen interfaces.TokenGenerator,
	validator interfaces.Validator,
	audit interfaces.AuditLogger,
	rateLimiter interfaces.RateLimiter,
	email interfaces.EmailService,
	config *config.Config,
) interfaces.AuthService {
	return &authService{
		repo:        repo,
		hasher:      hasher,
		tokenGen:    tokenGen,
		validator:   validator,
		audit:       audit,
		rateLimiter: rateLimiter,
		email:       email,
		config:      config,
	}
}

// Register handles user registration with full validation and auditing
func (s *authService) Register(ctx context.Context, req *models.RegisterRequest) (*models.RegisterResponse, error) {
	// Input validation
	if err := s.validator.ValidateRegisterRequest(req); err != nil {
		s.logAuthEvent(ctx, nil, nil, interfaces.AuthEventRegister, false, err.Error())
		return nil, err
	}

	// Rate limiting
	if !s.allowRegistration(ctx, req.Email) {
		err := errors.ErrRateLimitExceeded
		s.logAuthEvent(ctx, nil, nil, interfaces.AuthEventRegister, false, "rate limit exceeded")
		return nil, err
	}

	// Check if user already exists
	existingUser, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil && !errors.IsAuthError(err) {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}
	if existingUser != nil {
		s.logAuthEvent(ctx, &existingUser.ID, &existingUser.OrganizationID, interfaces.AuthEventRegister, false, "user already exists")
		return nil, errors.ErrUserExists
	}

	// Hash password
	passwordHash, err := s.hasher.Hash(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	var user *models.User
	var organization *models.Organization

	// Use transaction for atomic user + organization creation
	err = s.repo.WithTx(ctx, func(txRepo interfaces.AuthRepository) error {
		var txErr error

		if req.OrganizationID != nil {
			// Join existing organization
			organization, txErr = txRepo.GetOrganizationByID(ctx, *req.OrganizationID)
			if txErr != nil {
				return fmt.Errorf("organization not found: %w", txErr)
			}

			// Create user in existing organization
			createUserReq := &models.CreateUserRequest{
				OrganizationID: *req.OrganizationID,
				Email:          req.Email,
				PasswordHash:   passwordHash,
				FirstName:      req.FirstName,
				LastName:       req.LastName,
				Role:           models.RoleUser,
				IsActive:       true,
				EmailVerified:  false,
			}

			user, txErr = txRepo.CreateUser(ctx, createUserReq)
			if txErr != nil {
				return fmt.Errorf("failed to create user: %w", txErr)
			}
		} else {
			// Create new organization
			orgReq := &models.CreateOrganizationRequest{
				Name:        req.Organization,
				Description: fmt.Sprintf("Organization for %s %s", req.FirstName, req.LastName),
				Settings:    models.Settings{PasswordPolicy: models.DefaultPasswordPolicy()},
			}

			organization, txErr = txRepo.CreateOrganization(ctx, orgReq)
			if txErr != nil {
				return fmt.Errorf("failed to create organization: %w", txErr)
			}

			// Create user as organization owner
			createUserReq := &models.CreateUserRequest{
				OrganizationID: organization.ID,
				Email:          req.Email,
				PasswordHash:   passwordHash,
				FirstName:      req.FirstName,
				LastName:       req.LastName,
				Role:           models.RoleOwner,
				IsActive:       true,
				EmailVerified:  false,
			}

			user, txErr = txRepo.CreateUser(ctx, createUserReq)
			if txErr != nil {
				return fmt.Errorf("failed to create user: %w", txErr)
			}
		}

		return nil
	})

	if err != nil {
		s.logAuthEvent(ctx, nil, nil, interfaces.AuthEventRegister, false, err.Error())
		return nil, err
	}

	// Generate token pair
	tokenPair, err := s.generateTokenPair(user)
	if err != nil {
		s.logAuthEvent(ctx, &user.ID, &user.OrganizationID, interfaces.AuthEventRegister, false, err.Error())
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	// Log successful registration
	s.logAuthEvent(ctx, &user.ID, &user.OrganizationID, interfaces.AuthEventRegister, true, "")

	// Send welcome email (non-blocking)
	if s.email != nil {
		go func() {
			if err := s.email.SendWelcomeEmail(context.Background(), user, organization); err != nil {
				// Log error but don't fail the registration
				s.logAuthEvent(context.Background(), &user.ID, &user.OrganizationID, "welcome_email_failed", false, err.Error())
			}
		}()
	}

	return &models.RegisterResponse{
		User:         user,
		Organization: organization,
		Token:        tokenPair,
		Message:      "Registration successful",
	}, nil
}

// Login handles user authentication
func (s *authService) Login(ctx context.Context, req *models.LoginRequest) (*models.LoginResponse, error) {
	// Input validation
	if err := s.validator.ValidateLoginRequest(req); err != nil {
		s.logAuthEvent(ctx, nil, nil, interfaces.AuthEventLoginFailed, false, err.Error())
		return nil, err
	}

	// Rate limiting
	if !s.allowLogin(ctx, req.Email) {
		err := errors.ErrRateLimitExceeded
		s.logAuthEvent(ctx, nil, nil, interfaces.AuthEventLoginFailed, false, "rate limit exceeded")
		return nil, err
	}

	// Get user by email
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if errors.IsAuthError(err) {
			s.recordFailedLogin(ctx, req.Email)
			s.logAuthEvent(ctx, nil, nil, interfaces.AuthEventLoginFailed, false, "user not found")
			return nil, errors.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Check if user is active
	if !user.IsActive {
		s.logAuthEvent(ctx, &user.ID, &user.OrganizationID, interfaces.AuthEventLoginFailed, false, "user inactive")
		return nil, errors.ErrUserInactive
	}

	// Verify password
	if err := s.hasher.Verify(req.Password, user.PasswordHash); err != nil {
		s.recordFailedLogin(ctx, req.Email)
		s.logAuthEvent(ctx, &user.ID, &user.OrganizationID, interfaces.AuthEventLoginFailed, false, "invalid password")
		return nil, errors.ErrInvalidCredentials
	}

	// Check organization access if specified
	if req.OrganizationID != nil && user.OrganizationID != *req.OrganizationID {
		s.logAuthEvent(ctx, &user.ID, &user.OrganizationID, interfaces.AuthEventLoginFailed, false, "organization access denied")
		return nil, errors.NewAuthError(errors.ErrCodeOrgAccessDenied, "Access to organization denied")
	}

	// Get organization details
	organization, err := s.repo.GetOrganizationByID(ctx, user.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	// Check if password needs rehashing
	if s.hasher.NeedsRehash(user.PasswordHash) {
		go func() {
			newHash, err := s.hasher.Hash(req.Password)
			if err == nil {
				s.repo.SetUserPassword(context.Background(), user.ID, newHash)
			}
		}()
	}

	// Clear failed login attempts
	s.clearFailedLogins(ctx, req.Email)

	// Generate token pair
	tokenPair, err := s.generateTokenPair(user)
	if err != nil {
		s.logAuthEvent(ctx, &user.ID, &user.OrganizationID, interfaces.AuthEventLoginFailed, false, err.Error())
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	// Update last login
	s.repo.UpdateLastLogin(ctx, user.ID)

	// Log successful login
	s.logAuthEvent(ctx, &user.ID, &user.OrganizationID, interfaces.AuthEventLogin, true, "")

	return &models.LoginResponse{
		User:         user,
		Organization: organization,
		Token:        tokenPair,
		ExpiresAt:    tokenPair.AccessTokenExpiresAt,
		Message:      "Login successful",
	}, nil
}

// Logout handles user logout
func (s *authService) Logout(ctx context.Context, userID uuid.UUID, sessionToken string) error {
	// Revoke user sessions
	if err := s.repo.DeleteUserSessions(ctx, userID); err != nil {
		return fmt.Errorf("failed to revoke user sessions: %w", err)
	}

	// Revoke refresh tokens
	if err := s.repo.DeleteUserRefreshTokens(ctx, userID); err != nil {
		return fmt.Errorf("failed to revoke refresh tokens: %w", err)
	}

	// Log logout
	s.logAuthEvent(ctx, &userID, nil, interfaces.AuthEventLogout, true, "")

	return nil
}

// RefreshToken handles token refresh
func (s *authService) RefreshToken(ctx context.Context, req *models.RefreshTokenRequest) (*models.TokenPair, error) {
	// Validate refresh token
	claims, err := s.tokenGen.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		return nil, err
	}

	// Get user to ensure they're still active
	user, err := s.repo.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if !user.IsActive {
		return nil, errors.ErrUserInactive
	}

	// Generate new token pair
	tokenPair, err := s.generateTokenPair(user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new tokens: %w", err)
	}

	// If refresh token rotation is enabled, revoke old refresh token
	if s.config.JWT.RefreshTokenRotation {
		// This would require storing refresh tokens in the database
		// For simplicity, we'll skip this for now
	}

	// Log token refresh
	s.logAuthEvent(ctx, &user.ID, &user.OrganizationID, interfaces.AuthEventTokenRefresh, true, "")

	return tokenPair, nil
}

// ValidateToken validates an access token and returns the user
func (s *authService) ValidateToken(ctx context.Context, token string) (*models.User, error) {
	// Validate token
	claims, err := s.tokenGen.ValidateAccessToken(token)
	if err != nil {
		return nil, err
	}

	// Get user to ensure they're still active
	user, err := s.repo.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if !user.IsActive {
		return nil, errors.ErrUserInactive
	}

	return user, nil
}

// ChangePassword handles user password change
func (s *authService) ChangePassword(ctx context.Context, userID uuid.UUID, req *models.ChangePasswordRequest) error {
	// Validate request
	if err := s.validator.ValidatePasswordStrength(req.NewPassword, &models.PasswordPolicy{
		MinLength:        12,
		MaxLength:        128,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireNumbers:   true,
		RequireSymbols:   true,
		DisallowCommon:   true,
	}); err != nil {
		s.logAuthEvent(ctx, &userID, nil, interfaces.AuthEventPasswordChange, false, err.Error())
		return err
	}

	// Get user to verify current password
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Verify current password
	if err := s.hasher.Verify(req.CurrentPassword, user.PasswordHash); err != nil {
		s.logAuthEvent(ctx, &userID, &user.OrganizationID, interfaces.AuthEventPasswordChange, false, "invalid current password")
		return errors.ErrInvalidCredentials
	}

	// Hash new password
	newPasswordHash, err := s.hasher.Hash(req.NewPassword)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	// Update password in database
	if err := s.repo.SetUserPassword(ctx, userID, newPasswordHash); err != nil {
		s.logAuthEvent(ctx, &userID, &user.OrganizationID, interfaces.AuthEventPasswordChange, false, err.Error())
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Revoke all existing sessions and tokens for security
	s.repo.DeleteUserSessions(ctx, userID)
	s.repo.DeleteUserRefreshTokens(ctx, userID)

	// Log successful password change
	s.logAuthEvent(ctx, &userID, &user.OrganizationID, interfaces.AuthEventPasswordChange, true, "")

	return nil
}

// GetUserProfile returns user profile information
func (s *authService) GetUserProfile(ctx context.Context, userID uuid.UUID) (*models.UserProfile, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.IsAuthError(err) {
			return nil, errors.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Get organization details
	organization, err := s.repo.GetOrganizationByID(ctx, user.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	profile := models.NewUserProfile(user)
	profile.Organization = organization

	return profile, nil
}

// UpdateUserProfile updates user profile information
func (s *authService) UpdateUserProfile(ctx context.Context, userID uuid.UUID, req *models.UpdateUserRequest) (*models.UserProfile, error) {
	// Validate request
	if err := s.validator.ValidateUpdateUserRequest(req); err != nil {
		return nil, err
	}

	// Update user
	updatedUser, err := s.repo.UpdateUser(ctx, userID, req)
	if err != nil {
		if errors.IsAuthError(err) {
			return nil, errors.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Get organization details
	organization, err := s.repo.GetOrganizationByID(ctx, updatedUser.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	profile := models.NewUserProfile(updatedUser)
	profile.Organization = organization

	return profile, nil
}

// DeleteUser soft deletes a user
func (s *authService) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if err := s.repo.DeleteUser(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	// Revoke all sessions and tokens
	s.repo.DeleteUserSessions(ctx, userID)
	s.repo.DeleteUserRefreshTokens(ctx, userID)

	// Log user deletion
	s.logAuthEvent(ctx, &userID, &user.OrganizationID, "user_deleted", true, "")

	return nil
}

// RequestPasswordReset handles password reset requests
func (s *authService) RequestPasswordReset(ctx context.Context, req *models.ResetPasswordRequest) error {
	// Rate limiting
	if s.rateLimiter != nil {
		allowed, err := s.rateLimiter.AllowPasswordReset(ctx, req.Email)
		if err != nil {
			return fmt.Errorf("rate limiter error: %w", err)
		}
		if !allowed {
			return errors.ErrRateLimitExceeded
		}
	}

	// Get user by email
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		// For security, don't reveal if email exists
		return nil
	}

	// Generate reset token
	resetToken, err := security.GenerateTokenHash()
	if err != nil {
		return fmt.Errorf("failed to generate reset token: %w", err)
	}

	// Create password reset record
	passwordReset := &interfaces.PasswordReset{
		ID:        uuid.New(),
		UserID:    user.ID,
		Token:     resetToken,
		ExpiresAt: time.Now().Add(15 * time.Minute), // 15 minute expiry
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repo.CreatePasswordReset(ctx, passwordReset); err != nil {
		return fmt.Errorf("failed to create password reset: %w", err)
	}

	// Send reset email (non-blocking)
	if s.email != nil {
		go func() {
			if err := s.email.SendPasswordResetEmail(context.Background(), user, resetToken); err != nil {
				s.logAuthEvent(context.Background(), &user.ID, &user.OrganizationID, "password_reset_email_failed", false, err.Error())
			}
		}()
	}

	// Log password reset request
	s.logAuthEvent(ctx, &user.ID, &user.OrganizationID, interfaces.AuthEventPasswordReset, true, "")

	return nil
}

// ConfirmPasswordReset handles password reset confirmation
func (s *authService) ConfirmPasswordReset(ctx context.Context, req *models.ConfirmResetPasswordRequest) error {
	// Get password reset record
	resetRecord, err := s.repo.GetPasswordReset(ctx, req.Token)
	if err != nil {
		return errors.ErrInvalidToken
	}

	// Check if token is expired
	if time.Now().After(resetRecord.ExpiresAt) {
		return errors.ErrTokenExpired
	}

	// Check if already used
	if resetRecord.UsedAt != nil {
		return errors.ErrInvalidToken
	}

	// Validate new password
	if err := s.validator.ValidatePasswordStrength(req.NewPassword, &models.PasswordPolicy{
		MinLength:        12,
		MaxLength:        128,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireNumbers:   true,
		RequireSymbols:   true,
		DisallowCommon:   true,
	}); err != nil {
		return err
	}

	// Hash new password
	newPasswordHash, err := s.hasher.Hash(req.NewPassword)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	// Update password and mark reset as used
	if err := s.repo.WithTx(ctx, func(txRepo interfaces.AuthRepository) error {
		if err := txRepo.SetUserPassword(ctx, resetRecord.UserID, newPasswordHash); err != nil {
			return err
		}

		// Mark reset token as used
		now := time.Now().UTC()
		resetRecord.UsedAt = &now
		return txRepo.DeletePasswordReset(ctx, req.Token)
	}); err != nil {
		return fmt.Errorf("failed to reset password: %w", err)
	}

	// Get user for logging
	user, _ := s.repo.GetUserByID(ctx, resetRecord.UserID)

	// Revoke all existing sessions and tokens
	s.repo.DeleteUserSessions(ctx, resetRecord.UserID)
	s.repo.DeleteUserRefreshTokens(ctx, resetRecord.UserID)

	// Log password reset completion
	if user != nil {
		s.logAuthEvent(ctx, &user.ID, &user.OrganizationID, interfaces.AuthEventPasswordReset, true, "completed")
	}

	return nil
}

// CreateOrganization creates a new organization
func (s *authService) CreateOrganization(ctx context.Context, req *models.CreateOrganizationRequest) (*models.Organization, error) {
	// Validate request
	if err := s.validator.ValidateCreateOrganizationRequest(req); err != nil {
		return nil, err
	}

	// Create organization
	organization, err := s.repo.CreateOrganization(ctx, req)
	if err != nil {
		if errors.IsAuthError(err) {
			return nil, errors.ErrOrgExists
		}
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	return organization, nil
}

// GetOrganization returns organization details
func (s *authService) GetOrganization(ctx context.Context, orgID uuid.UUID) (*models.Organization, error) {
	organization, err := s.repo.GetOrganizationByID(ctx, orgID)
	if err != nil {
		if errors.IsAuthError(err) {
			return nil, errors.ErrOrgNotFound
		}
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	return organization, nil
}

// UpdateOrganization updates organization information
func (s *authService) UpdateOrganization(ctx context.Context, orgID uuid.UUID, req *models.UpdateOrganizationRequest) (*models.Organization, error) {
	// Validate request
	if err := s.validator.ValidateUpdateOrganizationRequest(req); err != nil {
		return nil, err
	}

	// Update organization
	organization, err := s.repo.UpdateOrganization(ctx, orgID, req)
	if err != nil {
		if errors.IsAuthError(err) {
			return nil, errors.ErrOrgNotFound
		}
		return nil, fmt.Errorf("failed to update organization: %w", err)
	}

	return organization, nil
}

// DeleteOrganization soft deletes an organization
func (s *authService) DeleteOrganization(ctx context.Context, orgID uuid.UUID) error {
	if err := s.repo.DeleteOrganization(ctx, orgID); err != nil {
		if errors.IsAuthError(err) {
			return errors.ErrOrgNotFound
		}
		return fmt.Errorf("failed to delete organization: %w", err)
	}

	// Log organization deletion
	s.logAuthEvent(ctx, nil, &orgID, "organization_deleted", true, "")

	return nil
}

// GetOrganizationMembers returns organization members
func (s *authService) GetOrganizationMembers(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*models.OrganizationMember, int, error) {
	users, total, err := s.repo.GetOrganizationMembers(ctx, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get organization members: %w", err)
	}

	members := make([]*models.OrganizationMember, len(users))
	for i, user := range users {
		members[i] = models.NewOrganizationMember(user)
	}

	return members, total, nil
}

// GetOrganizationStats returns organization statistics
func (s *authService) GetOrganizationStats(ctx context.Context, orgID uuid.UUID) (*models.OrganizationStats, error) {
	stats, err := s.repo.GetOrganizationStats(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization stats: %w", err)
	}

	return stats, nil
}

// ListUsers returns a list of users for admin purposes
func (s *authService) ListUsers(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*models.UserProfile, int, error) {
	users, total, err := s.repo.ListUsers(ctx, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}

	profiles := make([]*models.UserProfile, len(users))
	for i, user := range users {
		profiles[i] = models.NewUserProfile(user)
	}

	return profiles, total, nil
}

// CreateUser creates a new user (admin operation)
func (s *authService) CreateUser(ctx context.Context, req *models.CreateUserRequest) (*models.UserProfile, error) {
	// Create user
	user, err := s.repo.CreateUser(ctx, req)
	if err != nil {
		if errors.IsAuthError(err) {
			return nil, errors.ErrUserExists
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Get organization details
	organization, err := s.repo.GetOrganizationByID(ctx, user.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	profile := models.NewUserProfile(user)
	profile.Organization = organization

	// Log user creation
	s.logAuthEvent(ctx, &user.ID, &user.OrganizationID, "user_created", true, "")

	return profile, nil
}

// UpdateUserRole updates a user's role (admin operation)
func (s *authService) UpdateUserRole(ctx context.Context, userID uuid.UUID, role models.Role) error {
	// Validate role
	if err := s.validator.ValidateRole(role); err != nil {
		return err
	}

	// Get user for logging
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Update user role
	updateReq := &models.UpdateUserRequest{
		Role: &role,
	}

	_, err = s.repo.UpdateUser(ctx, userID, updateReq)
	if err != nil {
		return fmt.Errorf("failed to update user role: %w", err)
	}

	// Log role change
	s.logAuthEvent(ctx, &userID, &user.OrganizationID, interfaces.AuthEventRoleChanged, true, fmt.Sprintf("role changed to %s", role))

	return nil
}

// SetUserActive activates or deactivates a user
func (s *authService) SetUserActive(ctx context.Context, userID uuid.UUID, active bool) error {
	// Get user for logging
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Update user active status
	updateReq := &models.UpdateUserRequest{
		IsActive: &active,
	}

	_, err = s.repo.UpdateUser(ctx, userID, updateReq)
	if err != nil {
		return fmt.Errorf("failed to update user status: %w", err)
	}

	// Revoke all sessions if deactivating
	if !active {
		s.repo.DeleteUserSessions(ctx, userID)
		s.repo.DeleteUserRefreshTokens(ctx, userID)
	}

	// Log status change
	eventType := interfaces.AuthEventAccountLocked
	if active {
		eventType = interfaces.AuthEventAccountUnlocked
	}
	s.logAuthEvent(ctx, &userID, &user.OrganizationID, eventType, true, "")

	return nil
}

// GetAuthEvents returns authentication events for auditing
func (s *authService) GetAuthEvents(ctx context.Context, filter *interfaces.AuthEventFilter) ([]*models.AuthEventResponse, int, error) {
	events, total, err := s.repo.GetAuthEvents(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get auth events: %w", err)
	}

	responses := make([]*models.AuthEventResponse, len(events))
	for i, event := range events {
		responses[i] = &models.AuthEventResponse{
			ID:          event.ID,
			Type:        event.Type,
			IPAddress:   event.IPAddress,
			UserAgent:   event.UserAgent,
			Success:     event.Success,
			ErrorReason: event.ErrorReason,
			Metadata:    event.Metadata,
			Timestamp:   event.CreatedAt,
		}

		// Add user and org details if available
		if event.UserID != nil {
			responses[i].UserID = *event.UserID
			if user, err := s.repo.GetUserByID(ctx, *event.UserID); err == nil {
				responses[i].UserEmail = user.Email
			}
		}

		if event.OrganizationID != nil {
			responses[i].OrgID = *event.OrganizationID
			if org, err := s.repo.GetOrganizationByID(ctx, *event.OrganizationID); err == nil {
				responses[i].OrgName = org.Name
			}
		}
	}

	return responses, total, nil
}

// RevokeAllUserSessions revokes all active sessions for a user
func (s *authService) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	// Get user for logging
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Delete all user sessions
	if err := s.repo.DeleteUserSessions(ctx, userID); err != nil {
		return fmt.Errorf("failed to revoke user sessions: %w", err)
	}

	// Delete all refresh tokens
	if err := s.repo.DeleteUserRefreshTokens(ctx, userID); err != nil {
		return fmt.Errorf("failed to revoke refresh tokens: %w", err)
	}

	// Log session revocation
	s.logAuthEvent(ctx, &userID, &user.OrganizationID, interfaces.AuthEventTokenRevoke, true, "all sessions revoked")

	return nil
}

// GetActiveSessions returns active sessions for a user
func (s *authService) GetActiveSessions(ctx context.Context, userID uuid.UUID) ([]*interfaces.UserSession, error) {
	// For this implementation, we'll use a placeholder since JWT tokens are stateless
	// In a real implementation with session storage, this would query the database
	return []*interfaces.UserSession{}, nil
}

// HealthCheck performs a health check of the auth service
func (s *authService) HealthCheck(ctx context.Context) (*models.HealthResponse, error) {
	response := &models.HealthResponse{
		Status:    "healthy",
		Version:   "1.0.0",
		Timestamp: time.Now().UTC(),
		Uptime:    "unknown", // Would track this in a real implementation
		Services:  make(map[string]string),
	}

	// Check database connectivity
	if err := s.repo.Ping(ctx); err != nil {
		response.Status = "unhealthy"
		response.Services["database"] = "down"
	} else {
		response.Services["database"] = "up"
	}

	// Check rate limiter (if configured)
	if s.rateLimiter != nil {
		response.Services["rate_limiter"] = "up"
	} else {
		response.Services["rate_limiter"] = "disabled"
	}

	// Check email service (if configured)
	if s.email != nil {
		response.Services["email"] = "up"
	} else {
		response.Services["email"] = "disabled"
	}

	// Check audit logger (if configured)
	if s.audit != nil {
		response.Services["audit"] = "up"
	} else {
		response.Services["audit"] = "disabled"
	}

	return response, nil
}

// Helper methods

func (s *authService) generateTokenPair(user *models.User) (*models.TokenPair, error) {
	if jwtGen, ok := s.tokenGen.(*security.JWTTokenGenerator); ok {
		return jwtGen.GenerateTokenPair(user)
	}

	// Fallback to individual token generation
	accessToken, err := s.tokenGen.GenerateAccessToken(user)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.tokenGen.GenerateRefreshToken(user)
	if err != nil {
		return nil, err
	}

	return &models.TokenPair{
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		AccessTokenExpiresAt:  time.Now().Add(s.config.JWT.AccessTokenExpiry),
		RefreshTokenExpiresAt: time.Now().Add(s.config.JWT.RefreshTokenExpiry),
		TokenType:             "Bearer",
	}, nil
}

func (s *authService) logAuthEvent(ctx context.Context, userID *uuid.UUID, orgID *uuid.UUID, eventType string, success bool, errorReason string) {
	if s.audit == nil {
		return
	}

	event := &interfaces.AuthEvent{
		ID:             uuid.New(),
		Type:           eventType,
		UserID:         userID,
		OrganizationID: orgID,
		Success:        success,
		ErrorReason:    errorReason,
		CreatedAt:      time.Now().UTC(),
	}

	// Log in background to not block main flow
	go func() {
		if err := s.audit.LogAuthEvent(context.Background(), event); err != nil {
			// Log to system logger as fallback
			fmt.Printf("Failed to log auth event: %v\n", err)
		}
	}()
}

func (s *authService) allowLogin(ctx context.Context, identifier string) bool {
	if s.rateLimiter == nil {
		return true
	}

	allowed, err := s.rateLimiter.AllowLogin(ctx, identifier)
	if err != nil {
		// Log error but allow the operation (fail open)
		fmt.Printf("Rate limiter error: %v\n", err)
		return true
	}

	return allowed
}

func (s *authService) allowRegistration(ctx context.Context, identifier string) bool {
	if s.rateLimiter == nil {
		return true
	}

	allowed, err := s.rateLimiter.AllowRegistration(ctx, identifier)
	if err != nil {
		// Log error but allow the operation (fail open)
		fmt.Printf("Rate limiter error: %v\n", err)
		return true
	}

	return allowed
}

func (s *authService) recordFailedLogin(ctx context.Context, identifier string) {
	if s.rateLimiter == nil {
		return
	}

	go func() {
		if err := s.rateLimiter.RecordFailedLogin(context.Background(), identifier); err != nil {
			fmt.Printf("Failed to record failed login: %v\n", err)
		}
	}()
}

func (s *authService) clearFailedLogins(ctx context.Context, identifier string) {
	if s.rateLimiter == nil {
		return
	}

	go func() {
		if err := s.rateLimiter.ClearFailedLogins(context.Background(), identifier); err != nil {
			fmt.Printf("Failed to clear failed logins: %v\n", err)
		}
	}()
}