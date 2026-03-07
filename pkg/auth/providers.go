package auth

import (
	"strconv"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/pbkdf2"
)

// ProviderModel represents an AI model configuration
type ProviderModel struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// ProviderConfig represents the configuration for an AI provider
type ProviderConfig struct {
	Enabled  bool            `json:"enabled"`
	APIKey   string          `json:"api_key,omitempty"`
	Endpoint string          `json:"endpoint,omitempty"`
	Models   []ProviderModel `json:"models"`
}

// ProvidersConfig represents the configuration for all AI providers
type ProvidersConfig struct {
	OpenAI    ProviderConfig `json:"openai"`
	Anthropic ProviderConfig `json:"anthropic"`
	Google    ProviderConfig `json:"google"`
	Local     ProviderConfig `json:"local"`
}

// ProviderTestRequest represents a request to test provider connection
type ProviderTestRequest struct {
	APIKey   string          `json:"api_key,omitempty"`
	Endpoint string          `json:"endpoint,omitempty"`
	Models   []ProviderModel `json:"models,omitempty"`
}

// ProviderTestResponse represents the response from testing a provider
type ProviderTestResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Models  []ProviderModel `json:"models,omitempty"`
}

// encryptionKey derives an encryption key from a user's ID
func deriveEncryptionKey(userID string) []byte {
	// In production, you'd want to use a proper key derivation with a secure salt
	return pbkdf2.Key([]byte(userID), []byte("spg-salt-2024"), 10000, 32, sha256.New)
}

// encryptData encrypts data using AES-GCM
func encryptData(data, userID string) (string, error) {
	key := deriveEncryptionKey(userID)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(data), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptData decrypts data using AES-GCM
func decryptData(encryptedData, userID string) (string, error) {
	key := deriveEncryptionKey(userID)

	data, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// GetProviderConfig retrieves the provider configuration for a user
func GetProviderConfig(userID string) (*ProvidersConfig, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `SELECT config_data FROM provider_configs WHERE user_id = ?`
	var encryptedConfig string

	err := db.QueryRow(query, userID).Scan(&encryptedConfig)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			// Return default configuration
			return &ProvidersConfig{
				OpenAI: ProviderConfig{
					Enabled: false,
					Models: []ProviderModel{
						{ID: "gpt-4", Name: "GPT-4", Enabled: false},
						{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", Enabled: true},
						{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", Enabled: false},
					},
				},
				Anthropic: ProviderConfig{
					Enabled: false,
					Models: []ProviderModel{
						{ID: "claude-3-haiku", Name: "Claude 3 Haiku", Enabled: true},
						{ID: "claude-3-sonnet", Name: "Claude 3 Sonnet", Enabled: false},
						{ID: "claude-3-opus", Name: "Claude 3 Opus", Enabled: false},
					},
				},
				Google: ProviderConfig{
					Enabled: false,
					Models: []ProviderModel{
						{ID: "gemini-pro", Name: "Gemini Pro", Enabled: true},
						{ID: "gemini-pro-vision", Name: "Gemini Pro Vision", Enabled: false},
					},
				},
				Local: ProviderConfig{
					Enabled: false,
					Endpoint: "http://localhost:11434",
					Models: []ProviderModel{
						{ID: "llama2", Name: "Llama 2", Enabled: true},
						{ID: "mistral", Name: "Mistral", Enabled: false},
						{ID: "codellama", Name: "Code Llama", Enabled: false},
					},
				},
			}, nil
		}
		return nil, fmt.Errorf("failed to query provider config: %w", err)
	}

	// Decrypt the configuration
	configJSON, err := decryptData(encryptedConfig, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt provider config: %w", err)
	}

	var config ProvidersConfig
	err = json.Unmarshal([]byte(configJSON), &config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal provider config: %w", err)
	}

	return &config, nil
}

// SaveProviderConfig saves the provider configuration for a user
func SaveProviderConfig(userID string, config *ProvidersConfig) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Serialize configuration
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal provider config: %w", err)
	}

	// Encrypt the configuration
	encryptedConfig, err := encryptData(string(configJSON), userID)
	if err != nil {
		return fmt.Errorf("failed to encrypt provider config: %w", err)
	}

	// Save to database
	query := `INSERT OR REPLACE INTO provider_configs (user_id, config_data, updated_at)
			  VALUES (?, ?, ?)`

	_, err = db.Exec(query, userID, encryptedConfig, time.Now())
	if err != nil {
		return fmt.Errorf("failed to save provider config: %w", err)
	}

	return nil
}

