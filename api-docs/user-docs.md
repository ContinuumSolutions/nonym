# Sovereign Privacy Gateway API Documentation

This directory contains comprehensive API documentation for the Sovereign Privacy Gateway using OpenAPI 3.0 (Swagger) specification.

## Quick Start

### View API Documentation

1. **Online Documentation (Recommended)**
   - Start the gateway: `docker compose up` or `go run cmd/gateway/main.go`
   - Open your browser to: `http://localhost:8080/api/docs`
   - The interactive documentation will be available with a full API explorer

2. **Offline Documentation**
   - Open `api-docs.html` in your browser
   - Make sure `swagger.yaml` is in the same directory

### API Documentation Files

- `swagger.yaml` - Complete OpenAPI 3.0 specification
- `api-docs.html` - Standalone HTML viewer using Swagger UI
- `API-DOCUMENTATION.md` - This documentation file

## API Overview

The Sovereign Privacy Gateway provides several categories of endpoints:

### 🔐 Authentication & Users
- `POST /api/v1/auth/register` - Register new user
- `POST /api/v1/auth/login` - User login
- `POST /api/v1/auth/logout` - User logout
- `GET /api/v1/auth/me` - Get user profile

### 🔑 API Key Management
- `GET /api/v1/api-keys` - List API keys
- `POST /api/v1/api-keys` - Create new API key
- `GET /api/v1/api-keys/{id}/full` - Get full API key for copying
- `PATCH /api/v1/api-keys/{id}/revoke` - Revoke API key
- `DELETE /api/v1/api-keys/{id}` - Delete API key

### 🏢 Organization & Team
- `GET /api/v1/organization` - Get organization info
- `PUT /api/v1/organization` - Update organization
- `GET /api/v1/team/members` - List team members
- `POST /api/v1/team/members` - Invite team member
- `DELETE /api/v1/team/members/{id}` - Remove team member

### ⚙️ AI Provider Configuration
- `GET /api/v1/provider-config` - Get provider settings
- `PUT /api/v1/provider-config` - Update provider settings
- `POST /api/v1/providers/{provider}/test` - Test provider connection

### 🛡️ Security Settings
- `PUT /api/v1/security/2fa` - Update two-factor auth
- `DELETE /api/v1/security/sessions/{id}` - Terminate session
- `PUT /api/v1/security/settings` - Update security settings

### 📊 Analytics & Monitoring
- `GET /api/v1/statistics` - System statistics
- `GET /api/v1/transactions` - Transaction history
- `GET /api/v1/protection-events` - Protection events
- `GET /api/v1/protection-stats` - Protection statistics

### 🤖 AI Proxy Endpoints
- `POST /v1/chat/completions` - Chat completions with PII protection
- `POST /v1/completions` - Text completions with PII protection
- `POST /v1/embeddings` - Embeddings with PII protection
- `GET /v1/models` - List available models

### 💚 Health & Status
- `GET /health` - Basic health check
- `GET /gateway/status` - Detailed component status
- `GET /gateway/stats` - Performance statistics

## Authentication Methods

The API uses two different authentication schemes:

### 1. Bearer Token (Dashboard/Management APIs)
```bash
# Get token via login
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "password"}'

# Use token for management endpoints
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  http://localhost:8080/api/v1/statistics
```

### 2. API Key (Proxy Endpoints)
```bash
# Use API key for AI proxy endpoints
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "X-API-Key: your-spg-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [
      {"role": "user", "content": "Hello, my email is john@example.com"}
    ]
  }'
```

## Usage Examples

### Creating an API Key
```bash
# 1. Login to get JWT token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "password"}' \
  | jq -r '.token')

# 2. Create API key
curl -X POST http://localhost:8080/api/v1/api-keys \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My API Key",
    "permissions": "write",
    "expiryDate": "2024-12-31"
  }'
```

### Testing PII Protection
```bash
# Send request with PII - it will be automatically detected and anonymized
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "X-API-Key: your-spg-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [
      {
        "role": "user",
        "content": "My SSN is 123-45-6789 and email is john.doe@company.com"
      }
    ]
  }'
```

The gateway will:
1. Detect PII (SSN, email)
2. Replace with tokens (e.g., `[SSN_TOKEN_1]`, `[EMAIL_TOKEN_1]`)
3. Send anonymized request to AI provider
4. Restore original PII in the response
5. Log the transaction for audit

## Error Responses

All endpoints return consistent error responses:

```json
{
  "error": "Error type",
  "message": "Detailed error message"
}
```

Common HTTP status codes:
- `200` - Success
- `201` - Created successfully
- `400` - Bad request / validation error
- `401` - Authentication required / invalid credentials
- `403` - Forbidden / insufficient permissions
- `404` - Resource not found
- `500` - Internal server error

## Development & Testing

### Viewing Documentation Locally

```bash
# Start the gateway
go run cmd/gateway/main.go

# Open browser to:
# http://localhost:8080/api/docs
```

### Using Different Environments

The Swagger specification supports multiple server environments:

- **Development**: `http://localhost:8080`
- **Production**: `https://gateway.your-domain.com`

Switch between them in the Swagger UI server dropdown.

### Testing with Curl

Each endpoint in the documentation includes curl examples. You can copy them directly from the Swagger UI.

## Support

- **Documentation Issues**: Check the interactive documentation at `/api/docs`
- **API Questions**: Review the comprehensive examples in this guide
- **Technical Support**: Contact licensing@sovereignprivacy.com

## License

This API documentation is part of the Sovereign Privacy Gateway project, licensed under the OpenVPN-style commercial license. See the main project README for license details.