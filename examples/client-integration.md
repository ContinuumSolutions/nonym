# Client Integration Examples

This document provides examples of how to integrate your applications with the Sovereign Privacy Gateway using various programming languages and frameworks.

## 🔑 Authentication Requirements

**Starting with version 2.0, all proxy requests require SPG API key authentication:**

1. **Generate API Key**: Visit your dashboard at `/integrations` → API Keys → Generate Key
2. **Include X-API-Key Header**: Add your SPG API key to all requests
3. **Provider API Key**: Still include your AI provider's API key as usual

## Python Examples

### OpenAI SDK with SPG

```python
import openai
import os

# Configure OpenAI to use SPG as proxy
openai.api_base = "http://localhost:8080/v1"

# REQUIRED: SPG API key for gateway authentication
openai.default_headers = {
    "X-API-Key": os.getenv("SPG_API_KEY")  # spg_your_key_here
}

# Your OpenAI API key (passed in Authorization header)
openai.api_key = os.getenv("OPENAI_API_KEY")

# Use normally - PII protection is automatic
response = openai.ChatCompletion.create(
    model="gpt-4",
    messages=[
        {"role": "user", "content": "My email is john@company.com, process this data"}
    ]
)

print(response.choices[0].message.content)
```

### Requests Library

```python
import requests
import os

def call_ai_with_privacy(prompt, model="gpt-4"):
    url = "http://localhost:8080/v1/chat/completions"

    headers = {
        "Content-Type": "application/json",
        "X-API-Key": os.getenv("SPG_API_KEY"),          # SPG authentication
        "Authorization": f"Bearer {os.getenv('OPENAI_API_KEY')}"  # OpenAI key
    }

    payload = {
        "model": model,
        "messages": [{"role": "user", "content": prompt}]
    }

    response = requests.post(url, json=payload, headers=headers)
    return response.json()

# Example with PII that will be automatically protected
result = call_ai_with_privacy(
    "Analyze this customer: email john@company.com, SSN 123-45-6789"
)
```

### Anthropic SDK with SPG

```python
import anthropic
import os

# Create client pointing to SPG
client = anthropic.Anthropic(
    api_key=os.getenv("ANTHROPIC_API_KEY"),
    base_url="http://localhost:8080/v1",  # Point to SPG
    default_headers={
        "X-API-Key": os.getenv("SPG_API_KEY")  # SPG authentication
    }
)

# Use normally - PII protection is automatic
message = client.messages.create(
    model="claude-3-sonnet",
    max_tokens=1000,
    messages=[
        {"role": "user", "content": "Review this customer data with PII..."}
    ]
)

print(message.content[0].text)
```

## JavaScript/Node.js Examples

### OpenAI SDK

```javascript
import OpenAI from 'openai';

const openai = new OpenAI({
  apiKey: process.env.OPENAI_API_KEY,
  baseURL: 'http://localhost:8080/v1',  // Point to SPG
  defaultHeaders: {
    'X-API-Key': process.env.SPG_API_KEY  // SPG authentication
  }
});

async function chatWithPII() {
  const completion = await openai.chat.completions.create({
    messages: [
      {
        role: "user",
        content: "Process this customer: email john@company.com, phone 555-123-4567"
      }
    ],
    model: "gpt-4",
  });

  console.log(completion.choices[0].message.content);
}

chatWithPII();
```

### Fetch API

```javascript
async function callAIThroughGateway(prompt) {
  const response = await fetch('http://localhost:8080/v1/chat/completions', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-API-Key': process.env.SPG_API_KEY,           // SPG authentication
      'Authorization': `Bearer ${process.env.OPENAI_API_KEY}`  // OpenAI key
    },
    body: JSON.stringify({
      model: 'gpt-4',
      messages: [
        { role: 'user', content: prompt }
      ]
    })
  });

  return response.json();
}

// Example usage
const result = await callAIThroughGateway(
  "Analyze customer data: email jane@company.com, credit card 4111-1111-1111-1111"
);
```

## cURL Examples

### OpenAI Chat Completions

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-API-Key: spg_your_gateway_api_key_here" \
  -H "Authorization: Bearer your-openai-api-key" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "My customer info: email john@company.com, SSN 123-45-6789, CC 4111-1111-1111-1111"
      }
    ]
  }'
