<template>
  <DashboardLayout>
    <div class="p-6">
      <!-- Header -->
      <div class="mb-6">
        <h1 class="text-2xl font-semibold text-gray-900">Integration Documentation</h1>
        <p class="text-sm text-gray-600 mt-1">Complete guide to integrating the Privacy Gateway into your applications</p>
      </div>

      <!-- Navigation -->
      <div class="grid grid-cols-1 lg:grid-cols-4 gap-6">
        <!-- Sidebar Navigation -->
        <div class="lg:col-span-1">
          <nav class="bg-white rounded border p-4 sticky top-6">
            <h3 class="font-medium text-gray-900 mb-4">Contents</h3>
            <ul class="space-y-2 text-sm">
              <li><a href="#quick-start" class="text-gray-600 hover:text-gray-900 block py-1">Quick Start</a></li>
              <li><a href="#proxy-setup" class="text-gray-600 hover:text-gray-900 block py-1">Proxy Setup</a></li>
              <li><a href="#api-integration" class="text-gray-600 hover:text-gray-900 block py-1">API Integration</a></li>
              <li><a href="#configuration" class="text-gray-600 hover:text-gray-900 block py-1">Configuration</a></li>
              <li><a href="#examples" class="text-gray-600 hover:text-gray-900 block py-1">Code Examples</a></li>
              <li><a href="#events-api" class="text-gray-600 hover:text-gray-900 block py-1">Events API</a></li>
              <li><a href="#troubleshooting" class="text-gray-600 hover:text-gray-900 block py-1">Troubleshooting</a></li>
            </ul>
          </nav>
        </div>

        <!-- Main Content -->
        <div class="lg:col-span-3 space-y-8">
          <!-- Quick Start -->
          <section id="quick-start" class="bg-white rounded border p-6">
            <h2 class="text-xl font-semibold text-gray-900 mb-4">Quick Start</h2>
            <p class="text-gray-600 mb-4">
              Get started with the Privacy Gateway in under 5 minutes. The gateway acts as a transparent proxy between your application and AI providers.
            </p>

            <div class="bg-gray-50 rounded p-4 mb-4">
              <h4 class="font-medium text-gray-900 mb-2">Installation</h4>
              <pre class="text-sm"><code>docker run -p 8080:8080 -p 8081:8081 \\
  -e OPENAI_API_KEY=your_key \\
  sovereignprivacy/gateway:latest</code></pre>
            </div>

            <div class="bg-blue-50 border border-blue-200 rounded p-4">
              <div class="flex">
                <svg class="w-4 h-4 text-blue-600 mt-0.5 mr-3" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd"/>
                </svg>
                <div class="text-sm text-blue-800">
                  <strong>Gateway URLs:</strong><br>
                  • Privacy Gateway: http://localhost:8080<br>
                  • Dashboard: http://localhost:8081
                </div>
              </div>
            </div>
          </section>

          <!-- Proxy Setup -->
          <section id="proxy-setup" class="bg-white rounded border p-6">
            <h2 class="text-xl font-semibold text-gray-900 mb-4">Proxy Setup</h2>
            <p class="text-gray-600 mb-4">
              Configure your application to route AI API calls through the Privacy Gateway.
            </p>

            <div class="space-y-4">
              <div>
                <h4 class="font-medium text-gray-900 mb-2">Python (OpenAI)</h4>
                <pre class="bg-gray-50 rounded p-4 text-sm overflow-x-auto"><code>import openai

# Configure OpenAI client to use Privacy Gateway
client = openai.OpenAI(
    api_key="your_openai_key",
    base_url="http://localhost:8080/v1"  # Privacy Gateway URL
)

# Use normally - PII is automatically detected and protected
response = client.chat.completions.create(
    model="gpt-3.5-turbo",
    messages=[{"role": "user", "content": "Hello, my email is john@example.com"}]
)
</code></pre>
              </div>

              <div>
                <h4 class="font-medium text-gray-900 mb-2">Node.js</h4>
                <pre class="bg-gray-50 rounded p-4 text-sm overflow-x-auto"><code>const OpenAI = require('openai');

