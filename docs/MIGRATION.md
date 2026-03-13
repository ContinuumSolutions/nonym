# Migration Guide: Upgrading to API Key Authentication

This guide helps you migrate from earlier versions of the Sovereign Privacy Gateway to the new secure API key authentication model.

## 🔄 What Changed in v2.0

### Security Enhancement
- **API Key Authentication**: All proxy requests now require SPG API key authentication
- **Improved Security**: Prevents unauthorized access to your privacy gateway
- **Access Control**: Fine-grained permissions for different API keys
- **Audit Trail**: Better tracking of who's using your gateway

### Breaking Changes
- ❌ **Unauthenticated requests** to proxy endpoints (`/v1/*`, `/api/*`) are no longer allowed
- ✅ **Dashboard and health endpoints** remain unchanged
- ✅ **All existing AI provider integrations** work the same way after adding the API key

## 🚀 Migration Steps

### Step 1: Generate Your SPG API Key

1. **Start the Gateway**: Ensure your gateway is running
2. **Access Dashboard**: Go to `http://localhost:8081` (or your dashboard URL)
3. **Login**: Use your existing credentials
4. **Navigate to Integrations**: Click "Integrations" in the sidebar
5. **Generate API Key**:
   - Go to the "API Keys" tab
   - Click "Generate New API Key"
   - Give it a name (e.g., "Migration Key", "Production Access")
   - Select permissions (typically "write" for full access)
   - **Copy the key** - it starts with `spg_` and you'll only see it once

### Step 2: Update Your Client Code

#### Before (v1.x)
```python
import openai

openai.api_base = "http://localhost:8080/v1"
openai.api_key = "your-openai-key"

# This worked without SPG authentication
response = openai.ChatCompletion.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Hello"}]
)
```

#### After (v2.0+)
```python
import openai

openai.api_base = "http://localhost:8080/v1"
openai.api_key = "your-openai-key"

# NEW: Add SPG API key authentication
openai.default_headers = {
    "X-API-Key": "spg_your_gateway_api_key_here"
}

# Now works with SPG authentication
response = openai.ChatCompletion.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Hello"}]
)
```

### Step 3: Update Environment Variables

Add your SPG API key to your environment:

```bash
# Add to your .env file
SPG_API_KEY=spg_your_gateway_api_key_here

# Your existing keys remain the same
OPENAI_API_KEY=sk-your-openai-key
ANTHROPIC_API_KEY=sk-ant-your-anthropic-key
```

### Step 4: Update Application Configuration

#### Docker Compose
```yaml
# Update your docker-compose.yml
services:
  your-app:
    environment:
      - SPG_API_KEY=${SPG_API_KEY}  # Add this line
      - OPENAI_API_KEY=${OPENAI_API_KEY}
```

#### Kubernetes
```yaml
# Update your deployment
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: your-app
        env:
        - name: SPG_API_KEY
          value: "spg_your_gateway_api_key_here"
        - name: OPENAI_API_KEY
          value: "sk-your-openai-key"
```

## 📝 Language-Specific Migration

### Python (OpenAI SDK)

**Before:**
```python
import openai
openai.api_base = "http://localhost:8080/v1"
```

**After:**
```python
import openai
import os

openai.api_base = "http://localhost:8080/v1"
openai.default_headers = {
    "X-API-Key": os.getenv("SPG_API_KEY")
}
```

### Python (Requests)

**Before:**
```python
headers = {
    "Authorization": f"Bearer {openai_key}",
    "Content-Type": "application/json"
}
```

**After:**
```python
headers = {
    "X-API-Key": os.getenv("SPG_API_KEY"),  # Add this
    "Authorization": f"Bearer {openai_key}",
    "Content-Type": "application/json"
}
```

### JavaScript/Node.js

**Before:**
```javascript
const openai = new OpenAI({
  apiKey: process.env.OPENAI_API_KEY,
  baseURL: 'http://localhost:8080/v1'
});
```

**After:**
```javascript
const openai = new OpenAI({
  apiKey: process.env.OPENAI_API_KEY,
  baseURL: 'http://localhost:8080/v1',
  defaultHeaders: {
    'X-API-Key': process.env.SPG_API_KEY  // Add this
  }
});
```

### cURL

**Before:**
```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer your-openai-key" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4", "messages": [...]}'
```

