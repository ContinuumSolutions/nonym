package security

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/ContinuumSolutions/nonym/pkg/auth/config"
	"github.com/ContinuumSolutions/nonym/pkg/auth/errors"
	"github.com/ContinuumSolutions/nonym/pkg/auth/interfaces"
	"github.com/ContinuumSolutions/nonym/pkg/auth/models"
)

// JWTTokenGenerator implements the TokenGenerator interface
type JWTTokenGenerator struct {
	config *config.Config
	secret []byte
}

// NewJWTTokenGenerator creates a new JWT token generator
func NewJWTTokenGenerator(cfg *config.Config) (*JWTTokenGenerator, error) {
	if len(cfg.JWT.SecretKey) < 32 {
		return nil, fmt.Errorf("JWT secret key must be at least 32 characters")
	}

	return &JWTTokenGenerator{
		config: cfg,
		secret: []byte(cfg.JWT.SecretKey),
	}, nil
}

// GenerateAccessToken generates a new access token for the user
func (j *JWTTokenGenerator) GenerateAccessToken(user *models.User) (string, error) {
	now := time.Now()
	expiresAt := now.Add(j.config.JWT.AccessTokenExpiry)

	claims := jwt.MapClaims{
		"iss":        j.config.JWT.Issuer,
		"aud":        j.config.JWT.Audience,
		"sub":        user.ID.String(),
		"iat":        now.Unix(),
		"exp":        expiresAt.Unix(),
		"token_type": "access",

		// Custom claims
		"user_id":         user.ID.String(),
		"organization_id": user.OrganizationID.String(),
		"email":           user.Email,
		"role":            string(user.Role),
		"session_id":      uuid.New().String(), // Generate unique session ID
		"is_active":       user.IsActive,
		"email_verified":  user.EmailVerified,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(j.secret)
	if err != nil {
		return "", fmt.Errorf("failed to sign access token: %w", err)
	}

	return tokenString, nil
}

// GenerateRefreshToken generates a new refresh token for the user
func (j *JWTTokenGenerator) GenerateRefreshToken(user *models.User) (string, error) {
	now := time.Now()
	expiresAt := now.Add(j.config.JWT.RefreshTokenExpiry)

	claims := jwt.MapClaims{
		"iss":        j.config.JWT.Issuer,
		"aud":        j.config.JWT.Audience,
		"sub":        user.ID.String(),
		"iat":        now.Unix(),
		"exp":        expiresAt.Unix(),
		"token_type": "refresh",

		// Minimal claims for refresh token
		"user_id":         user.ID.String(),
		"organization_id": user.OrganizationID.String(),
		"jti":             uuid.New().String(), // Unique token ID for revocation
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(j.secret)
	if err != nil {
		return "", fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return tokenString, nil
}

// ValidateAccessToken validates and parses an access token
func (j *JWTTokenGenerator) ValidateAccessToken(tokenString string) (*interfaces.TokenClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Check signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.secret, nil
	})

	if err != nil {
		return nil, errors.ErrInvalidToken.WithCause(err)
	}

	if !token.Valid {
		return nil, errors.ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.ErrInvalidToken.WithDetails("invalid claims format")
	}

	// Verify token type
	tokenType, ok := claims["token_type"].(string)
	if !ok || tokenType != "access" {
		return nil, errors.ErrInvalidToken.WithDetails("invalid token type")
	}

	// Parse claims
	tokenClaims, err := j.parseTokenClaims(claims)
	if err != nil {
		return nil, err
	}

	// Check expiration
	if time.Unix(tokenClaims.ExpiresAt, 0).Before(time.Now()) {
		return nil, errors.ErrTokenExpired
	}

	return tokenClaims, nil
}

// ValidateRefreshToken validates and parses a refresh token
func (j *JWTTokenGenerator) ValidateRefreshToken(tokenString string) (*interfaces.TokenClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Check signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.secret, nil
	})

	if err != nil {
		return nil, errors.NewAuthError(errors.ErrCodeRefreshTokenInvalid, "Invalid refresh token").WithCause(err)
	}

	if !token.Valid {
		return nil, errors.NewAuthError(errors.ErrCodeRefreshTokenInvalid, "Invalid refresh token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.NewAuthError(errors.ErrCodeRefreshTokenInvalid, "Invalid refresh token claims")
	}

	// Verify token type
	tokenType, ok := claims["token_type"].(string)
	if !ok || tokenType != "refresh" {
		return nil, errors.NewAuthError(errors.ErrCodeRefreshTokenInvalid, "Invalid token type")
	}

	// Parse claims
	tokenClaims, err := j.parseTokenClaims(claims)
	if err != nil {
		return nil, err
	}

	// Check expiration
	if time.Unix(tokenClaims.ExpiresAt, 0).Before(time.Now()) {
		return nil, errors.NewAuthError(errors.ErrCodeRefreshTokenInvalid, "Refresh token expired")
	}

	return tokenClaims, nil
}

