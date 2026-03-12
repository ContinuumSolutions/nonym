import openai
import json
import os

# Configure OpenAI client with Privacy Gateway
url = os.getenv('PRIVACY_GATEWAY_URL', 'http://localhost/v1')
client = openai.OpenAI(
    api_key=os.getenv('OPENAI_API_KEY', 'sk...'),
    base_url=url
)

if __name__ == '__main__':
    api_key = os.getenv('OPENAI_API_KEY')
    print(f"Gateway URL: {url}")
    print(f"API Key set: {'Yes' if api_key else 'No'}")
    if api_key:
        print(f"API Key starts with: {api_key[:10]}...")

    user_message = "My card detail is 4242424242424242"

    try:
        print(f"Sending request to: {client._base_url}")
        response = client.chat.completions.create(
            model='gpt-3.5-turbo',
            messages=[{'role': 'user', 'content': user_message}]
        )
        print("✅ Success!")
        print(f"Response: {response.choices[0].message.content}")
    except Exception as e:
        print(f"❌ Error: {e}")
        print(f"Error type: {type(e)}")

        # Check if it's an API key issue
        if "api_key" in str(e).lower() or "invalid_api_key" in str(e).lower():
            print("\n🔑 This appears to be an API key issue.")
            print("Please set a valid OpenAI API key:")
            print("export OPENAI_API_KEY='sk-your-actual-key-here'")