// TestProviderConnection tests the connection to a provider
func TestProviderConnection(provider string, config *ProviderTestRequest) (*ProviderTestResponse, error) {
	switch provider {
	case "openai":
		return testOpenAIConnection(config)
	case "anthropic":
		return testAnthropicConnection(config)
	case "google":
		return testGoogleConnection(config)
	case "local":
		return testLocalConnection(config)
	default:
		return &ProviderTestResponse{
			Success: false,
			Error:   "Unknown provider",
		}, nil
	}
}

func testOpenAIConnection(config *ProviderTestRequest) (*ProviderTestResponse, error) {
	if config.APIKey == "" {
		return &ProviderTestResponse{
			Success: false,
			Error:   "API key is required",
		}, nil
	}

	// Test OpenAI API
	req, err := http.NewRequest("GET", "https://api.openai.com/v1/models", nil)
	if err != nil {
		return &ProviderTestResponse{
			Success: false,
			Error:   "Failed to create request",
		}, nil
	}

	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return &ProviderTestResponse{
			Success: false,
			Error:   "Connection failed: " + err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return &ProviderTestResponse{
			Success: false,
			Error:   fmt.Sprintf("API returned status %d", resp.StatusCode),
		}, nil
	}

	return &ProviderTestResponse{
		Success: true,
	}, nil
}

func testAnthropicConnection(config *ProviderTestRequest) (*ProviderTestResponse, error) {
	if config.APIKey == "" {
		return &ProviderTestResponse{
			Success: false,
			Error:   "API key is required",
		}, nil
	}

	// Test Anthropic API with a simple request
	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", nil)
	if err != nil {
		return &ProviderTestResponse{
			Success: false,
			Error:   "Failed to create request",
		}, nil
	}

	req.Header.Set("x-api-key", config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return &ProviderTestResponse{
			Success: false,
			Error:   "Connection failed: " + err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	// Anthropic returns 400 for empty body, but that means auth worked
	if resp.StatusCode == 400 || resp.StatusCode == 200 {
		return &ProviderTestResponse{
			Success: true,
		}, nil
	}

	return &ProviderTestResponse{
		Success: false,
		Error:   fmt.Sprintf("API returned status %d", resp.StatusCode),
	}, nil
}

func testGoogleConnection(config *ProviderTestRequest) (*ProviderTestResponse, error) {
	if config.APIKey == "" {
		return &ProviderTestResponse{
			Success: false,
			Error:   "API key is required",
		}, nil
	}

	// Test Google Gemini API
	req, err := http.NewRequest("GET", "https://generativelanguage.googleapis.com/v1/models?key="+config.APIKey, nil)
	if err != nil {
		return &ProviderTestResponse{
			Success: false,
			Error:   "Failed to create request",
		}, nil
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return &ProviderTestResponse{
			Success: false,
			Error:   "Connection failed: " + err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return &ProviderTestResponse{
			Success: false,
			Error:   fmt.Sprintf("API returned status %d", resp.StatusCode),
		}, nil
	}

	return &ProviderTestResponse{
		Success: true,
	}, nil
}

func testLocalConnection(config *ProviderTestRequest) (*ProviderTestResponse, error) {
	endpoint := config.Endpoint
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}

	// Test local LLM endpoint (typically Ollama)
	req, err := http.NewRequest("GET", endpoint+"/api/tags", nil)
	if err != nil {
		return &ProviderTestResponse{
			Success: false,
			Error:   "Failed to create request",
		}, nil
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return &ProviderTestResponse{
			Success: false,
			Error:   "Connection failed: " + err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return &ProviderTestResponse{
			Success: false,
			Error:   fmt.Sprintf("Endpoint returned status %d", resp.StatusCode),
		}, nil
	}

	return &ProviderTestResponse{
		Success: true,
	}, nil
}

// HTTP Handlers

// HandleGetProviderConfig handles GET /api/provider-config
func HandleGetProviderConfig(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	config, err := GetProviderConfig(strconv.Itoa(user.ID))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to fetch provider configuration",
		})
	}

	return c.JSON(fiber.Map{
		"providers": config,
	})
}

// HandleSaveProviderConfig handles PUT /api/provider-config
func HandleSaveProviderConfig(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	var req struct {
		Providers *ProvidersConfig `json:"providers"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Providers == nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Providers configuration is required",
		})
	}

	err := SaveProviderConfig(strconv.Itoa(user.ID), req.Providers)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to save provider configuration",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Provider configuration saved successfully",
	})
}

// HandleTestProviderConnection handles POST /api/providers/:provider/test
func HandleTestProviderConnection(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*User)
	_ = user
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	provider := c.Params("provider")
	if provider == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Provider name is required",
		})
	}

	var config ProviderTestRequest
	if err := c.BodyParser(&config); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	result, err := TestProviderConnection(provider, &config)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to test provider connection",
		})
	}

	return c.JSON(result)
}