```

### Anthropic Messages

```bash
curl -X POST http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "X-API-Key: spg_your_gateway_api_key_here" \
  -H "x-api-key: your-anthropic-api-key" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-sonnet-20240229",
    "max_tokens": 1000,
    "messages": [
      {
        "role": "user",
        "content": "Process customer data: email jane@company.com, phone 555-123-4567"
      }
    ]
  }'
```

### Health Check (No Auth Required)

```bash
curl http://localhost:8080/health
```

## Go Examples

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
)

type ChatRequest struct {
    Model    string    `json:"model"`
    Messages []Message `json:"messages"`
}

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

func callAIThroughGateway(prompt string) error {
    req := ChatRequest{
        Model: "gpt-4",
        Messages: []Message{
            {Role: "user", Content: prompt},
        },
    }

    jsonData, _ := json.Marshal(req)

    httpReq, err := http.NewRequest("POST", "http://localhost:8080/v1/chat/completions", bytes.NewBuffer(jsonData))
    if err != nil {
        return err
    }

    // Required headers
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("X-API-Key", os.Getenv("SPG_API_KEY"))                    // SPG authentication
    httpReq.Header.Set("Authorization", "Bearer "+os.Getenv("OPENAI_API_KEY"))  // OpenAI key

    client := &http.Client{}
    resp, err := client.Do(httpReq)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    // Process response...
    return nil
}

func main() {
    err := callAIThroughGateway("Customer data: email john@company.com, SSN 123-45-6789")
    if err != nil {
        fmt.Printf("Error: %v\n", err)
    }
}
```

## Environment Variables Setup

Create a `.env` file for your application:

```bash
# SPG Gateway Configuration
SPG_API_KEY=spg_your_gateway_api_key_here
SPG_BASE_URL=http://localhost:8080

# AI Provider Keys (your existing keys)
OPENAI_API_KEY=sk-your-openai-key
ANTHROPIC_API_KEY=sk-ant-your-anthropic-key
GOOGLE_API_KEY=your-google-ai-key
```

## Docker Compose Integration

If running SPG in production, update your `docker-compose.yml`:

```yaml
version: '3.8'
services:
  your-app:
    build: .
    environment:
      - SPG_API_KEY=${SPG_API_KEY}
      - SPG_BASE_URL=http://sovereign-privacy-gateway:8080
      - OPENAI_API_KEY=${OPENAI_API_KEY}
    depends_on:
      - sovereign-privacy-gateway
    networks:
      - spg-network

  sovereign-privacy-gateway:
    image: sovereignprivacy/gateway:latest
    ports:
      - "8080:8080"
    networks:
      - spg-network

networks:
  spg-network:
    driver: bridge
```

## Migration from Previous Versions

### Before (v1.x)
```python
# No authentication required
openai.api_base = "http://localhost:8080/v1"
```

### After (v2.0+)
```python
# API key authentication required
openai.api_base = "http://localhost:8080/v1"
openai.default_headers = {
    "X-API-Key": "spg_your_gateway_api_key_here"
}
```

## Error Handling

The gateway returns structured error responses for authentication issues:

```python
import requests

try:
    response = requests.post(url, json=payload, headers=headers)
    response.raise_for_status()
except requests.exceptions.HTTPError as e:
    if response.status_code == 401:
        error_data = response.json()
        print(f"Authentication Error: {error_data['message']}")
        print(f"Documentation: {error_data['documentation']}")
    else:
        print(f"Other error: {e}")
```

## Best Practices

1. **Store API Keys Securely**: Use environment variables or secure key management
2. **Handle Authentication Errors**: Implement proper error handling for 401 responses
3. **Monitor Usage**: Use the SPG dashboard to monitor your API usage
4. **Rotate Keys Regularly**: Generate new SPG API keys periodically
5. **Use HTTPS in Production**: Always use HTTPS for production deployments

## Support

For issues with client integration:
- Check the [API documentation](https://docs.sovereignprivacy.com)
- Review [GitHub Issues](https://github.com/sovereignprivacy/gateway/issues)
- Contact support: support@sovereignprivacy.com