**After:**
```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "X-API-Key: spg_your_gateway_api_key_here" \
  -H "Authorization: Bearer your-openai-key" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4", "messages": [...]}'
```

## 🔧 Testing Your Migration

### 1. Test Authentication
```bash
# This should now return 401 without SPG API key
curl http://localhost:8080/v1/chat/completions

# This should work with SPG API key
curl -H "X-API-Key: spg_your_key" http://localhost:8080/v1/chat/completions
```

### 2. Test Your Application
Run your application with the new API key configuration and verify:
- ✅ Requests succeed with proper authentication
- ✅ PII protection still works as expected
- ✅ Responses are properly de-anonymized

### 3. Check Dashboard
- Visit your dashboard to confirm API usage is being tracked
- Verify your API key shows "Last Used" timestamp
- Check protection events are still being logged

## 🚨 Troubleshooting

### Common Errors

#### 401 Authentication Required
```json
{
  "error": "Authentication required",
  "message": "This Sovereign Privacy Gateway requires an API key for access. Please include your SPG API key in the X-API-Key header.",
  "documentation": "Visit the dashboard to generate your API key at /integrations"
}
```

**Solution**: Add the `X-API-Key` header with your SPG API key.

#### 401 Invalid API Key
```json
{
  "error": "Invalid API key",
  "message": "The provided API key is invalid, expired, or revoked. Please check your key and try again."
}
```

**Solutions**:
- Check your API key is correct and starts with `spg_`
- Verify the API key hasn't been revoked in the dashboard
- Generate a new API key if necessary

#### Headers Not Being Sent
```python
# Common mistake - headers not properly configured
openai.api_base = "http://localhost:8080/v1"
# Missing: openai.default_headers = {"X-API-Key": "..."}
```

**Solution**: Ensure you're setting headers correctly for your HTTP client.

### Health Check Still Works
The health check endpoint doesn't require authentication:
```bash
curl http://localhost:8080/health
# Should return: {"status": "healthy", ...}
```

### Dashboard Access Unchanged
Your dashboard login and functionality remain the same:
- Same login credentials
- Same URL (typically `:8081`)
- All features available

## 🔐 Security Best Practices

### 1. API Key Management
- **Generate separate keys** for different environments (dev, staging, prod)
- **Use descriptive names** to track key usage
- **Rotate keys regularly** (monthly or quarterly)
- **Revoke unused keys** to reduce attack surface

### 2. Storage Security
- **Environment variables**: Store keys in environment variables, not code
- **Secret management**: Use proper secret management in production (AWS Secrets Manager, Azure Key Vault, etc.)
- **Never commit keys**: Add `.env` to `.gitignore`

### 3. Monitoring
- **Regular audits**: Review API key usage in the dashboard
- **Monitor logs**: Watch for failed authentication attempts
- **Set up alerts**: Configure notifications for unusual activity

## 📈 Benefits After Migration

### Enhanced Security
- **Controlled access** to your privacy gateway
- **Audit trail** of all API usage
- **Prevention** of unauthorized data processing

### Better Monitoring
- **User attribution** in logs and analytics
- **Usage tracking** per API key
- **Performance metrics** per client

### Future Features
- **Rate limiting** per API key (coming soon)
- **Quota management** and billing (coming soon)
- **Advanced permissions** and scoping (coming soon)

## 🆘 Getting Help

If you encounter issues during migration:

1. **Check the logs**: `docker compose logs gateway` for error details
2. **Review this guide**: Ensure all steps were followed correctly
3. **Test incrementally**: Start with a simple request before complex workflows
4. **Community support**:
   - [GitHub Issues](https://github.com/sovereignprivacy/gateway/issues)
   - [GitHub Discussions](https://github.com/sovereignprivacy/gateway/discussions)
5. **Commercial support**: support@sovereignprivacy.com

## 📋 Migration Checklist

- [ ] **Generate SPG API key** via dashboard
- [ ] **Copy API key** securely
- [ ] **Update environment variables** with SPG_API_KEY
- [ ] **Modify client code** to include X-API-Key header
- [ ] **Test authentication** with simple request
- [ ] **Run full application** test
- [ ] **Verify PII protection** still works
- [ ] **Check dashboard** for API usage tracking
- [ ] **Update documentation** for your team
- [ ] **Deploy to staging** environment
- [ ] **Deploy to production** environment

---

**Need help?** Contact us at support@sovereignprivacy.com or create an issue on GitHub.