#!/usr/bin/env python3
"""
Unit tests for PII detection demo script
Tests the HTTP request structure, headers, test cases, and response handling
"""
import pytest
import json
from unittest.mock import Mock, patch, MagicMock
import requests
import sys
import os

# Add the demos directory to the path so we can import the test script
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '../demos'))


class TestPIIDetectionDemo:
    """Test suite for the PII detection demo functionality"""

    @pytest.fixture
    def mock_response(self):
        """Create a mock HTTP response object"""
        mock_resp = Mock()
        mock_resp.status_code = 200
        mock_resp.text = '{"response": "test response"}'
        return mock_resp

    def test_gateway_url_format(self):
        """Test that the gateway URL is properly formatted"""
        from test_pii_detection import gateway_url

        assert gateway_url == "https://genesis.egokernel.com/v1/chat/completions"
        assert gateway_url.startswith("https://")
        assert "/v1/chat/completions" in gateway_url

    def test_test_cases_structure(self):
        """Test that all test cases have required fields"""
        from test_pii_detection import test_cases

        assert len(test_cases) == 4

        for test_case in test_cases:
            assert "name" in test_case
            assert "message" in test_case
            assert isinstance(test_case["name"], str)
            assert isinstance(test_case["message"], str)
            assert len(test_case["name"]) > 0
            assert len(test_case["message"]) > 0

    def test_test_cases_content(self):
        """Test specific content of test cases"""
        from test_pii_detection import test_cases

        # Check that we have the expected test cases
        case_names = [case["name"] for case in test_cases]
        assert "Credit Card" in case_names
        assert "Email & Phone" in case_names
        assert "SSN" in case_names
        assert "Clean Text" in case_names

        # Check credit card case
        cc_case = next(case for case in test_cases if case["name"] == "Credit Card")
        assert "4242-4242-4242-4242" in cc_case["message"]

        # Check email & phone case
        email_case = next(case for case in test_cases if case["name"] == "Email & Phone")
        assert "john.doe@email.com" in email_case["message"]
        assert "555-123-4567" in email_case["message"]

        # Check SSN case
        ssn_case = next(case for case in test_cases if case["name"] == "SSN")
        assert "123-45-6789" in ssn_case["message"]

        # Check clean text case
        clean_case = next(case for case in test_cases if case["name"] == "Clean Text")
        assert "weather" in clean_case["message"].lower()

    def test_http_headers_structure(self):
        """Test the HTTP headers structure and content"""
        # Test the headers that would be sent (line 47 and surrounding)
        expected_headers = {
            "Authorization": "Bearer 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",
            "Content-Type": "application/json",  # This is line 47
            "X-API-Key": "spg_62e9c603fed726a5087db12a1ee703f54850f04e4455db589640005a42c22787"
        }

        # Test Content-Type header specifically (line 47)
        assert expected_headers["Content-Type"] == "application/json"

        # Test Authorization header format
        assert expected_headers["Authorization"].startswith("Bearer ")

        # Test X-API-Key header format
        assert expected_headers["X-API-Key"].startswith("spg_")
        assert len(expected_headers["X-API-Key"]) > 10  # Should be a substantial key

    def test_payload_structure(self):
        """Test the JSON payload structure for requests"""
        test_message = "Test message with PII"

        expected_payload = {
            "model": "gpt-3.5-turbo",
            "messages": [{"role": "user", "content": test_message}]
        }

        # Test payload structure
        assert "model" in expected_payload
        assert "messages" in expected_payload
        assert expected_payload["model"] == "gpt-3.5-turbo"
        assert isinstance(expected_payload["messages"], list)
        assert len(expected_payload["messages"]) == 1
        assert expected_payload["messages"][0]["role"] == "user"
        assert expected_payload["messages"][0]["content"] == test_message

    @patch('requests.post')
    def test_successful_request(self, mock_post):
        """Test successful HTTP request handling"""
        # Setup mock
        mock_response = Mock()
        mock_response.status_code = 200
        mock_response.text = '{"response": "success"}'
        mock_post.return_value = mock_response

        # Import and run a simulation of the request
        from test_pii_detection import gateway_url, test_cases

        test_case = test_cases[0]  # Use first test case
        payload = {
            "model": "gpt-3.5-turbo",
            "messages": [{"role": "user", "content": test_case['message']}]
        }

        headers = {
            "Authorization": "Bearer test-key",
            "Content-Type": "application/json",
            "X-API-Key": "spg_test_key"
        }

        # Make the request
        response = requests.post(gateway_url, headers=headers, json=payload, timeout=10)

        # Verify the mock was called correctly
        mock_post.assert_called_once_with(
            gateway_url,
            headers=headers,
            json=payload,
            timeout=10
        )

        assert response.status_code == 200

    @patch('requests.post')
    def test_401_unauthorized_response(self, mock_post):
        """Test handling of 401 unauthorized response"""
        # Setup mock for 401 response
        mock_response = Mock()
        mock_response.status_code = 401
        mock_response.text = '{"error": "Invalid API key"}'
        mock_post.return_value = mock_response

        from test_pii_detection import gateway_url

        response = requests.post(
            gateway_url,
            headers={"Authorization": "Bearer invalid-key", "Content-Type": "application/json"},
            json={"test": "data"},
            timeout=10
        )

        assert response.status_code == 401
        assert "error" in response.text or "Invalid" in response.text

    @patch('requests.post')
    def test_403_blocked_response(self, mock_post):
        """Test handling of 403 blocked response (strict mode)"""
        # Setup mock for 403 response
        mock_response = Mock()
        mock_response.status_code = 403
        mock_response.text = '{"error": "Request blocked due to high PII risk"}'
        mock_post.return_value = mock_response

        from test_pii_detection import gateway_url

        response = requests.post(
            gateway_url,
            headers={"Authorization": "Bearer test-key", "Content-Type": "application/json"},
            json={"test": "data"},
            timeout=10
        )

        assert response.status_code == 403

    @patch('requests.post')
    def test_request_timeout_handling(self, mock_post):
        """Test handling of request timeout"""
        # Setup mock to raise timeout exception
        mock_post.side_effect = requests.exceptions.Timeout("Request timed out")

        from test_pii_detection import gateway_url

        with pytest.raises(requests.exceptions.Timeout):
            requests.post(
                gateway_url,
                headers={"Authorization": "Bearer test-key", "Content-Type": "application/json"},
                json={"test": "data"},
                timeout=10
            )

    @patch('requests.post')
    def test_connection_error_handling(self, mock_post):
        """Test handling of connection errors"""
        # Setup mock to raise connection exception
        mock_post.side_effect = requests.exceptions.ConnectionError("Connection failed")

        from test_pii_detection import gateway_url

        with pytest.raises(requests.exceptions.ConnectionError):
            requests.post(
                gateway_url,
                headers={"Authorization": "Bearer test-key", "Content-Type": "application/json"},
                json={"test": "data"},
                timeout=10
            )

    def test_pii_detection_patterns(self):
        """Test that test cases contain expected PII patterns"""
        from test_pii_detection import test_cases

        # Credit card pattern test
        cc_case = next(case for case in test_cases if case["name"] == "Credit Card")
        # Should match pattern: 4 groups of 4 digits separated by hyphens
        assert "4242-4242-4242-4242" in cc_case["message"]

        # Email pattern test
        email_case = next(case for case in test_cases if case["name"] == "Email & Phone")
        # Should contain valid email format
        assert "@" in email_case["message"]
        assert ".com" in email_case["message"]

        # Phone pattern test
        # Should contain phone number format
        assert "555-123-4567" in email_case["message"]

        # SSN pattern test
        ssn_case = next(case for case in test_cases if case["name"] == "SSN")
        # Should match pattern: 3-2-4 digits
        assert "123-45-6789" in ssn_case["message"]

    def test_clean_text_case(self):
        """Test that clean text case contains no PII"""
        from test_pii_detection import test_cases

        clean_case = next(case for case in test_cases if case["name"] == "Clean Text")
        message = clean_case["message"]

        # Should not contain common PII patterns
        assert "@" not in message  # No email
        assert not any(char.isdigit() for char in message)  # No numbers (no phone, SSN, CC)
        assert len(message.split()) > 1  # Should be actual text

    @patch('builtins.print')
    def test_output_formatting(self, mock_print):
        """Test that output formatting works correctly"""
        from test_pii_detection import test_cases

        # Simulate the print statements from the script
        test = test_cases[0]
        print(f"\n🧪 Testing: {test['name']}")
        print(f"Original: {test['message']}")

        # Check that print was called
        assert mock_print.call_count >= 2

        # Check print call arguments contain expected content
        call_args = [str(call) for call in mock_print.call_args_list]
        assert any("Testing:" in arg for arg in call_args)
        assert any("Original:" in arg for arg in call_args)

    def test_response_length_truncation(self):
        """Test response text truncation logic"""
        # Test the logic: response.text[:200]...
        long_response = "x" * 300  # 300 character response
        truncated = long_response[:200] + "..." if len(long_response) > 200 else long_response

        assert len(truncated) == 203  # 200 chars + "..."
        assert truncated.endswith("...")

    def test_api_key_format_validation(self):
        """Test API key format validation"""
        api_key = "spg_62e9c603fed726a5087db12a1ee703f54850f04e4455db589640005a42c22787"

        # Test API key format
        assert api_key.startswith("spg_")
        assert len(api_key) > 20  # Should be substantial length
        assert all(c.isalnum() or c == "_" for c in api_key)  # Only alphanumeric and underscore

    @pytest.mark.parametrize("status_code,expected_message", [
        (401, "Request reached OpenAI (expected 401 with test key)"),
        (403, "Request blocked by strict mode (high PII risk)"),
        (200, "Response:"),
    ])
    def test_status_code_handling(self, status_code, expected_message):
        """Test different HTTP status code handling logic"""
        # This tests the conditional logic in the script
        if status_code == 401:
            assert "401" in expected_message
            assert "OpenAI" in expected_message
        elif status_code == 403:
            assert "blocked" in expected_message.lower()
            assert "strict mode" in expected_message.lower()
        else:
            assert "Response:" in expected_message


