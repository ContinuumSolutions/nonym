package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config represents the complete auth system configuration
type Config struct {
	Database DatabaseConfig `json:"database"`
	JWT      JWTConfig      `json:"jwt"`
	Security SecurityConfig `json:"security"`
	Email    EmailConfig    `json:"email"`
	Server   ServerConfig   `json:"server"`
	Logging  LoggingConfig  `json:"logging"`
}

// DatabaseConfig contains database connection settings
type DatabaseConfig struct {
	Driver          string        `json:"driver"`           // "postgres" or "sqlite"
	Host            string        `json:"host"`
	Port            int           `json:"port"`
	Database        string        `json:"database"`
	Username        string        `json:"username"`
	Password        string        `json:"password"`
	SSLMode         string        `json:"ssl_mode"`
	MaxOpenConns    int           `json:"max_open_conns"`
	MaxIdleConns    int           `json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime"`
	MigrationsPath  string        `json:"migrations_path"`
}

// JWTConfig contains JWT token settings
type JWTConfig struct {
	SecretKey              string        `json:"secret_key"`
	AccessTokenExpiry      time.Duration `json:"access_token_expiry"`
	RefreshTokenExpiry     time.Duration `json:"refresh_token_expiry"`
	Issuer                 string        `json:"issuer"`
	Audience               string        `json:"audience"`
	RefreshTokenRotation   bool          `json:"refresh_token_rotation"`
	MaxRefreshTokens       int           `json:"max_refresh_tokens_per_user"`
}

// SecurityConfig contains security-related settings
type SecurityConfig struct {
	PasswordPolicy     PasswordPolicyConfig `json:"password_policy"`
	SessionTimeout     time.Duration        `json:"session_timeout"`
	MaxLoginAttempts   int                  `json:"max_login_attempts"`
	LockoutDuration    time.Duration        `json:"lockout_duration"`
	RequireEmailVerify bool                 `json:"require_email_verification"`
	MFAEnabled         bool                 `json:"mfa_enabled"`
	RateLimiting       RateLimitConfig      `json:"rate_limiting"`
}

// PasswordPolicyConfig defines password requirements
type PasswordPolicyConfig struct {
	MinLength        int  `json:"min_length"`
	RequireUppercase bool `json:"require_uppercase"`
	RequireLowercase bool `json:"require_lowercase"`
	RequireNumbers   bool `json:"require_numbers"`
	RequireSymbols   bool `json:"require_symbols"`
	MaxLength        int  `json:"max_length"`
	DisallowCommon   bool `json:"disallow_common"`
}

// RateLimitConfig defines rate limiting settings
type RateLimitConfig struct {
	LoginAttempts        RateLimit `json:"login_attempts"`
	RegistrationAttempts RateLimit `json:"registration_attempts"`
	PasswordResetAttempts RateLimit `json:"password_reset_attempts"`
	TokenRefreshAttempts RateLimit `json:"token_refresh_attempts"`
}

// RateLimit defines a rate limit configuration
type RateLimit struct {
	Requests int           `json:"requests"`
	Window   time.Duration `json:"window"`
	Enabled  bool          `json:"enabled"`
}

// EmailConfig contains email service settings
type EmailConfig struct {
	Provider     string `json:"provider"`      // "smtp", "sendgrid", "ses", etc.
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	FromAddress  string `json:"from_address"`
	FromName     string `json:"from_name"`
	Templates    EmailTemplateConfig `json:"templates"`
}

