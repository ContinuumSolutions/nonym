package validation

import (
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"unicode"

	"github.com/sovereignprivacy/gateway/pkg/auth/errors"
	"github.com/sovereignprivacy/gateway/pkg/auth/models"
)

// validator implements the Validator interface
type validator struct {
	commonPasswords map[string]bool
}

// New creates a new validator instance
func New() *validator {
	return &validator{
		commonPasswords: loadCommonPasswords(),
	}
}

// ValidateRegisterRequest validates a registration request
func (v *validator) ValidateRegisterRequest(req *models.RegisterRequest) error {
	validationErrors := &errors.ValidationErrors{}

	// Email validation
	if err := v.ValidateEmail(req.Email); err != nil {
		validationErrors.Add("email", err.Error(), "invalid_email")
	}

	// Password validation
	defaultPolicy := models.DefaultPasswordPolicy()
	if err := v.ValidatePasswordStrength(req.Password, &defaultPolicy); err != nil {
		if authErr, ok := errors.AsAuthError(err); ok {
			validationErrors.Add("password", authErr.Message, string(authErr.Code))
		} else {
			validationErrors.Add("password", err.Error(), "invalid_password")
		}
	}

	// Name validation
	if strings.TrimSpace(req.FirstName) == "" {
		validationErrors.Add("first_name", "First name is required", "required")
	} else if len(req.FirstName) > 100 {
		validationErrors.Add("first_name", "First name must be 100 characters or less", "max_length")
	}

	if strings.TrimSpace(req.LastName) == "" {
		validationErrors.Add("last_name", "Last name is required", "required")
	} else if len(req.LastName) > 100 {
		validationErrors.Add("last_name", "Last name must be 100 characters or less", "max_length")
	}

	// Organization validation
	if req.OrganizationID == nil && strings.TrimSpace(req.Organization) == "" {
		validationErrors.Add("organization", "Organization name is required when not joining existing organization", "required")
	}

	if req.Organization != "" {
		if len(req.Organization) < 2 {
			validationErrors.Add("organization", "Organization name must be at least 2 characters", "min_length")
		}
		if len(req.Organization) > 255 {
			validationErrors.Add("organization", "Organization name must be 255 characters or less", "max_length")
		}
	}

	if validationErrors.HasErrors() {
		return validationErrors
	}

	return nil
}

// ValidateLoginRequest validates a login request
func (v *validator) ValidateLoginRequest(req *models.LoginRequest) error {
	validationErrors := &errors.ValidationErrors{}

	// Email validation
	if err := v.ValidateEmail(req.Email); err != nil {
		validationErrors.Add("email", err.Error(), "invalid_email")
	}

	// Password validation (basic - just check not empty)
	if strings.TrimSpace(req.Password) == "" {
		validationErrors.Add("password", "Password is required", "required")
	}

	if validationErrors.HasErrors() {
		return validationErrors
	}

	return nil
}

// ValidateUpdateUserRequest validates a user update request
func (v *validator) ValidateUpdateUserRequest(req *models.UpdateUserRequest) error {
	validationErrors := &errors.ValidationErrors{}

	if req.FirstName != nil {
		if strings.TrimSpace(*req.FirstName) == "" {
			validationErrors.Add("first_name", "First name cannot be empty", "required")
		} else if len(*req.FirstName) > 100 {
			validationErrors.Add("first_name", "First name must be 100 characters or less", "max_length")
		}
	}

	if req.LastName != nil {
		if strings.TrimSpace(*req.LastName) == "" {
			validationErrors.Add("last_name", "Last name cannot be empty", "required")
		} else if len(*req.LastName) > 100 {
			validationErrors.Add("last_name", "Last name must be 100 characters or less", "max_length")
		}
	}

	if req.Role != nil {
		if err := v.ValidateRole(*req.Role); err != nil {
			validationErrors.Add("role", err.Error(), "invalid_role")
		}
	}

	if validationErrors.HasErrors() {
		return validationErrors
	}

	return nil
}

// ValidateCreateOrganizationRequest validates an organization creation request
func (v *validator) ValidateCreateOrganizationRequest(req *models.CreateOrganizationRequest) error {
	validationErrors := &errors.ValidationErrors{}

	// Name validation
	if strings.TrimSpace(req.Name) == "" {
		validationErrors.Add("name", "Organization name is required", "required")
	} else if len(req.Name) < 2 {
		validationErrors.Add("name", "Organization name must be at least 2 characters", "min_length")
	} else if len(req.Name) > 255 {
		validationErrors.Add("name", "Organization name must be 255 characters or less", "max_length")
	}

	// Description validation
	if len(req.Description) > 1000 {
		validationErrors.Add("description", "Description must be 1000 characters or less", "max_length")
	}

	if validationErrors.HasErrors() {
		return validationErrors
	}

	return nil
}

