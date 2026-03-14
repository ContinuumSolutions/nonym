package errors

import (
	"fmt"
	"net/http"
	"time"
)

// ErrorCode represents specific error codes for the auth system
type ErrorCode string

const (
	// Authentication errors
	ErrCodeInvalidCredentials   ErrorCode = "INVALID_CREDENTIALS"
	ErrCodeUserNotFound        ErrorCode = "USER_NOT_FOUND"
	ErrCodeUserInactive        ErrorCode = "USER_INACTIVE"
	ErrCodeUserExists          ErrorCode = "USER_EXISTS"
	ErrCodeInvalidToken        ErrorCode = "INVALID_TOKEN"
	ErrCodeTokenExpired        ErrorCode = "TOKEN_EXPIRED"
	ErrCodeRefreshTokenInvalid ErrorCode = "REFRESH_TOKEN_INVALID"

	// Authorization errors
	ErrCodeUnauthorized        ErrorCode = "UNAUTHORIZED"
	ErrCodeInsufficientPrivs   ErrorCode = "INSUFFICIENT_PRIVILEGES"
	ErrCodeOrgAccessDenied     ErrorCode = "ORGANIZATION_ACCESS_DENIED"

	// Validation errors
	ErrCodeValidationFailed    ErrorCode = "VALIDATION_FAILED"
	ErrCodeInvalidEmail        ErrorCode = "INVALID_EMAIL"
	ErrCodeWeakPassword        ErrorCode = "WEAK_PASSWORD"
	ErrCodePasswordMismatch    ErrorCode = "PASSWORD_MISMATCH"
	ErrCodeInvalidRole         ErrorCode = "INVALID_ROLE"

	// Organization errors
	ErrCodeOrgNotFound         ErrorCode = "ORGANIZATION_NOT_FOUND"
	ErrCodeOrgExists           ErrorCode = "ORGANIZATION_EXISTS"
	ErrCodeOrgInactive         ErrorCode = "ORGANIZATION_INACTIVE"
	ErrCodeInvalidOrgSlug      ErrorCode = "INVALID_ORGANIZATION_SLUG"

	// System errors
	ErrCodeDatabaseError       ErrorCode = "DATABASE_ERROR"
	ErrCodeInternalError       ErrorCode = "INTERNAL_ERROR"
	ErrCodeConfigError         ErrorCode = "CONFIGURATION_ERROR"
	ErrCodeRateLimitExceeded   ErrorCode = "RATE_LIMIT_EXCEEDED"

	// Security errors
	ErrCodeSuspiciousActivity  ErrorCode = "SUSPICIOUS_ACTIVITY"
	ErrCodeAccountLocked       ErrorCode = "ACCOUNT_LOCKED"
	ErrCodeMFARequired         ErrorCode = "MFA_REQUIRED"
	ErrCodeInvalidMFACode      ErrorCode = "INVALID_MFA_CODE"
)

// AuthError represents a structured authentication error
type AuthError struct {
	Code       ErrorCode              `json:"code"`
	Message    string                 `json:"message"`
	Details    string                 `json:"details,omitempty"`
	StatusCode int                    `json:"status_code"`
	Timestamp  time.Time              `json:"timestamp"`
	TraceID    string                 `json:"trace_id,omitempty"`
	Context    map[string]interface{} `json:"context,omitempty"`
	Cause      error                  `json:"-"`
}

