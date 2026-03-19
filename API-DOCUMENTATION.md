# Sovereign Privacy Gateway API Documentation

## Overview

The Sovereign Privacy Gateway is a high-performance Privacy Air-Gap middleware that intercepts data between internal systems and external AI models. It acts as a transparent reverse proxy that detects and anonymizes PII in real-time, ensuring sensitive data never leaves your infrastructure.

## Authentication

The API uses two authentication methods:

### 1. Bearer Token Authentication (Dashboard/Management)
For dashboard and management endpoints, use JWT tokens obtained from the login endpoint:

```bash
# Login to get token
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"yourpassword"}'

# Use token in subsequent requests
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  http://localhost:8080/api/v1/auth/me
```

### 2. API Key Authentication (Proxy Endpoints)
For AI proxy endpoints, use API keys in the X-API-Key header:

```bash
# Create API key (requires JWT token)
curl -X POST http://localhost:8080/api/v1/api-keys \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"My API Key","permissions":"write"}'

# Use API key for proxy requests
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "X-API-Key: spg_your_api_key_here" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-3.5-turbo","messages":[{"role":"user","content":"Hello"}]}'
```

## Quick Start

### 1. Health Check
```bash
curl http://localhost:8080/health
```

### 2. Register Account
```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@yourcompany.com",
    "password": "yourpassword123",
    "name": "Admin User",
    "organization": "Your Company"
  }'
```

### 3. Create API Key
```bash
# Login first
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@yourcompany.com","password":"yourpassword123"}' \
  | jq -r '.token')

# Create API key
curl -X POST http://localhost:8080/api/v1/api-keys \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Production Key","permissions":"write"}'
```

### 4. Use Proxy with PII Protection
```bash
# This request will automatically detect and anonymize the email
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "X-API-Key: spg_your_api_key_here" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [
      {
        "role": "user",
        "content": "My email is john.doe@company.com, can you help me?"
      }
    ]
  }'
```

## API Endpoints

### Health & Status
- `GET /health` - Basic health check
- `GET /gateway/status` - Detailed component status
- `GET /gateway/stats` - Performance statistics
- `GET /api/v1/debug` - Debug information

### Authentication
- `POST /api/v1/auth/register` - Register new user
- `POST /api/v1/auth/login` - User login
- `POST /api/v1/auth/logout` - User logout
- `GET /api/v1/auth/me` - Get current user profile

### API Key Management
- `GET /api/v1/api-keys` - List user's API keys
- `POST /api/v1/api-keys` - Create new API key
- `GET /api/v1/api-keys/:id/full` - Get full API key (for copying)
- `PATCH /api/v1/api-keys/:id/revoke` - Revoke API key
- `DELETE /api/v1/api-keys/:id` - Delete API key

### Organization Management
- `GET /api/v1/organization` - Get organization info
- `PUT /api/v1/organization` - Update organization
- `GET /api/v1/team/members` - List team members
- `POST /api/v1/team/members` - Invite team member
- `DELETE /api/v1/team/members/:id` - Remove team member

### Analytics & Monitoring
- `GET /api/v1/statistics` - System statistics
- `GET /api/v1/transactions` - Transaction history
- `GET /api/v1/protection-events` - Protection events
- `GET /api/v1/protection-stats` - Protection statistics

### AI Proxy Endpoints
- `POST /v1/chat/completions` - Chat completions (OpenAI compatible)
- `POST /v1/completions` - Text completions
- `POST /v1/embeddings` - Text embeddings
- `GET /v1/models` - List available models

### Provider Configuration
- `GET /api/v1/provider-config` - Get provider config
- `PUT /api/v1/provider-config` - Update provider config
- `POST /api/v1/providers/:provider/test` - Test provider connection

### Security Settings
- `PUT /api/v1/security/2fa` - Update 2FA settings
- `DELETE /api/v1/security/sessions/:id` - Terminate session
- `PUT /api/v1/security/settings` - Update security settings

## Documentation
- `GET /api/docs` - Swagger UI documentation
- `GET /swagger.yaml` - OpenAPI specification
- `GET /docs` - Redirect to documentation

## PII Protection Features

### Automatic Detection
The gateway automatically detects and protects:
- Email addresses
- Social Security Numbers (SSN)
- Credit card numbers
- API keys
- Phone numbers
- IP addresses

### Protection Modes
1. **Anonymization Mode** (default): Replaces PII with secure tokens
2. **Strict Mode**: Blocks requests containing high-sensitivity data

### Example Protection Flow
```
Original Request: "My email is john@example.com"
↓
Gateway Processing: Detects email, creates token {{EMAIL_abc123}}
↓
To AI Provider: "My email is {{EMAIL_abc123}}"
↓
AI Response: "I can help with your email {{EMAIL_abc123}}"
↓
Gateway Processing: Replaces token with original value
↓
To Client: "I can help with your email john@example.com"
```

## Error Handling

The API returns standard HTTP status codes:

- `200` - Success
- `201` - Created
- `400` - Bad Request (validation errors)
- `401` - Unauthorized (authentication required)
- `403` - Forbidden (insufficient permissions)
- `404` - Not Found
- `429` - Rate Limited
- `500` - Internal Server Error

Error responses follow this format:
```json
{
  "error": "authentication_required",
  "message": "Please provide a valid API key in the X-API-Key header"
}
```

## Rate Limiting

- Management endpoints: 100 requests/minute per user
- Proxy endpoints: 1000 requests/minute per API key
- Login attempts: 5 attempts per 15 minutes per IP

## Security Best Practices

1. **Store API keys securely** - Never commit API keys to code repositories
2. **Use environment variables** - Store sensitive configuration in environment variables
3. **Rotate keys regularly** - Create new API keys and revoke old ones periodically
4. **Monitor usage** - Use the analytics endpoints to monitor API usage
5. **Enable 2FA** - Enable two-factor authentication for admin accounts
6. **Use HTTPS** - Always use HTTPS in production environments

## Support

For technical support, licensing questions, or feature requests:
- Email: licensing@sovereignprivacy.com
- Documentation: http://localhost:8080/api/docs
- Issues: Report bugs via your organization's support channel

## License

This software uses an OpenVPN-style commercial license:
- ✅ Free for personal, academic, research use
- ✅ Free for internal business use within single organization
- ❌ Commercial redistribution/SaaS offerings require licensing
- Contact: licensing@sovereignprivacy.com