package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/suite"
)

// OrganizationTestSuite is the test suite for organization and team functionality
type OrganizationTestSuite struct {
	suite.Suite
	app  *fiber.App
	user *User
}

func (suite *OrganizationTestSuite) SetupTest() {
	// Create test user
	suite.user = &User{
		ID:    1,
		Email: "org@test.com",
		Name:  "Organization Test User",
		Role:  "admin",
	}

	// Setup fiber app with auth middleware mock
	suite.app = fiber.New()
	suite.app.Use(suite.mockAuthMiddleware)

	// Register routes
	suite.app.Get("/api/v1/organization", HandleGetOrganizationV1)
	suite.app.Put("/api/v1/organization", HandleUpdateOrganizationV1)
	suite.app.Get("/api/v1/team/members", HandleGetTeamMembersV1)
	suite.app.Post("/api/v1/team/members", HandleInviteTeamMemberV1)
	suite.app.Delete("/api/v1/team/members/:id", HandleRemoveTeamMemberV1)
	suite.app.Put("/api/v1/security/2fa", HandleUpdateTwoFactorV1)
	suite.app.Delete("/api/v1/security/sessions/:id", HandleTerminateSessionV1)
	suite.app.Put("/api/v1/security/settings", HandleUpdateSecuritySettingsV1)
}

// Mock middleware that sets user context
func (suite *OrganizationTestSuite) mockAuthMiddleware(c *fiber.Ctx) error {
	c.Locals("user", suite.user)
	return c.Next()
}

func TestOrganizationSuite(t *testing.T) {
	suite.Run(t, new(OrganizationTestSuite))
}