// EmailTemplateConfig contains email template settings
type EmailTemplateConfig struct {
	WelcomePath       string `json:"welcome_path"`
	PasswordResetPath string `json:"password_reset_path"`
	VerificationPath  string `json:"verification_path"`
	InvitePath        string `json:"invite_path"`
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	Host           string        `json:"host"`
	Port           int           `json:"port"`
	ReadTimeout    time.Duration `json:"read_timeout"`
	WriteTimeout   time.Duration `json:"write_timeout"`
	IdleTimeout    time.Duration `json:"idle_timeout"`
	MaxHeaderSize  int           `json:"max_header_size"`
	TLSEnabled     bool          `json:"tls_enabled"`
	TLSCertPath    string        `json:"tls_cert_path"`
	TLSKeyPath     string        `json:"tls_key_path"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level      string `json:"level"`       // "debug", "info", "warn", "error"
	Format     string `json:"format"`      // "json" or "text"
	Output     string `json:"output"`      // "stdout", "stderr", or file path
	Structured bool   `json:"structured"`  // Enable structured logging
}

// LoadConfig loads configuration from environment variables with secure defaults
func LoadConfig() (*Config, error) {
	config := &Config{
		Database: DatabaseConfig{
			Driver:          getEnvString("DB_DRIVER", "sqlite"),
			Host:            getEnvString("DB_HOST", "localhost"),
			Port:            getEnvInt("DB_PORT", 5432),
			Database:        getEnvString("DB_NAME", "auth.db"),
			Username:        getEnvString("DB_USER", ""),
			Password:        getEnvString("DB_PASSWORD", ""),
			SSLMode:         getEnvString("DB_SSL_MODE", "require"),
			MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
			MigrationsPath:  getEnvString("DB_MIGRATIONS_PATH", "./migrations"),
		},

		JWT: JWTConfig{
			SecretKey:            getEnvString("JWT_SECRET", ""),
			AccessTokenExpiry:    getEnvDuration("JWT_ACCESS_EXPIRY", 15*time.Minute),
			RefreshTokenExpiry:   getEnvDuration("JWT_REFRESH_EXPIRY", 7*24*time.Hour),
			Issuer:              getEnvString("JWT_ISSUER", "sovereign-privacy-gateway"),
			Audience:            getEnvString("JWT_AUDIENCE", "spg-api"),
			RefreshTokenRotation: getEnvBool("JWT_REFRESH_ROTATION", true),
			MaxRefreshTokens:    getEnvInt("JWT_MAX_REFRESH_TOKENS", 5),
		},

		Security: SecurityConfig{
			PasswordPolicy: PasswordPolicyConfig{
				MinLength:        getEnvInt("PASSWORD_MIN_LENGTH", 12),
				RequireUppercase: getEnvBool("PASSWORD_REQUIRE_UPPERCASE", true),
				RequireLowercase: getEnvBool("PASSWORD_REQUIRE_LOWERCASE", true),
				RequireNumbers:   getEnvBool("PASSWORD_REQUIRE_NUMBERS", true),
				RequireSymbols:   getEnvBool("PASSWORD_REQUIRE_SYMBOLS", true),
				MaxLength:        getEnvInt("PASSWORD_MAX_LENGTH", 128),
				DisallowCommon:   getEnvBool("PASSWORD_DISALLOW_COMMON", true),
			},
			SessionTimeout:     getEnvDuration("SESSION_TIMEOUT", 24*time.Hour),
			MaxLoginAttempts:   getEnvInt("MAX_LOGIN_ATTEMPTS", 5),
			LockoutDuration:    getEnvDuration("LOCKOUT_DURATION", 15*time.Minute),
			RequireEmailVerify: getEnvBool("REQUIRE_EMAIL_VERIFY", false),
			MFAEnabled:         getEnvBool("MFA_ENABLED", false),
			RateLimiting: RateLimitConfig{
				LoginAttempts: RateLimit{
					Requests: getEnvInt("RATE_LIMIT_LOGIN_REQUESTS", 10),
					Window:   getEnvDuration("RATE_LIMIT_LOGIN_WINDOW", time.Minute),
					Enabled:  getEnvBool("RATE_LIMIT_LOGIN_ENABLED", true),
				},
				RegistrationAttempts: RateLimit{
					Requests: getEnvInt("RATE_LIMIT_REGISTER_REQUESTS", 5),
					Window:   getEnvDuration("RATE_LIMIT_REGISTER_WINDOW", time.Hour),
					Enabled:  getEnvBool("RATE_LIMIT_REGISTER_ENABLED", true),
				},
				PasswordResetAttempts: RateLimit{
					Requests: getEnvInt("RATE_LIMIT_RESET_REQUESTS", 3),
					Window:   getEnvDuration("RATE_LIMIT_RESET_WINDOW", time.Hour),
					Enabled:  getEnvBool("RATE_LIMIT_RESET_ENABLED", true),
				},
				TokenRefreshAttempts: RateLimit{
					Requests: getEnvInt("RATE_LIMIT_REFRESH_REQUESTS", 20),
					Window:   getEnvDuration("RATE_LIMIT_REFRESH_WINDOW", time.Hour),
					Enabled:  getEnvBool("RATE_LIMIT_REFRESH_ENABLED", true),
				},
			},
		},

		Email: EmailConfig{
			Provider:     getEnvString("EMAIL_PROVIDER", "smtp"),
			SMTPHost:     getEnvString("SMTP_HOST", ""),
			SMTPPort:     getEnvInt("SMTP_PORT", 587),
			SMTPUsername: getEnvString("SMTP_USERNAME", ""),
			SMTPPassword: getEnvString("SMTP_PASSWORD", ""),
			FromAddress:  getEnvString("EMAIL_FROM_ADDRESS", "noreply@example.com"),
			FromName:     getEnvString("EMAIL_FROM_NAME", "Auth System"),
			Templates: EmailTemplateConfig{
				WelcomePath:       getEnvString("EMAIL_TEMPLATE_WELCOME", "./templates/welcome.html"),
				PasswordResetPath: getEnvString("EMAIL_TEMPLATE_RESET", "./templates/reset.html"),
				VerificationPath:  getEnvString("EMAIL_TEMPLATE_VERIFY", "./templates/verify.html"),
				InvitePath:        getEnvString("EMAIL_TEMPLATE_INVITE", "./templates/invite.html"),
			},
		},

		Server: ServerConfig{
			Host:          getEnvString("SERVER_HOST", "0.0.0.0"),
			Port:          getEnvInt("SERVER_PORT", 8080),
			ReadTimeout:   getEnvDuration("SERVER_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:  getEnvDuration("SERVER_WRITE_TIMEOUT", 10*time.Second),
			IdleTimeout:   getEnvDuration("SERVER_IDLE_TIMEOUT", 60*time.Second),
			MaxHeaderSize: getEnvInt("SERVER_MAX_HEADER_SIZE", 1<<20), // 1MB
			TLSEnabled:    getEnvBool("TLS_ENABLED", false),
			TLSCertPath:   getEnvString("TLS_CERT_PATH", ""),
			TLSKeyPath:    getEnvString("TLS_KEY_PATH", ""),
		},

		Logging: LoggingConfig{
			Level:      getEnvString("LOG_LEVEL", "info"),
			Format:     getEnvString("LOG_FORMAT", "json"),
			Output:     getEnvString("LOG_OUTPUT", "stdout"),
			Structured: getEnvBool("LOG_STRUCTURED", true),
		},
	}

	// Validate critical configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// validateConfig validates critical configuration settings
func validateConfig(config *Config) error {
	// JWT Secret validation
	if config.JWT.SecretKey == "" {
		return fmt.Errorf("JWT_SECRET is required and cannot be empty")
	}
	if len(config.JWT.SecretKey) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters long")
	}

	// Database validation
	if config.Database.Driver == "postgres" {
		if config.Database.Host == "" {
			return fmt.Errorf("DB_HOST is required for PostgreSQL")
		}
		if config.Database.Database == "" {
			return fmt.Errorf("DB_NAME is required for PostgreSQL")
		}
		if config.Database.Username == "" {
			return fmt.Errorf("DB_USER is required for PostgreSQL")
		}
	}

	// Password policy validation
	if config.Security.PasswordPolicy.MinLength < 8 {
		return fmt.Errorf("minimum password length cannot be less than 8")
	}
	if config.Security.PasswordPolicy.MaxLength < config.Security.PasswordPolicy.MinLength {
		return fmt.Errorf("maximum password length cannot be less than minimum")
	}

	return nil
}

// Helper functions for environment variable parsing
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// IsPostgreSQL checks if the configuration is for PostgreSQL
func (c *Config) IsPostgreSQL() bool {
	return c.Database.Driver == "postgres"
}

// IsSQLite checks if the configuration is for SQLite
func (c *Config) IsSQLite() bool {
	return c.Database.Driver == "sqlite"
}

// GetDSN returns the database connection string
func (c *Config) GetDSN() string {
	if c.IsPostgreSQL() {
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			c.Database.Host,
			c.Database.Port,
			c.Database.Username,
			c.Database.Password,
			c.Database.Database,
			c.Database.SSLMode,
		)
	}
	return c.Database.Database // For SQLite, this is the file path
}