const openai = new OpenAI({
  apiKey: 'your_openai_key',
  baseURL: 'http://localhost:8080/v1'  // Privacy Gateway URL
});

const response = await openai.chat.completions.create({
  model: 'gpt-3.5-turbo',
  messages: [{ role: 'user', content: 'My phone number is 555-123-4567' }]
});
</code></pre>
              </div>

              <div>
                <h4 class="font-medium text-gray-900 mb-2">cURL</h4>
                <pre class="bg-gray-50 rounded p-4 text-sm overflow-x-auto"><code>curl -X POST http://localhost:8080/v1/chat/completions \\
  -H "Authorization: Bearer your_openai_key" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "My SSN is 123-45-6789"}]
  }'
</code></pre>
              </div>
            </div>
          </section>

          <!-- API Integration -->
          <section id="api-integration" class="bg-white rounded border p-6">
            <h2 class="text-xl font-semibold text-gray-900 mb-4">API Integration</h2>
            <p class="text-gray-600 mb-4">
              Advanced integration using the Privacy Gateway management API.
            </p>

            <div class="space-y-4">
              <div>
                <h4 class="font-medium text-gray-900 mb-2">Authentication</h4>
                <p class="text-sm text-gray-600 mb-2">Use your API key in the X-API-Key header:</p>
                <pre class="bg-gray-50 rounded p-4 text-sm"><code>curl -H "X-API-Key: your_gateway_api_key" \\
  http://localhost:8081/api/v1/statistics</code></pre>
              </div>

              <div>
                <h4 class="font-medium text-gray-900 mb-2">Available Endpoints</h4>
                <div class="overflow-x-auto">
                  <table class="min-w-full divide-y divide-gray-200">
                    <thead class="bg-gray-50">
                      <tr>
                        <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Method</th>
                        <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Endpoint</th>
                        <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Description</th>
                      </tr>
                    </thead>
                    <tbody class="bg-white divide-y divide-gray-200">
                      <tr>
                        <td class="px-4 py-2 text-sm font-mono text-green-600">GET</td>
                        <td class="px-4 py-2 text-sm font-mono">/api/v1/statistics</td>
                        <td class="px-4 py-2 text-sm text-gray-900">Get protection statistics</td>
                      </tr>
                      <tr>
                        <td class="px-4 py-2 text-sm font-mono text-green-600">GET</td>
                        <td class="px-4 py-2 text-sm font-mono">/api/v1/events</td>
                        <td class="px-4 py-2 text-sm text-gray-900">Get protection events</td>
                      </tr>
                      <tr>
                        <td class="px-4 py-2 text-sm font-mono text-blue-600">POST</td>
                        <td class="px-4 py-2 text-sm font-mono">/api/v1/events/webhook</td>
                        <td class="px-4 py-2 text-sm text-gray-900">Configure event webhooks</td>
                      </tr>
                      <tr>
                        <td class="px-4 py-2 text-sm font-mono text-green-600">GET</td>
                        <td class="px-4 py-2 text-sm font-mono">/api/v1/health</td>
                        <td class="px-4 py-2 text-sm text-gray-900">Health check</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
          </section>

          <!-- Configuration -->
          <section id="configuration" class="bg-white rounded border p-6">
            <h2 class="text-xl font-semibold text-gray-900 mb-4">Configuration</h2>
            <p class="text-gray-600 mb-4">
              Configure the Privacy Gateway behavior and protection rules.
            </p>

            <div class="space-y-4">
              <div>
                <h4 class="font-medium text-gray-900 mb-2">Environment Variables</h4>
                <div class="overflow-x-auto">
                  <table class="min-w-full divide-y divide-gray-200">
                    <thead class="bg-gray-50">
                      <tr>
                        <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Variable</th>
                        <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Default</th>
                        <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Description</th>
                      </tr>
                    </thead>
                    <tbody class="bg-white divide-y divide-gray-200">
                      <tr>
                        <td class="px-4 py-2 text-sm font-mono">PORT</td>
                        <td class="px-4 py-2 text-sm">8080</td>
                        <td class="px-4 py-2 text-sm">Gateway proxy port</td>
                      </tr>
                      <tr>
                        <td class="px-4 py-2 text-sm font-mono">DASHBOARD_PORT</td>
                        <td class="px-4 py-2 text-sm">8081</td>
                        <td class="px-4 py-2 text-sm">Dashboard UI port</td>
                      </tr>
                      <tr>
                        <td class="px-4 py-2 text-sm font-mono">STRICT_MODE</td>
                        <td class="px-4 py-2 text-sm">false</td>
                        <td class="px-4 py-2 text-sm">Block requests with PII</td>
                      </tr>
                      <tr>
                        <td class="px-4 py-2 text-sm font-mono">LOG_LEVEL</td>
                        <td class="px-4 py-2 text-sm">info</td>
                        <td class="px-4 py-2 text-sm">Logging level</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
          </section>

          <!-- Events API -->
          <section id="events-api" class="bg-white rounded border p-6">
            <h2 class="text-xl font-semibold text-gray-900 mb-4">Events API</h2>
            <p class="text-gray-600 mb-4">
              Monitor and receive real-time protection events from the Privacy Gateway.
            </p>

            <div class="space-y-4">
              <div>
                <h4 class="font-medium text-gray-900 mb-2">Get Events</h4>
                <pre class="bg-gray-50 rounded p-4 text-sm overflow-x-auto"><code>GET /api/v1/events?limit=50&amp;offset=0&amp;type=pii_detected