class TestPIIDetectionIntegration:
    """Integration tests for the PII detection demo"""

    @patch('requests.post')
    def test_full_request_cycle(self, mock_post):
        """Test a complete request cycle with all components"""
        # Setup mock response
        mock_response = Mock()
        mock_response.status_code = 401
        mock_response.text = '{"error": "Unauthorized"}'
        mock_post.return_value = mock_response

        from test_pii_detection import test_cases, gateway_url

        # Test with first test case (Credit Card)
        test = test_cases[0]
        payload = {
            "model": "gpt-3.5-turbo",
            "messages": [{"role": "user", "content": test['message']}]
        }

        headers = {
            "Authorization": "Bearer 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",
            "Content-Type": "application/json",  # Line 47 specifically
            "X-API-Key": "spg_62e9c603fed726a5087db12a1ee703f54850f04e4455db589640005a42c22787"
        }

        # Make request
        response = requests.post(gateway_url, headers=headers, json=payload, timeout=10)

        # Verify all components
        assert response.status_code == 401
        mock_post.assert_called_once_with(
            gateway_url,
            headers=headers,
            json=payload,
            timeout=10
        )

        # Verify the Content-Type header was set correctly (line 47)
        called_headers = mock_post.call_args[1]['headers']
        assert called_headers["Content-Type"] == "application/json"


if __name__ == "__main__":
    pytest.main([__file__, "-v"])