// RevokeToken revokes a token (placeholder - tokens are stateless)
func (j *JWTTokenGenerator) RevokeToken(token string) error {
	// For stateless JWT, we can't revoke individual tokens
	// This would require a token blacklist in the database
	// For now, return nil (no-op)
	return nil
}

// parseTokenClaims parses JWT claims into TokenClaims struct
func (j *JWTTokenGenerator) parseTokenClaims(claims jwt.MapClaims) (*interfaces.TokenClaims, error) {
	// Parse user ID
	userIDStr, ok := claims["user_id"].(string)
	if !ok {
		return nil, errors.ErrInvalidToken.WithDetails("missing user_id claim")
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, errors.ErrInvalidToken.WithDetails("invalid user_id format")
	}

	// Parse organization ID
	orgIDStr, ok := claims["organization_id"].(string)
	if !ok {
		return nil, errors.ErrInvalidToken.WithDetails("missing organization_id claim")
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		return nil, errors.ErrInvalidToken.WithDetails("invalid organization_id format")
	}

	// Parse email
	email, ok := claims["email"].(string)
	if !ok {
		email = "" // Email might not be present in refresh tokens
	}

	// Parse role
	roleStr, ok := claims["role"].(string)
	var role models.Role
	if ok {
		role = models.Role(roleStr)
	}

	// Parse session ID
	sessionIDStr, ok := claims["session_id"].(string)
	var sessionID uuid.UUID
	if ok {
		sessionID, _ = uuid.Parse(sessionIDStr)
	}

	// Parse issued at
	iatFloat, ok := claims["iat"].(float64)
	if !ok {
		return nil, errors.ErrInvalidToken.WithDetails("missing iat claim")
	}

	// Parse expires at
	expFloat, ok := claims["exp"].(float64)
	if !ok {
		return nil, errors.ErrInvalidToken.WithDetails("missing exp claim")
	}

	// Parse token type
	tokenType, ok := claims["token_type"].(string)
	if !ok {
		tokenType = "access" // Default to access
	}

	return &interfaces.TokenClaims{
		UserID:         userID,
		OrganizationID: orgID,
		Email:          email,
		Role:           role,
		SessionID:      sessionID,
		IssuedAt:       int64(iatFloat),
		ExpiresAt:      int64(expFloat),
		TokenType:      tokenType,
	}, nil
}

// GenerateTokenHash generates a secure hash for storing refresh tokens
func GenerateTokenHash() (string, error) {
	bytes := make([]byte, 32) // 256 bits
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate token hash: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateTokenPair generates both access and refresh tokens
func (j *JWTTokenGenerator) GenerateTokenPair(user *models.User) (*models.TokenPair, error) {
	// Generate access token
	accessToken, err := j.GenerateAccessToken(user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate refresh token
	refreshToken, err := j.GenerateRefreshToken(user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &models.TokenPair{
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		AccessTokenExpiresAt:  time.Now().Add(j.config.JWT.AccessTokenExpiry),
		RefreshTokenExpiresAt: time.Now().Add(j.config.JWT.RefreshTokenExpiry),
		TokenType:             "Bearer",
	}, nil
}