Response:
{
  "events": [
    {
      "id": "evt_12345",
      "timestamp": "2024-01-15T10:30:00Z",
      "type": "pii_detected",
      "pii_type": "email",
      "action": "anonymized",
      "request_id": "req_abc123",
      "metadata": {
        "provider": "openai",
        "model": "gpt-3.5-turbo"
      }
    }
  ],
  "total": 1,
  "has_more": false
}
</code></pre>
              </div>

              <div>
                <h4 class="font-medium text-gray-900 mb-2">Webhook Configuration</h4>
                <pre class="bg-gray-50 rounded p-4 text-sm overflow-x-auto"><code>POST /api/v1/events/webhook

{
  "url": "https://your-app.com/webhooks/privacy-events",
  "events": ["pii_detected", "request_blocked"],
  "secret": "webhook_secret_key"
}

Response:
{
  "id": "wh_12345",
  "url": "https://your-app.com/webhooks/privacy-events",
  "status": "active"
}
</code></pre>
              </div>

              <div>
                <h4 class="font-medium text-gray-900 mb-2">Event Types</h4>
                <ul class="text-sm space-y-2">
                  <li><code class="bg-gray-100 px-2 py-1 rounded text-xs">pii_detected</code> - PII was found and anonymized</li>
                  <li><code class="bg-gray-100 px-2 py-1 rounded text-xs">request_blocked</code> - Request blocked due to strict mode</li>
                  <li><code class="bg-gray-100 px-2 py-1 rounded text-xs">provider_error</code> - AI provider returned an error</li>
                  <li><code class="bg-gray-100 px-2 py-1 rounded text-xs">rate_limit_exceeded</code> - Rate limit reached</li>
                </ul>
              </div>
            </div>
          </section>

          <!-- Examples -->
          <section id="examples" class="bg-white rounded border p-6">
            <h2 class="text-xl font-semibold text-gray-900 mb-4">Complete Examples</h2>

            <div class="space-y-6">
              <div>
                <h4 class="font-medium text-gray-900 mb-2">React Application</h4>
                <pre class="bg-gray-50 rounded p-4 text-sm overflow-x-auto"><code>// openai-client.js
