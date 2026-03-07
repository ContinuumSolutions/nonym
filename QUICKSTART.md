# Quick Start Guide - Sovereign Privacy Gateway

Get up and running with the Privacy Gateway in under 5 minutes.

## Prerequisites

- Docker & Docker Compose installed
- At least one AI provider API key

## 1. Get API Keys

Choose at least one provider and get an API key:

### OpenAI (Recommended for general use)
1. Go to [OpenAI API Keys](https://platform.openai.com/api-keys)
2. Click "Create new secret key"
3. Copy the key (starts with `sk-`)

### Anthropic (Recommended for privacy-sensitive data)
1. Go to [Anthropic Console](https://console.anthropic.com/)
2. Navigate to "API Keys"
3. Create a new key (starts with `sk-ant-`)

### Google AI (Good for embeddings)
1. Go to [Google AI Studio](https://aistudio.google.com/app/apikey)
2. Create an API key

## 2. Quick Setup

```bash
# Clone repository
git clone https://github.com/sovereignprivacy/gateway
cd gateway

# Run setup script
./scripts/setup-production.sh

# Add your API keys to .env
nano .env
```

Add at least one API key:
```bash
OPENAI_API_KEY=sk-your-openai-key-here
ANTHROPIC_API_KEY=sk-ant-your-anthropic-key-here
GOOGLE_API_KEY=your-google-key-here
```

## 3. Start Gateway

```bash
# Development mode (recommended for testing)
docker compose up -d

# Production mode
docker compose -f docker-compose.prod.yml up -d
```

## 4. Verify Setup

```bash
# Health check
curl http://localhost:8080/health

# Test PII detection
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-openai-key" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [
      {
        "role": "user",
        "content": "My email is john@example.com"
      }
    ],
    "max_tokens": 10
  }'
```

## 5. Access Dashboard

Visit **http://localhost:8081** to see:
- Real-time request monitoring
- PII detection statistics
- Provider performance
- Privacy compliance metrics

## What's Protected

The gateway automatically detects and anonymizes:
- ✅ Email addresses
- ✅ Social Security Numbers (SSN)
- ✅ Credit card numbers
- ✅ API keys and tokens
- ✅ Phone numbers
- ✅ IP addresses

## How It Works

```
Your App → Privacy Gateway → AI Provider
          (detects PII)     (anonymized data)
          ↓
          Dashboard
          (monitoring)
```

1. **Detect**: Gateway scans requests for PII
2. **Anonymize**: Replaces PII with tokens like `{{EMAIL_ABC123}}`
3. **Route**: Sends anonymized request to AI provider
4. **Restore**: Converts tokens back to original data in response
5. **Monitor**: Logs everything for compliance

## Next Steps

- **Configure monitoring**: `docker compose -f docker-compose.prod.yml --profile monitoring up -d`
- **Read full docs**: [Installation Guide](docs/installation.md)
- **Test thoroughly**: `./scripts/verify-setup.sh`
- **Set up SSL**: See production deployment section

## Troubleshooting

### Gateway won't start
```bash
# Check logs
docker compose logs gateway

# Common issues:
# - Invalid API key format
# - Port 8080 already in use
```

### API calls failing
```bash
# Check health
curl http://localhost:8080/health

# Check if API key is correct
echo $OPENAI_API_KEY  # Should start with sk-
```

### Need help?
- Check [docs/installation.md](docs/installation.md) for detailed instructions
- Run `./scripts/verify-setup.sh` for diagnostic information
- Open an issue on GitHub

---

**You're now protecting your AI interactions with enterprise-grade privacy!**
