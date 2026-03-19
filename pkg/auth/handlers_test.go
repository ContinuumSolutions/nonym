package auth

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	_ "modernc.org/sqlite"
)

func setupTestApp() *fiber.App {
	app := fiber.New()

	// Auth routes
	app.Post("/api/auth/register", HandleRegister)
	app.Post("/api/auth/login", HandleLogin)
	app.Post("/api/auth/logout", HandleLogout)
	app.Get("/api/auth/me", AuthMiddleware, HandleGetMe)

	return app
}

func TestHandleRegister(t *testing.T) {
	// Set up test database
	testDB, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}
	defer testDB.Close()

	if err := Initialize(testDB); err != nil {
		t.Fatalf("Failed to initialize auth: %v", err)
	}

	app := setupTestApp()

	// Test successful registration
	payload := SignupRequest{
		Email:        "test@example.com",
		Password:     "password123",
		FirstName:    "John",
		LastName:     "Doe",
		Organization: "Test Corp",
	}

	jsonPayload, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 201, got %d. Body: %s", resp.StatusCode, body)
	}

	var response map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&response)

	if response["message"] != "User registered successfully" {
		t.Errorf("Expected success message, got %v", response["message"])
	}

	if response["user"] == nil {
		t.Error("Expected user in response")
	}

	if response["organization"] == nil {
		t.Error("Expected organization in response")
	}

	// Test duplicate email registration
	req2 := httptest.NewRequest("POST", "/api/auth/register", bytes.NewBuffer(jsonPayload))
	req2.Header.Set("Content-Type", "application/json")

	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("Failed to send duplicate request: %v", err)
	}

	if resp2.StatusCode != 400 {
		t.Fatalf("Expected status 400 for duplicate email, got %d", resp2.StatusCode)
	}

	// Test invalid request body
	req3 := httptest.NewRequest("POST", "/api/auth/register", strings.NewReader("invalid json"))
	req3.Header.Set("Content-Type", "application/json")

	resp3, err := app.Test(req3)
	if err != nil {
		t.Fatalf("Failed to send invalid request: %v", err)
	}

	if resp3.StatusCode != 400 {
		t.Fatalf("Expected status 400 for invalid JSON, got %d", resp3.StatusCode)
	}
}

func TestHandleLogin(t *testing.T) {
	// Set up test database
	testDB, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}
	defer testDB.Close()

	if err := Initialize(testDB); err != nil {
		t.Fatalf("Failed to initialize auth: %v", err)
	}

	app := setupTestApp()

	// First register a user
	signupReq := SignupRequest{
		Email:        "login@example.com",
		Password:     "password123",
		FirstName:    "Login",
		LastName:     "User",
		Organization: "Login Corp",
	}

	_, _, err = RegisterUser(&signupReq)
	if err != nil {
		t.Fatalf("Failed to register user: %v", err)
	}

	// Test successful login
	loginPayload := LoginRequest{
		Email:    "login@example.com",
		Password: "password123",
	}

	jsonPayload, _ := json.Marshal(loginPayload)
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to send login request: %v", err)
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, body)
	}

	var response map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&response)

	if response["message"] != "Login successful" {
		t.Errorf("Expected success message, got %v", response["message"])
	}

	if response["token"] == nil {
		t.Error("Expected token in response")
	}

	if response["user"] == nil {
		t.Error("Expected user in response")
	}

	// Test invalid password
	invalidLoginPayload := LoginRequest{
		Email:    "login@example.com",
		Password: "wrongpassword",
	}

	jsonPayload2, _ := json.Marshal(invalidLoginPayload)
	req2 := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(jsonPayload2))
	req2.Header.Set("Content-Type", "application/json")

	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("Failed to send invalid login request: %v", err)
	}

	if resp2.StatusCode != 401 {
		t.Fatalf("Expected status 401 for invalid password, got %d", resp2.StatusCode)
	}

	// Test non-existent user
	nonExistentPayload := LoginRequest{
		Email:    "nonexistent@example.com",
		Password: "password123",
	}

	jsonPayload3, _ := json.Marshal(nonExistentPayload)
	req3 := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(jsonPayload3))
	req3.Header.Set("Content-Type", "application/json")

	resp3, err := app.Test(req3)
	if err != nil {
		t.Fatalf("Failed to send non-existent user request: %v", err)
	}

	if resp3.StatusCode != 401 {
		t.Fatalf("Expected status 401 for non-existent user, got %d", resp3.StatusCode)
	}
}