import OpenAI from 'openai';

const openai = new OpenAI({
  apiKey: process.env.REACT_APP_OPENAI_KEY,
  baseURL: process.env.REACT_APP_PRIVACY_GATEWAY_URL || 'http://localhost:8080/v1',
  dangerouslyAllowBrowser: true
});

export const sendMessage = async (message) => {
  try {
    const response = await openai.chat.completions.create({
      model: 'gpt-3.5-turbo',
      messages: [{ role: 'user', content: message }]
    });
    return response.choices[0].message.content;
  } catch (error) {
    console.error('Error sending message:', error);
    throw error;
  }
};
</code></pre>
              </div>

              <div>
                <h4 class="font-medium text-gray-900 mb-2">Python Flask Application</h4>
                <pre class="bg-gray-50 rounded p-4 text-sm overflow-x-auto"><code>from flask import Flask, request, jsonify
import openai
import os

app = Flask(__name__)

# Configure OpenAI client with Privacy Gateway
client = openai.OpenAI(
    api_key=os.getenv('OPENAI_API_KEY'),
    base_url=os.getenv('PRIVACY_GATEWAY_URL', 'http://localhost:8080/v1')
)

@app.route('/chat', methods=['POST'])
def chat():
    user_message = request.json.get('message')

    try:
        response = client.chat.completions.create(
            model='gpt-3.5-turbo',
            messages=[{'role': 'user', 'content': user_message}]
        )
        return jsonify({
            'response': response.choices[0].message.content
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500

if __name__ == '__main__':
    app.run(debug=True)
</code></pre>
              </div>
            </div>
          </section>

          <!-- Troubleshooting -->
          <section id="troubleshooting" class="bg-white rounded border p-6">
            <h2 class="text-xl font-semibold text-gray-900 mb-4">Troubleshooting</h2>

            <div class="space-y-4">
              <div>
                <h4 class="font-medium text-gray-900 mb-2">Common Issues</h4>
                <div class="space-y-3">
                  <div class="border-l-4 border-yellow-400 bg-yellow-50 p-4">
                    <h5 class="font-medium text-yellow-800">Connection Refused</h5>
                    <p class="text-sm text-yellow-700 mt-1">
                      Ensure the Privacy Gateway is running on the correct port. Check with: <code>curl http://localhost:8080/health</code>
                    </p>
                  </div>

                  <div class="border-l-4 border-red-400 bg-red-50 p-4">
                    <h5 class="font-medium text-red-800">API Key Errors</h5>
                    <p class="text-sm text-red-700 mt-1">
                      Verify your AI provider API key is correctly configured. Check the Integrations page in the dashboard.
                    </p>
                  </div>

                  <div class="border-l-4 border-blue-400 bg-blue-50 p-4">
                    <h5 class="font-medium text-blue-800">PII Not Detected</h5>
                    <p class="text-sm text-blue-700 mt-1">
                      The Privacy Gateway continuously improves PII detection. You can customize detection rules in the dashboard settings.
                    </p>
                  </div>
                </div>
              </div>

              <div>
                <h4 class="font-medium text-gray-900 mb-2">Support</h4>
                <p class="text-sm text-gray-600">
                  For additional support, check the logs in the dashboard or contact support at
                  <a href="mailto:support@sovereignprivacy.com" class="text-blue-600 hover:text-blue-800">support@sovereignprivacy.com</a>
                </p>
              </div>
            </div>
          </section>
        </div>
      </div>
    </div>
  </DashboardLayout>
</template>

<script>
import DashboardLayout from '../components/DashboardLayout.vue'

export default {
  name: 'Documentation',
  components: {
    DashboardLayout
  }
}
</script>

<style scoped>
/* Smooth scroll behavior for anchor links */
html {
  scroll-behavior: smooth;
}
</style>