// Error implements the error interface
func (e *AuthError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap implements error unwrapping for Go 1.13+
func (e *AuthError) Unwrap() error {
	return e.Cause
}

// WithContext adds context to the error
func (e *AuthError) WithContext(key string, value interface{}) *AuthError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithTraceID adds a trace ID to the error
func (e *AuthError) WithTraceID(traceID string) *AuthError {
	e.TraceID = traceID
	return e
}

// WithCause adds the underlying cause error
func (e *AuthError) WithCause(cause error) *AuthError {
	e.Cause = cause
	return e
}

// WithDetails adds additional details to the error
func (e *AuthError) WithDetails(details string) *AuthError {
	e.Details = details
	return e
}

// NewAuthError creates a new AuthError
func NewAuthError(code ErrorCode, message string) *AuthError {
	return &AuthError{
		Code:       code,
		Message:    message,
		StatusCode: getDefaultStatusCode(code),
		Timestamp:  time.Now().UTC(),
	}
}

// NewAuthErrorWithDetails creates a new AuthError with details
func NewAuthErrorWithDetails(code ErrorCode, message, details string) *AuthError {
	return &AuthError{
		Code:       code,
		Message:    message,
		Details:    details,
		StatusCode: getDefaultStatusCode(code),
		Timestamp:  time.Now().UTC(),
	}
}

// Validation Error Collection
type ValidationErrors struct {
	Errors []*ValidationError `json:"errors"`
}

func (ve *ValidationErrors) Error() string {
	if len(ve.Errors) == 0 {
		return "validation failed"
	}
	return fmt.Sprintf("validation failed: %s", ve.Errors[0].Message)
}

func (ve *ValidationErrors) HasErrors() bool {
	return len(ve.Errors) > 0
}

func (ve *ValidationErrors) Add(field, message, code string) {
	ve.Errors = append(ve.Errors, &ValidationError{
		Field:   field,
		Message: message,
		Code:    code,
	})
}

type ValidationError struct {
	Field   string      `json:"field"`
	Message string      `json:"message"`
	Code    string      `json:"code"`
	Value   interface{} `json:"value,omitempty"`
}

// Pre-defined common errors
var (
	ErrInvalidCredentials = NewAuthError(ErrCodeInvalidCredentials, "Invalid email or password")
	ErrUserNotFound      = NewAuthError(ErrCodeUserNotFound, "User not found")
	ErrUserInactive      = NewAuthError(ErrCodeUserInactive, "User account is disabled")
	ErrUserExists        = NewAuthError(ErrCodeUserExists, "User with this email already exists")
	ErrInvalidToken      = NewAuthError(ErrCodeInvalidToken, "Invalid or malformed token")
	ErrTokenExpired      = NewAuthError(ErrCodeTokenExpired, "Token has expired")
	ErrUnauthorized      = NewAuthError(ErrCodeUnauthorized, "Unauthorized access")
	ErrOrgNotFound       = NewAuthError(ErrCodeOrgNotFound, "Organization not found")
	ErrOrgExists         = NewAuthError(ErrCodeOrgExists, "Organization already exists")
	ErrInternalError     = NewAuthError(ErrCodeInternalError, "Internal server error")
	ErrRateLimitExceeded = NewAuthError(ErrCodeRateLimitExceeded, "Rate limit exceeded")
)

// getDefaultStatusCode maps error codes to HTTP status codes
func getDefaultStatusCode(code ErrorCode) int {
	switch code {
	case ErrCodeInvalidCredentials, ErrCodeUserNotFound, ErrCodeInvalidToken,
		 ErrCodeTokenExpired, ErrCodeRefreshTokenInvalid, ErrCodePasswordMismatch:
		return http.StatusUnauthorized

	case ErrCodeUnauthorized, ErrCodeInsufficientPrivs, ErrCodeOrgAccessDenied:
		return http.StatusForbidden

	case ErrCodeOrgNotFound:
		return http.StatusNotFound

	case ErrCodeUserExists, ErrCodeOrgExists:
		return http.StatusConflict

	case ErrCodeValidationFailed, ErrCodeInvalidEmail, ErrCodeWeakPassword,
		 ErrCodeInvalidRole, ErrCodeInvalidOrgSlug:
		return http.StatusBadRequest

	case ErrCodeUserInactive, ErrCodeOrgInactive, ErrCodeAccountLocked:
		return http.StatusForbidden

	case ErrCodeRateLimitExceeded:
		return http.StatusTooManyRequests

	case ErrCodeMFARequired, ErrCodeInvalidMFACode:
		return http.StatusUnauthorized

	case ErrCodeDatabaseError, ErrCodeInternalError, ErrCodeConfigError:
		return http.StatusInternalServerError

	default:
		return http.StatusInternalServerError
	}
}

// IsAuthError checks if an error is an AuthError
func IsAuthError(err error) bool {
	_, ok := err.(*AuthError)
	return ok
}

// AsAuthError converts an error to AuthError if possible
func AsAuthError(err error) (*AuthError, bool) {
	authErr, ok := err.(*AuthError)
	return authErr, ok
}