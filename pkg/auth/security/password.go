package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

// PasswordHasher handles secure password hashing and verification
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, hash string) error
	NeedsRehash(hash string) bool
}

// HashingAlgorithm represents the hashing algorithm to use
type HashingAlgorithm int

const (
	AlgorithmBcrypt HashingAlgorithm = iota
	AlgorithmArgon2
)

// PasswordHasherConfig contains configuration for password hashing
type PasswordHasherConfig struct {
	Algorithm HashingAlgorithm

	// Bcrypt configuration
	BcryptCost int

	// Argon2 configuration
	Argon2Time    uint32
	Argon2Memory  uint32
	Argon2Threads uint8
	Argon2KeyLen  uint32
	Argon2SaltLen uint32
}

// DefaultPasswordHasherConfig returns secure default configuration
func DefaultPasswordHasherConfig() *PasswordHasherConfig {
	return &PasswordHasherConfig{
		Algorithm:     AlgorithmArgon2, // Argon2 is more secure for new implementations
		BcryptCost:    14,              // High cost for bcrypt
		Argon2Time:    3,               // Number of iterations
		Argon2Memory:  64 * 1024,       // 64 MB memory
		Argon2Threads: 2,               // Number of threads
		Argon2KeyLen:  32,              // 32 byte key
		Argon2SaltLen: 16,              // 16 byte salt
	}
}

// passwordHasher implements PasswordHasher
type passwordHasher struct {
	config *PasswordHasherConfig
}

// NewPasswordHasher creates a new password hasher
func NewPasswordHasher(config *PasswordHasherConfig) PasswordHasher {
	if config == nil {
		config = DefaultPasswordHasherConfig()
	}
	return &passwordHasher{config: config}
}

// Hash hashes a password using the configured algorithm
func (h *passwordHasher) Hash(password string) (string, error) {
	switch h.config.Algorithm {
	case AlgorithmBcrypt:
		return h.hashBcrypt(password)
	case AlgorithmArgon2:
		return h.hashArgon2(password)
	default:
		return "", fmt.Errorf("unsupported hashing algorithm: %d", h.config.Algorithm)
	}
}

// Verify verifies a password against a hash
func (h *passwordHasher) Verify(password, hash string) error {
	// Detect algorithm from hash format
	if strings.HasPrefix(hash, "$2a$") || strings.HasPrefix(hash, "$2b$") || strings.HasPrefix(hash, "$2y$") {
		return h.verifyBcrypt(password, hash)
	} else if strings.HasPrefix(hash, "$argon2id$") {
		return h.verifyArgon2(password, hash)
	} else {
		return fmt.Errorf("unsupported hash format")
	}
}

// NeedsRehash checks if a hash needs to be rehashed (e.g., cost too low)
func (h *passwordHasher) NeedsRehash(hash string) bool {
	if strings.HasPrefix(hash, "$2a$") || strings.HasPrefix(hash, "$2b$") || strings.HasPrefix(hash, "$2y$") {
		// For bcrypt, extract cost and compare
		cost, err := bcrypt.Cost([]byte(hash))
		if err != nil {
			return true // If we can't parse it, rehash
		}
		return cost < h.config.BcryptCost
	}

	// For Argon2, we could parse parameters, but for simplicity,
	// we'll assume modern hashes don't need rehashing
	if strings.HasPrefix(hash, "$argon2id$") {
		return false
	}

	// Unknown format should be rehashed
	return true
}

// hashBcrypt hashes password using bcrypt
func (h *passwordHasher) hashBcrypt(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), h.config.BcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password with bcrypt: %w", err)
	}
	return string(hash), nil
}

// verifyBcrypt verifies password using bcrypt
func (h *passwordHasher) verifyBcrypt(password, hash string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		return fmt.Errorf("password verification failed")
	}
	return nil
}

// hashArgon2 hashes password using Argon2id
func (h *passwordHasher) hashArgon2(password string) (string, error) {
	// Generate random salt
	salt := make([]byte, h.config.Argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// Hash password
	hash := argon2.IDKey(
		[]byte(password),
		salt,
		h.config.Argon2Time,
		h.config.Argon2Memory,
		h.config.Argon2Threads,
		h.config.Argon2KeyLen,
	)

	// Encode in PHC string format
	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedHash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		h.config.Argon2Memory,
		h.config.Argon2Time,
		h.config.Argon2Threads,
		encodedSalt,
		encodedHash,
	), nil
}

// verifyArgon2 verifies password using Argon2id
func (h *passwordHasher) verifyArgon2(password, hash string) error {
	// Parse PHC string format
	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		return fmt.Errorf("invalid argon2 hash format")
	}

	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return fmt.Errorf("failed to parse argon2 parameters: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return fmt.Errorf("failed to decode salt: %w", err)
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return fmt.Errorf("failed to decode hash: %w", err)
	}

	// Compute hash with provided parameters
	computedHash := argon2.IDKey(
		[]byte(password),
		salt,
		time,
		memory,
		threads,
		uint32(len(expectedHash)),
	)

	// Constant time comparison
	if subtle.ConstantTimeCompare(expectedHash, computedHash) != 1 {
		return fmt.Errorf("password verification failed")
	}

	return nil
}

// GenerateRandomPassword generates a cryptographically secure random password
func GenerateRandomPassword(length int) (string, error) {
	if length < 8 {
		length = 12 // Minimum secure length
	}
	if length > 128 {
		length = 128 // Maximum practical length
	}

	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()_+-=[]{}|;:,.<>?"

	password := make([]byte, length)
	for i := range password {
		// Generate random index
		randomBytes := make([]byte, 1)
		if _, err := rand.Read(randomBytes); err != nil {
			return "", fmt.Errorf("failed to generate random password: %w", err)
		}
		password[i] = charset[int(randomBytes[0])%len(charset)]
	}

	return string(password), nil
}