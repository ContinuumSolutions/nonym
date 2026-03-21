#!/usr/bin/env python3
"""
Test PII Detection in Privacy Gateway
This demo sends requests with PII to test detection without requiring real API keys
"""
import requests
import json

# Gateway URL
gateway_url = "https://gateway.nonym.io/v1/chat/completions"

# Test messages with different types of PII
test_cases = [
    {
        "name": "Clean Text (Should Pass)",
        "message": "What's the weather like today?",
        "expected": "pass"
    },
    {
        "name": "Low-Risk PII (Email)",
        "message": "Contact me at john.doe@email.com for more info",
        "expected": "anonymize"
    },
    {
        "name": "Single Credit Card (High Risk)",
        "message": "My card number is 4242-4242-4242-4242",
        "expected": "block_or_anonymize"
    },
    {
        "name": "Single SSN (Critical Risk)",
        "message": "My SSN is 123-45-6789 for verification",
        "expected": "block_or_anonymize"
    },
    {
        "name": "API Key (Critical Risk - SHOULD BLOCK)",
        "message": "Use this API key: sk-proj-abc123XYZ456def789ghi012jkl345mno678pqr901stu234vwx567yza890bcd",
        "expected": "block"
    },
    {
        "name": "AWS Credentials (Critical Risk - SHOULD BLOCK)",
        "message": "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE and AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
        "expected": "block"
    },
    {
        "name": "Multiple Critical PII (SHOULD BLOCK)",
        "message": "My SSN is 123-45-6789, credit card 4532-1234-5678-9012, and API key sk-abc123def456",
        "expected": "block"
    },
    {
        "name": "Database Connection String (SHOULD BLOCK)",
        "message": "Connect using: postgres://admin:password123@db.company.com:5432/production_db",
        "expected": "block"
    },
    {
        "name": "Private Key (SHOULD BLOCK)",
        "message": "-----BEGIN PRIVATE KEY-----\nMIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQC7VJTUt9Us8cKB\n-----END PRIVATE KEY-----",
        "expected": "block"
    },
    {
        "name": "Financial Data Combo (SHOULD BLOCK)",
        "message": "Account: 1234567890, SSN: 987-65-4321, CVV: 123, routing: 021000021",
        "expected": "block"
    },
    {
        "name": "Personal Data Overload (SHOULD BLOCK)",
        "message": "John Smith, SSN 555-44-3333, DOB 01/15/1980, Card 5555-4444-3333-2222, Phone 555-0123, Email john.smith@company.com, Address 123 Main St, Driver License D1234567",
        "expected": "block"
    }
]

for test in test_cases:
    print(f"\n🧪 Testing: {test['name']}")
    print(f"Expected: {test['expected']}")
    print(f"Original: {test['message'][:100]}{'...' if len(test['message']) > 100 else ''}")

    payload = {
        "model": "gpt-3.5-turbo",
        "messages": [{"role": "user", "content": test['message']}]
    }

    try:
        response = requests.post(
            gateway_url,
            headers={
                "Authorization": "Bearer OPENAI-KEY/ANTHROPIC",
                "X-API-Key": "gateway_api_key",
                "X-Strict-Mode": "true",  # Request strict mode for blocking behavior
                "X-Block-Critical-PII": "true"  # Additional header to trigger blocking
            },
            json=payload,
            timeout=10
        )

        print(f"Status: {response.status_code}")

        # Analyze response status and content
        if response.status_code == 200:
            try:
                response_json = response.json()
                # Check if message was modified (anonymized)
                if 'choices' in response_json and len(response_json['choices']) > 0:
                    response_content = response_json['choices'][0].get('message', {}).get('content', '')
                    if response_content and test['message'] not in response_content:
                        print("🔒 PII was anonymized/redacted in response")
                    else:
                        print("✅ Clean request passed through")
                else:
                    print("⚠️  Unexpected response format")
            except:
                print(f"📝 Response: {response.text[:150]}...")
        elif response.status_code == 401:
            print("✅ Request reached provider (expected 401 with test key)")
        elif response.status_code == 403:
            print("🛡️  REQUEST BLOCKED by gateway (high PII risk detected)")
            try:
                error_response = response.json()
                if 'error' in error_response:
                    print(f"   Reason: {error_response['error']}")
            except:
                print(f"   Response: {response.text}")
        elif response.status_code == 400:
            print("⚠️  Bad request - possible PII validation error")
            print(f"   Response: {response.text}")
        elif response.status_code >= 500:
            print("💥 Server error")
            print(f"   Response: {response.text[:100]}...")
        else:
            print(f"❓ Unexpected status: {response.status_code}")
            print(f"   Response: {response.text[:150]}...")

        # Show result vs expectation
        actual_result = "unknown"
        if response.status_code == 403:
            actual_result = "blocked"
        elif response.status_code == 200:
            actual_result = "passed/anonymized"
        elif response.status_code == 401:
            actual_result = "reached_provider"

        expectation_met = "❓"
        if test['expected'] == "block" and actual_result == "blocked":
            expectation_met = "✅ BLOCKED as expected"
        elif test['expected'] == "pass" and actual_result in ["passed/anonymized", "reached_provider"]:
            expectation_met = "✅ PASSED as expected"
        elif test['expected'] == "block" and actual_result != "blocked":
            expectation_met = "❌ Should have been BLOCKED"
        elif test['expected'] in ["block_or_anonymize", "anonymize"]:
            expectation_met = "ℹ️  Result may vary by configuration"

        print(f"   Result: {expectation_met}")

    except Exception as e:
        print(f"💥 Error: {e}")

print("\n" + "="*50)
print("📊 TEST SUMMARY")
print("="*50)
print("✅ = Working as expected")
print("❌ = Not working as expected (check configuration)")
print("ℹ️  = Behavior depends on gateway configuration")

print("\n💡 To view detailed PII detection logs:")
print("docker compose logs gateway --tail=50 | grep -i 'pii\\|block\\|redact'")

print("\n🔧 To enable strict blocking mode, ensure gateway configuration has:")
print("- strict_mode: true")
print("- blocked_entities: ['ssn', 'credit_card', 'api_key', 'private_key']")
print("- Or use environment variable: GATEWAY_STRICT_MODE=true")

print("\n📈 To check audit logs and events:")
print("curl -H 'X-API-Key: gateway_api_key' \\")
print("     https://gateway.nonym.io/api/v1/protection-events?limit=10")

print("\n🎯 Expected blocking triggers:")
print("- API keys, private keys, certificates")
print("- Multiple critical PII types in single request")
print("- Database connection strings, credentials")
print("- Financial data combinations (SSN + Card + CVV)")
print("- When strict_mode is enabled in gateway settings")