func (suite *OrganizationTestSuite) TestHandleGetOrganizationV1() {
	req := httptest.NewRequest("GET", "/api/v1/organization", nil)
	resp, err := suite.app.Test(req, -1)
	suite.NoError(err)
	defer resp.Body.Close()

	suite.Equal(200, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.NoError(err)

	// Verify organization data structure
	suite.Contains(response, "organization")
	org := response["organization"].(map[string]interface{})

	// Check required fields
	requiredFields := []string{"id", "name", "industry", "size", "country", "description", "compliance", "owner_id"}
	for _, field := range requiredFields {
		suite.Contains(org, field, "Organization should contain field: %s", field)
	}

	// Verify data types and sample values
	suite.Equal(float64(1), org["id"])
	suite.Equal("Acme Corporation", org["name"])
	suite.Equal("technology", org["industry"])
	suite.Equal("51-200", org["size"])
	suite.Equal("US", org["country"])
	suite.Equal(float64(suite.user.ID), org["owner_id"])

	// Check compliance object
	compliance := org["compliance"].(map[string]interface{})
	suite.Contains(compliance, "gdpr")
	suite.Contains(compliance, "hipaa")
	suite.Contains(compliance, "ccpa")
	suite.Contains(compliance, "soc2")
}

func (suite *OrganizationTestSuite) TestHandleUpdateOrganizationV1() {
	updateRequest := OrganizationUpdateRequestV1{
		Name:        "Updated Corporation",
		Industry:    "healthcare",
		Size:        "201-500",
		Country:     "CA",
		Description: "Updated description for testing",
		Compliance: map[string]bool{
			"gdpr":  true,
			"hipaa": true,
			"ccpa":  false,
			"soc2":  true,
		},
	}

	jsonData, err := json.Marshal(updateRequest)
	suite.NoError(err)

	req := httptest.NewRequest("PUT", "/api/v1/organization", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.app.Test(req, -1)
	suite.NoError(err)
	defer resp.Body.Close()

	suite.Equal(200, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.NoError(err)

	suite.Contains(response, "message")
	suite.Contains(response, "organization")

	org := response["organization"].(map[string]interface{})
	suite.Equal(updateRequest.Name, org["name"])
	suite.Equal(updateRequest.Industry, org["industry"])
	suite.Equal(updateRequest.Size, org["size"])
	suite.Equal(updateRequest.Country, org["country"])
	suite.Equal(updateRequest.Description, org["description"])
}

func (suite *OrganizationTestSuite) TestHandleUpdateOrganizationV1InvalidJSON() {
	req := httptest.NewRequest("PUT", "/api/v1/organization", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.app.Test(req, -1)
	suite.NoError(err)
	defer resp.Body.Close()

	suite.Equal(400, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.NoError(err)

	suite.Contains(response, "error")
	suite.Equal("Invalid request body", response["error"])
}

func (suite *OrganizationTestSuite) TestHandleGetTeamMembersV1() {
	req := httptest.NewRequest("GET", "/api/v1/team/members", nil)
	resp, err := suite.app.Test(req, -1)
	suite.NoError(err)
	defer resp.Body.Close()

	suite.Equal(200, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.NoError(err)

	suite.Contains(response, "members")
	members := response["members"].([]interface{})

	// Should contain at least the owner
	suite.GreaterOrEqual(len(members), 1)

	// Check first member (owner)
	owner := members[0].(map[string]interface{})
	suite.Equal(suite.user.Name, owner["name"])
	suite.Equal(suite.user.Email, owner["email"])
	suite.Equal("owner", owner["role"])
	suite.Equal("active", owner["status"])

	// Verify all members have required fields
	for _, memberData := range members {
		member := memberData.(map[string]interface{})
		requiredFields := []string{"id", "name", "email", "role", "status", "joined_at"}
		for _, field := range requiredFields {
			suite.Contains(member, field, "Member should contain field: %s", field)
		}
	}
}

func (suite *OrganizationTestSuite) TestHandleInviteTeamMemberV1() {
	tests := []struct {
		name           string
		inviteRequest  TeamMemberInviteRequestV1
		expectedStatus int
		shouldHaveID   bool
	}{
		{
			name: "valid invite",
			inviteRequest: TeamMemberInviteRequestV1{
				Email: "newmember@test.com",
				Role:  "member",
			},
			expectedStatus: 201,
			shouldHaveID:   true,
		},
		{
			name: "valid invite with admin role",
			inviteRequest: TeamMemberInviteRequestV1{
				Email: "admin@test.com",
				Role:  "admin",
			},
			expectedStatus: 201,
			shouldHaveID:   true,
		},
		{
			name: "valid invite with viewer role",
			inviteRequest: TeamMemberInviteRequestV1{
				Email: "viewer@test.com",
				Role:  "viewer",
			},
			expectedStatus: 201,
			shouldHaveID:   true,
		},
		{
			name: "missing email",
			inviteRequest: TeamMemberInviteRequestV1{
				Role: "member",
			},
			expectedStatus: 400,
			shouldHaveID:   false,
		},
		{
			name: "missing role",
			inviteRequest: TeamMemberInviteRequestV1{
				Email: "norole@test.com",
			},
			expectedStatus: 400,
			shouldHaveID:   false,
		},
		{
			name: "invalid role",
			inviteRequest: TeamMemberInviteRequestV1{
				Email: "invalid@test.com",
				Role:  "invalid",
			},
			expectedStatus: 400,
			shouldHaveID:   false,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			jsonData, err := json.Marshal(tt.inviteRequest)
			suite.NoError(err)

			req := httptest.NewRequest("POST", "/api/v1/team/members", bytes.NewReader(jsonData))
			req.Header.Set("Content-Type", "application/json")

			resp, err := suite.app.Test(req, -1)
			suite.NoError(err)
			defer resp.Body.Close()

			suite.Equal(tt.expectedStatus, resp.StatusCode)

			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			suite.NoError(err)

			if tt.shouldHaveID {
				suite.Contains(response, "id")
				suite.Contains(response, "message")
				suite.Contains(response, "member")

				member := response["member"].(map[string]interface{})
				suite.Equal(tt.inviteRequest.Email, member["email"])
				suite.Equal(tt.inviteRequest.Role, member["role"])
				suite.Equal("pending", member["status"])
			} else {
				suite.Contains(response, "error")
			}
		})
	}
}

func (suite *OrganizationTestSuite) TestHandleInviteTeamMemberV1InvalidJSON() {
	req := httptest.NewRequest("POST", "/api/v1/team/members", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.app.Test(req, -1)
	suite.NoError(err)
	defer resp.Body.Close()

	suite.Equal(400, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.NoError(err)

	suite.Contains(response, "error")
	suite.Equal("Invalid request body", response["error"])
}

func (suite *OrganizationTestSuite) TestHandleRemoveTeamMemberV1() {
	tests := []struct {
		name           string
		memberID       string
		expectedStatus int
		expectedError  bool
	}{
		{
			name:           "valid removal",
			memberID:       "member_002",
			expectedStatus: 200,
			expectedError:  false,
		},
		{
			name:           "remove owner (should fail)",
			memberID:       "member_001",
			expectedStatus: 400,
			expectedError:  true,
		},
		{
			name:           "empty member ID",
			memberID:       "",
			expectedStatus: 400,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			url := "/api/v1/team/members/" + tt.memberID
			req := httptest.NewRequest("DELETE", url, nil)

			resp, err := suite.app.Test(req, -1)
			suite.NoError(err)
			defer resp.Body.Close()

			suite.Equal(tt.expectedStatus, resp.StatusCode)

			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			suite.NoError(err)

			if tt.expectedError {
				suite.Contains(response, "error")
			} else {
				suite.Contains(response, "message")
			}
		})
	}
}

func (suite *OrganizationTestSuite) TestHandleUpdateTwoFactorV1() {
	tests := []struct {
		name           string
		request        TwoFactorUpdateRequestV1
		expectedStatus int
	}{
		{
			name:           "enable 2FA",
			request:        TwoFactorUpdateRequestV1{Enabled: true},
			expectedStatus: 200,
		},
		{
			name:           "disable 2FA",
			request:        TwoFactorUpdateRequestV1{Enabled: false},
			expectedStatus: 200,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			jsonData, err := json.Marshal(tt.request)
			suite.NoError(err)

			req := httptest.NewRequest("PUT", "/api/v1/security/2fa", bytes.NewReader(jsonData))
			req.Header.Set("Content-Type", "application/json")

			resp, err := suite.app.Test(req, -1)
			suite.NoError(err)
			defer resp.Body.Close()

			suite.Equal(tt.expectedStatus, resp.StatusCode)

			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			suite.NoError(err)

			suite.Contains(response, "message")
			suite.Contains(response, "enabled")
			suite.Equal(tt.request.Enabled, response["enabled"])
		})
	}
}

func (suite *OrganizationTestSuite) TestHandleUpdateTwoFactorV1InvalidJSON() {
	req := httptest.NewRequest("PUT", "/api/v1/security/2fa", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.app.Test(req, -1)
	suite.NoError(err)
	defer resp.Body.Close()

	suite.Equal(400, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.NoError(err)

	suite.Contains(response, "error")
	suite.Equal("Invalid request body", response["error"])
}

func (suite *OrganizationTestSuite) TestHandleTerminateSessionV1() {
	tests := []struct {
		name           string
		sessionID      string
		expectedStatus int
		expectedError  bool
	}{
		{
			name:           "valid termination",
			sessionID:      "session_123",
			expectedStatus: 200,
			expectedError:  false,
		},
		{
			name:           "terminate current session (should fail)",
			sessionID:      "current_session",
			expectedStatus: 400,
			expectedError:  true,
		},
		{
			name:           "empty session ID",
			sessionID:      "",
			expectedStatus: 400,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			url := "/api/v1/security/sessions/" + tt.sessionID
			req := httptest.NewRequest("DELETE", url, nil)

			resp, err := suite.app.Test(req, -1)
			suite.NoError(err)
			defer resp.Body.Close()

			suite.Equal(tt.expectedStatus, resp.StatusCode)

			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			suite.NoError(err)

			if tt.expectedError {
				suite.Contains(response, "error")
			} else {
				suite.Contains(response, "message")
			}
		})
	}
}

func (suite *OrganizationTestSuite) TestHandleUpdateSecuritySettingsV1() {
	settingsRequest := SecuritySettingsRequestV1{
		IPWhitelist:      true,
		RequireSignature: false,
		RateLimit:        true,
	}

	jsonData, err := json.Marshal(settingsRequest)
	suite.NoError(err)

	req := httptest.NewRequest("PUT", "/api/v1/security/settings", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.app.Test(req, -1)
	suite.NoError(err)
	defer resp.Body.Close()

	suite.Equal(200, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.NoError(err)

	suite.Contains(response, "message")
	suite.Contains(response, "settings")

	settings := response["settings"].(map[string]interface{})
	suite.Equal(settingsRequest.IPWhitelist, settings["ipWhitelist"])
	suite.Equal(settingsRequest.RequireSignature, settings["requireSignature"])
	suite.Equal(settingsRequest.RateLimit, settings["rateLimit"])
}

func (suite *OrganizationTestSuite) TestHandleUpdateSecuritySettingsV1InvalidJSON() {
	req := httptest.NewRequest("PUT", "/api/v1/security/settings", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.app.Test(req, -1)
	suite.NoError(err)
	defer resp.Body.Close()

	suite.Equal(400, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.NoError(err)

	suite.Contains(response, "error")
	suite.Equal("Invalid request body", response["error"])
}

// Test authentication required for all endpoints
func (suite *OrganizationTestSuite) TestAuthenticationRequired() {
	// Create app without auth middleware
	noAuthApp := fiber.New()
	noAuthApp.Get("/api/v1/organization", HandleGetOrganizationV1)
	noAuthApp.Put("/api/v1/organization", HandleUpdateOrganizationV1)
	noAuthApp.Get("/api/v1/team/members", HandleGetTeamMembersV1)
	noAuthApp.Post("/api/v1/team/members", HandleInviteTeamMemberV1)
	noAuthApp.Delete("/api/v1/team/members/test", HandleRemoveTeamMemberV1)
	noAuthApp.Put("/api/v1/security/2fa", HandleUpdateTwoFactorV1)
	noAuthApp.Delete("/api/v1/security/sessions/test", HandleTerminateSessionV1)
	noAuthApp.Put("/api/v1/security/settings", HandleUpdateSecuritySettingsV1)

	endpoints := []struct {
		method string
		path   string
		body   string
	}{
		{"GET", "/api/v1/organization", ""},
		{"PUT", "/api/v1/organization", `{"name": "test"}`},
		{"GET", "/api/v1/team/members", ""},
		{"POST", "/api/v1/team/members", `{"email": "test@test.com", "role": "member"}`},
		{"DELETE", "/api/v1/team/members/test", ""},
		{"PUT", "/api/v1/security/2fa", `{"enabled": true}`},
		{"DELETE", "/api/v1/security/sessions/test", ""},
		{"PUT", "/api/v1/security/settings", `{"ipWhitelist": true}`},
	}

	for _, endpoint := range endpoints {
		suite.Run(endpoint.method+" "+endpoint.path, func() {
			var req *http.Request
			if endpoint.body != "" {
				req = httptest.NewRequest(endpoint.method, endpoint.path, bytes.NewReader([]byte(endpoint.body)))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(endpoint.method, endpoint.path, nil)
			}

			resp, err := noAuthApp.Test(req, -1)
			suite.NoError(err)
			defer resp.Body.Close()

			suite.Equal(401, resp.StatusCode)

			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			suite.NoError(err)

			suite.Contains(response, "error")
			suite.Equal("Authentication required", response["error"])
		})
	}
}