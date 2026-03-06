package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrTokenInvalid = errors.New("token invalid")
	ErrTokenExpired = errors.New("token expired")
)

// JWTService handles JWT token generation and validation
type JWTService struct {
	secret    []byte
	secretDir string
}

// NewJWTService creates a new JWT service. It loads or generates a signing secret.
func NewJWTService(secretDir string) (*JWTService, error) {
	if secretDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			secretDir = "."
		} else {
			secretDir = filepath.Join(homeDir, ".ek1")
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(secretDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create secret directory: %w", err)
	}

	secretPath := filepath.Join(secretDir, "jwt_secret")

	// Try to load existing secret
	secret, err := os.ReadFile(secretPath)
	if errors.Is(err, fs.ErrNotExist) {
		// Generate new secret
		secret = make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return nil, fmt.Errorf("failed to generate secret: %w", err)
		}

		// Save secret to disk
		if err := os.WriteFile(secretPath, secret, 0600); err != nil {
			return nil, fmt.Errorf("failed to save secret: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to read secret: %w", err)
	}

	return &JWTService{
		secret:    secret,
		secretDir: secretDir,
	}, nil
}

// TokenClaims represents the JWT claims for EK-1
type TokenClaims struct {
	Subject   string `json:"sub"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	TokenID   string `json:"jti,omitempty"` // For logout tracking
	jwt.RegisteredClaims
}

// TokenResponse is returned when issuing new tokens
type TokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// GenerateToken creates a new JWT with 24-hour expiry
func (j *JWTService) GenerateToken() (*TokenResponse, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(24 * time.Hour)

	// Generate unique token ID for logout tracking
	tokenID, err := j.generateTokenID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token ID: %w", err)
	}

	claims := TokenClaims{
		Subject:   "ek1_user",
		IssuedAt:  now.Unix(),
		ExpiresAt: expiresAt.Unix(),
		TokenID:   tokenID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(j.secret)
	if err != nil {
		return nil, fmt.Errorf("failed to sign token: %w", err)
	}

	return &TokenResponse{
		Token:     tokenString,
		ExpiresAt: expiresAt,
	}, nil
}

// ValidateToken verifies a JWT and returns its claims
func (j *JWTService) ValidateToken(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Ensure the signing method is HMAC
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.secret, nil
	})

	if err != nil {
		return nil, ErrTokenInvalid
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}

	// Check expiration manually for better error handling
	if time.Now().Unix() > claims.ExpiresAt {
		return nil, ErrTokenExpired
	}

	return claims, nil
}

// generateTokenID creates a unique identifier for logout tracking
func (j *JWTService) generateTokenID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// ExtractTokenFromHeader extracts the JWT from Authorization: Bearer <token>
func ExtractTokenFromHeader(authHeader string) string {
	const bearerPrefix = "Bearer "
	if len(authHeader) > len(bearerPrefix) && authHeader[:len(bearerPrefix)] == bearerPrefix {
		return authHeader[len(bearerPrefix):]
	}
	return ""
}