func TestAuthMiddleware(t *testing.T) {
	// Set up test database
	testDB, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}
	defer testDB.Close()

	if err := Initialize(testDB); err != nil {
		t.Fatalf("Failed to initialize auth: %v", err)
	}

	app := setupTestApp()

	// First register and login a user to get a token
	signupReq := SignupRequest{
		Email:        "auth@example.com",
		Password:     "password123",
		FirstName:    "Auth",
		LastName:     "User",
		Organization: "Auth Corp",
	}

	_, _, err = RegisterUser(&signupReq)
	if err != nil {
		t.Fatalf("Failed to register user: %v", err)
	}

	loginReq := LoginRequest{
		Email:    "auth@example.com",
		Password: "password123",
	}

	loginResp, err := LoginUser(&loginReq, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("Failed to login: %v", err)
	}

	// Test with valid token
	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to send authenticated request: %v", err)
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200 for valid token, got %d. Body: %s", resp.StatusCode, body)
	}

	// Test without token
	req2 := httptest.NewRequest("GET", "/api/auth/me", nil)
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("Failed to send unauthenticated request: %v", err)
	}

	if resp2.StatusCode != 401 {
		t.Fatalf("Expected status 401 for missing token, got %d", resp2.StatusCode)
	}

	// Test with invalid token
	req3 := httptest.NewRequest("GET", "/api/auth/me", nil)
	req3.Header.Set("Authorization", "Bearer invalid.token.here")

	resp3, err := app.Test(req3)
	if err != nil {
		t.Fatalf("Failed to send invalid token request: %v", err)
	}

	if resp3.StatusCode != 401 {
		t.Fatalf("Expected status 401 for invalid token, got %d", resp3.StatusCode)
	}

	// Test with malformed authorization header
	req4 := httptest.NewRequest("GET", "/api/auth/me", nil)
	req4.Header.Set("Authorization", "InvalidFormat")

	resp4, err := app.Test(req4)
	if err != nil {
		t.Fatalf("Failed to send malformed auth header request: %v", err)
	}

	if resp4.StatusCode != 401 {
		t.Fatalf("Expected status 401 for malformed auth header, got %d", resp4.StatusCode)
	}
}

func TestHandleGetMe(t *testing.T) {
	// Set up test database
	testDB, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}
	defer testDB.Close()

	if err := Initialize(testDB); err != nil {
		t.Fatalf("Failed to initialize auth: %v", err)
	}

	app := setupTestApp()

	// Register and login a user to get a token
	signupReq := SignupRequest{
		Email:        "getme@example.com",
		Password:     "password123",
		FirstName:    "GetMe",
		LastName:     "User",
		Organization: "GetMe Corp",
	}

	_, _, err = RegisterUser(&signupReq)
	if err != nil {
		t.Fatalf("Failed to register user: %v", err)
	}

	loginReq := LoginRequest{
		Email:    "getme@example.com",
		Password: "password123",
	}

	loginResp, err := LoginUser(&loginReq, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("Failed to login: %v", err)
	}

	// Test getting user profile
	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to send get me request: %v", err)
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, body)
	}

	var response map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&response)

	user := response["user"].(map[string]interface{})
	if user["email"] != signupReq.Email {
		t.Errorf("Expected email %s, got %v", signupReq.Email, user["email"])
	}

	if user["first_name"] != signupReq.FirstName {
		t.Errorf("Expected first name %s, got %v", signupReq.FirstName, user["first_name"])
	}

	// Verify organization is included
	if user["organization"] == nil {
		t.Error("Expected organization to be included in user profile")
	}
}

func TestHandleLogout(t *testing.T) {
	// Set up test database
	testDB, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}
	defer testDB.Close()

	if err := Initialize(testDB); err != nil {
		t.Fatalf("Failed to initialize auth: %v", err)
	}

	app := setupTestApp()

	// Register and login a user to get a token
	signupReq := SignupRequest{
		Email:        "logout@example.com",
		Password:     "password123",
		FirstName:    "Logout",
		LastName:     "User",
		Organization: "Logout Corp",
	}

	_, _, err = RegisterUser(&signupReq)
	if err != nil {
		t.Fatalf("Failed to register user: %v", err)
	}

	loginReq := LoginRequest{
		Email:    "logout@example.com",
		Password: "password123",
	}

	loginResp, err := LoginUser(&loginReq, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("Failed to login: %v", err)
	}

	// Test successful logout
	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to send logout request: %v", err)
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, body)
	}

	var response map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&response)

	if response["message"] != "Logged out successfully" {
		t.Errorf("Expected logout success message, got %v", response["message"])
	}

	// Test logout without token
	req2 := httptest.NewRequest("POST", "/api/auth/logout", nil)
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("Failed to send logout request without token: %v", err)
	}

	if resp2.StatusCode != 400 {
		t.Fatalf("Expected status 400 for missing token, got %d", resp2.StatusCode)
	}
}