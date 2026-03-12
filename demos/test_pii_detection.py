#!/usr/bin/env python3
"""
Test PII Detection in Privacy Gateway
This demo sends requests with PII to test detection without requiring real API keys
"""
import requests
import json

# Gateway URL
gateway_url = "https://genesis.egokernel.com/v1/chat/completions"

# Test messages with different types of PII
test_cases = [
    {
        "name": "Credit Card",
        "message": "My card number is 4242-4242-4242-4242"
    },
    {
        "name": "Email & Phone",
        "message": "Contact me at john.doe@email.com or call 555-123-4567"
    },
    {
        "name": "SSN",
        "message": "My SSN is 123-45-6789 for verification"
    },
    {
        "name": "Clean Text",
        "message": "What's the weather like today?"
    }
]

for test in test_cases:
    print(f"\n🧪 Testing: {test['name']}")
    print(f"Original: {test['message']}")

    payload = {
        "model": "gpt-3.5-turbo",
        "messages": [{"role": "user", "content": test['message']}]
    }

    try:
        response = requests.post(
            gateway_url,
            headers={
                # "Authorization": "Bearer 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",
                "Authorization": "Bearer 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",
                "Content-Type": "application/json"
            },
            json=payload,
            timeout=10
        )

        print(f"Status: {response.status_code}")

        if response.status_code == 401:
            print("✅ Request reached OpenAI (expected 401 with test key)")
        elif response.status_code == 403:
            print("🛡️  Request blocked by strict mode (high PII risk)")
        else:
            print(f"Response: {response.text[:200]}...")

    except Exception as e:
        print(f"Error: {e}")

print("\n💡 Check gateway logs for PII detection details:")
print("docker compose logs gateway --tail=20")