// ValidateUpdateOrganizationRequest validates an organization update request
func (v *validator) ValidateUpdateOrganizationRequest(req *models.UpdateOrganizationRequest) error {
	validationErrors := &errors.ValidationErrors{}

	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			validationErrors.Add("name", "Organization name cannot be empty", "required")
		} else if len(*req.Name) < 2 {
			validationErrors.Add("name", "Organization name must be at least 2 characters", "min_length")
		} else if len(*req.Name) > 255 {
			validationErrors.Add("name", "Organization name must be 255 characters or less", "max_length")
		}
	}

	if req.Description != nil && len(*req.Description) > 1000 {
		validationErrors.Add("description", "Description must be 1000 characters or less", "max_length")
	}

	if validationErrors.HasErrors() {
		return validationErrors
	}

	return nil
}

// ValidatePasswordStrength validates password strength against policy
func (v *validator) ValidatePasswordStrength(password string, policy *models.PasswordPolicy) error {
	if len(password) < policy.MinLength {
		return errors.NewAuthError(errors.ErrCodeWeakPassword,
			fmt.Sprintf("Password must be at least %d characters long", policy.MinLength))
	}

	if len(password) > policy.MaxLength {
		return errors.NewAuthError(errors.ErrCodeWeakPassword,
			fmt.Sprintf("Password must be %d characters or less", policy.MaxLength))
	}

	var hasUpper, hasLower, hasNumber, hasSymbol bool

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSymbol = true
		}
	}

	if policy.RequireUppercase && !hasUpper {
		return errors.NewAuthError(errors.ErrCodeWeakPassword, "Password must contain at least one uppercase letter")
	}

	if policy.RequireLowercase && !hasLower {
		return errors.NewAuthError(errors.ErrCodeWeakPassword, "Password must contain at least one lowercase letter")
	}

	if policy.RequireNumbers && !hasNumber {
		return errors.NewAuthError(errors.ErrCodeWeakPassword, "Password must contain at least one number")
	}

	if policy.RequireSymbols && !hasSymbol {
		return errors.NewAuthError(errors.ErrCodeWeakPassword, "Password must contain at least one symbol")
	}

	// Check against common passwords
	if policy.DisallowCommon && v.isCommonPassword(password) {
		return errors.NewAuthError(errors.ErrCodeWeakPassword, "Password is too common, please choose a different one")
	}

	return nil
}

// ValidateEmail validates email format and constraints
func (v *validator) ValidateEmail(email string) error {
	email = strings.TrimSpace(email)

	if email == "" {
		return errors.NewAuthError(errors.ErrCodeInvalidEmail, "Email is required")
	}

	if len(email) > 255 {
		return errors.NewAuthError(errors.ErrCodeInvalidEmail, "Email must be 255 characters or less")
	}

	// Use Go's built-in email validation
	if _, err := mail.ParseAddress(email); err != nil {
		return errors.NewAuthError(errors.ErrCodeInvalidEmail, "Invalid email format")
	}

	// Additional email validation rules
	if strings.Count(email, "@") != 1 {
		return errors.NewAuthError(errors.ErrCodeInvalidEmail, "Invalid email format")
	}

	parts := strings.Split(email, "@")
	if len(parts[0]) == 0 || len(parts[1]) == 0 {
		return errors.NewAuthError(errors.ErrCodeInvalidEmail, "Invalid email format")
	}

	// Check for valid domain format
	domainRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)
	if !domainRegex.MatchString(parts[1]) {
		return errors.NewAuthError(errors.ErrCodeInvalidEmail, "Invalid email domain")
	}

	return nil
}

// ValidateRole validates user role
func (v *validator) ValidateRole(role models.Role) error {
	if !role.IsValid() {
		return errors.NewAuthError(errors.ErrCodeInvalidRole, "Invalid user role")
	}
	return nil
}

// isCommonPassword checks if password is in common passwords list
func (v *validator) isCommonPassword(password string) bool {
	return v.commonPasswords[strings.ToLower(password)]
}

// loadCommonPasswords loads a list of common passwords to reject
func loadCommonPasswords() map[string]bool {
	// Top 100 most common passwords - in production, load from file
	commonPasswords := []string{
		"password", "123456", "password123", "admin", "qwerty", "abc123",
		"password1", "1234567890", "welcome", "monkey", "dragon", "princess",
		"admin123", "letmein", "master", "hello", "freedom", "whatever",
		"123456789", "654321", "superman", "qwerty123", "123123", "football",
		"baseball", "welcome123", "ninja", "mustang", "access", "shadow",
		"master123", "michael", "computer", "sunshine", "iloveyou", "daniel",
		"1234567", "test", "guest", "1234", "12345", "login", "admin1",
		"password12", "temp", "changeme", "default", "root", "user", "demo",
	}

	passwordMap := make(map[string]bool)
	for _, pwd := range commonPasswords {
		passwordMap[pwd] = true
	}

	return passwordMap
}