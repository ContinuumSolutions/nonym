<template>
  <DashboardLayout>
    <div class="p-8">
      <!-- Header -->
      <div class="mb-8">
        <h1 class="text-3xl font-bold text-gray-900">Integrations</h1>
        <p class="text-gray-600 mt-2">Manage API keys and configure AI engine connections</p>
      </div>

      <!-- Tabs -->
      <div class="mb-6">
        <nav class="flex space-x-8">
          <button
            @click="activeTab = 'api-keys'"
            :class="[
              'py-2 px-1 border-b-2 font-medium text-sm',
              activeTab === 'api-keys'
                ? 'border-blue-500 text-blue-600'
                : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
            ]">
            API Keys
          </button>
          <button
            @click="activeTab = 'ai-engines'"
            :class="[
              'py-2 px-1 border-b-2 font-medium text-sm',
              activeTab === 'ai-engines'
                ? 'border-blue-500 text-blue-600'
                : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
            ]">
            AI Engines
          </button>
        </nav>
      </div>

      <!-- API Keys Tab -->
      <div v-if="activeTab === 'api-keys'" class="space-y-6">
        <!-- Create API Key -->
        <div class="bg-white rounded-lg shadow-sm p-6">
          <h3 class="text-lg font-medium text-gray-900 mb-4">Generate New API Key</h3>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-2">Key Name</label>
              <input
                v-model="newApiKey.name"
                type="text"
                placeholder="e.g., Production Key, Development Key"
                class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500">
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-2">Permissions</label>
              <select v-model="newApiKey.permissions" class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500">
                <option value="read">Read Only</option>
                <option value="write">Read & Write</option>
                <option value="admin">Full Admin</option>
              </select>
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-2">Expiry Date</label>
              <input
                v-model="newApiKey.expiryDate"
                type="date"
                :min="today"
                class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500">
            </div>
            <div class="flex items-end">
              <button
                @click="generateApiKey"
                :disabled="!newApiKey.name"
                class="w-full px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 transition-colors duration-200">
                Generate Key
              </button>
            </div>
          </div>
        </div>

        <!-- API Keys List -->
        <div class="bg-white rounded-lg shadow-sm overflow-hidden">
          <div class="px-6 py-4 border-b border-gray-200">
            <h3 class="text-lg font-medium text-gray-900">Existing API Keys</h3>
          </div>
          <div class="overflow-x-auto">
            <table class="min-w-full divide-y divide-gray-200">
              <thead class="bg-gray-50">
                <tr>
                  <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Name</th>
                  <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Key</th>
                  <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Permissions</th>
                  <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Created</th>
                  <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Expires</th>
                  <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
                  <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Actions</th>
                </tr>
              </thead>
              <tbody class="bg-white divide-y divide-gray-200">
                <tr v-for="key in apiKeys" :key="key.id" class="hover:bg-gray-50">
                  <td class="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
                    {{ key.name }}
                  </td>
                  <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500 font-mono">
                    <div class="flex items-center">
                      <span>{{ key.masked_key }}</span>
                      <button
                        @click="copyToClipboard(key.id)"
                        class="ml-2 text-gray-400 hover:text-gray-600">
                        <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                          <path d="M8 3a1 1 0 011-1h2a1 1 0 110 2H9a1 1 0 01-1-1z"/>
                          <path d="M6 3a2 2 0 00-2 2v11a2 2 0 002 2h8a2 2 0 002-2V5a2 2 0 00-2-2 3 3 0 01-3 3H9a3 3 0 01-3-3z"/>
                        </svg>
                      </button>
                    </div>
                  </td>
                  <td class="px-6 py-4 whitespace-nowrap">
                    <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium"
                          :class="getPermissionColor(key.permissions)">
                      {{ key.permissions }}
                    </span>
                  </td>
                  <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {{ formatDate(key.created_at) }}
                  </td>
                  <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {{ key.expires_at ? formatDate(key.expires_at) : 'Never' }}
                  </td>
                  <td class="px-6 py-4 whitespace-nowrap">
                    <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium"
                          :class="getStatusColor(key.status)">
                      {{ key.status }}
                    </span>
                  </td>
                  <td class="px-6 py-4 whitespace-nowrap text-sm font-medium space-x-2">
                    <button
                      v-if="key.status === 'active'"
                      @click="revokeApiKey(key.id)"
                      class="text-red-600 hover:text-red-900">
                      Revoke
                    </button>
                    <button
                      @click="deleteApiKey(key.id)"
                      class="text-gray-600 hover:text-gray-900">
                      Delete
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <!-- AI Engines Tab -->
      <div v-if="activeTab === 'ai-engines'" class="space-y-6">
        <!-- Security Notice -->
        <div class="bg-blue-50 border border-blue-200 rounded-lg p-4">
          <div class="flex">
            <div class="flex-shrink-0">
              <svg class="h-5 w-5 text-blue-400" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd"/>
              </svg>
            </div>
            <div class="ml-3">
              <h3 class="text-sm font-medium text-blue-800">Secure API Key Storage</h3>
              <p class="mt-1 text-sm text-blue-700">
                All API keys are encrypted using AES-256-GCM encryption before storage and automatically decrypted when routing requests to ensure your credentials remain secure.
              </p>
            </div>
          </div>
        </div>

        <!-- AI Providers -->
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          <!-- Sovereign Privacy Gateway -->
          <div class="bg-white rounded-lg shadow-sm p-6 border-2 border-blue-100">
            <div class="flex items-center mb-4">
              <div class="w-8 h-8 bg-gradient-to-br from-blue-500 to-purple-600 rounded mr-3 flex items-center justify-center">
                <svg class="w-5 h-5 text-white" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M2.166 4.999A11.954 11.954 0 0010 1.944 11.954 11.954 0 0017.834 5c.11.65.166 1.32.166 2.001 0 5.225-3.34 9.67-8 11.317C5.34 16.67 2 12.225 2 7c0-.682.057-1.35.166-2.001zm11.541 3.708a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/>
                </svg>
              </div>
              <h3 class="text-lg font-medium text-gray-900">SPG Instance</h3>
              <div class="ml-auto">
                <span :class="[
                  'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
                  providers.spg.enabled ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'
                ]">
                  {{ providers.spg.enabled ? 'Connected' : 'Disconnected' }}
                </span>
              </div>
            </div>
            <div class="space-y-3">
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">SPG API Key</label>
                <input
                  v-model="providers.spg.apiKey"
                  :type="showKeys.spg ? 'text' : 'password'"
                  placeholder="spg_..."
                  class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm">
                <button
                  @click="showKeys.spg = !showKeys.spg"
                  class="mt-1 text-xs text-gray-500 hover:text-gray-700">
                  {{ showKeys.spg ? 'Hide' : 'Show' }} Key
                </button>
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">SPG Endpoint URL</label>
                <input
                  v-model="providers.spg.endpoint"
                  type="text"
                  placeholder="https://your-spg-instance.com"
                  class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm">
              </div>
              <button
                @click="testConnection('spg')"
                class="w-full px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors duration-200">
                Test Connection
              </button>
            </div>
          </div>

          <!-- OpenAI -->
          <div class="bg-white rounded-lg shadow-sm p-6">
            <div class="flex items-center mb-4">
              <img src="https://cdn.worldvectorlogo.com/logos/openai-2.svg" alt="OpenAI" class="w-8 h-8 mr-3">
              <h3 class="text-lg font-medium text-gray-900">OpenAI</h3>
              <div class="ml-auto">
                <span :class="[
                  'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
                  providers.openai.enabled ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'
                ]">
                  {{ providers.openai.enabled ? 'Connected' : 'Disconnected' }}
                </span>
              </div>
            </div>
            <div class="space-y-3">
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">API Key</label>
                <input
                  v-model="providers.openai.apiKey"
                  :type="showKeys.openai ? 'text' : 'password'"
                  placeholder="sk-..."
                  class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm">
                <button
                  @click="showKeys.openai = !showKeys.openai"
                  class="mt-1 text-xs text-gray-500 hover:text-gray-700">
                  {{ showKeys.openai ? 'Hide' : 'Show' }} Key
                </button>
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">Models</label>
                <div class="space-y-1">
                  <label v-for="model in providers.openai.models" :key="model.id" class="flex items-center">
                    <input v-model="model.enabled" type="checkbox" class="rounded border-gray-300 text-blue-600 focus:ring-blue-500">
                    <span class="ml-2 text-sm text-gray-700">{{ model.name }}</span>
                  </label>
                </div>
              </div>
              <button
                @click="testConnection('openai')"
                class="w-full px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors duration-200">
                Test Connection
              </button>
            </div>
          </div>

          <!-- Anthropic -->
          <div class="bg-white rounded-lg shadow-sm p-6">
            <div class="flex items-center mb-4">
              <div class="w-8 h-8 bg-orange-500 rounded mr-3 flex items-center justify-center">
                <span class="text-white font-bold text-sm">A</span>
              </div>
              <h3 class="text-lg font-medium text-gray-900">Anthropic</h3>
              <div class="ml-auto">
                <span :class="[
                  'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
                  providers.anthropic.enabled ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'
                ]">
                  {{ providers.anthropic.enabled ? 'Connected' : 'Disconnected' }}
                </span>
              </div>
            </div>
            <div class="space-y-3">
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">API Key</label>
                <input
                  v-model="providers.anthropic.apiKey"
                  :type="showKeys.anthropic ? 'text' : 'password'"
                  placeholder="sk-ant-..."
                  class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm">
                <button
                  @click="showKeys.anthropic = !showKeys.anthropic"
                  class="mt-1 text-xs text-gray-500 hover:text-gray-700">
                  {{ showKeys.anthropic ? 'Hide' : 'Show' }} Key
                </button>
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">Models</label>
                <div class="space-y-1">
                  <label v-for="model in providers.anthropic.models" :key="model.id" class="flex items-center">
                    <input v-model="model.enabled" type="checkbox" class="rounded border-gray-300 text-blue-600 focus:ring-blue-500">
                    <span class="ml-2 text-sm text-gray-700">{{ model.name }}</span>
                  </label>
                </div>
              </div>
              <button
                @click="testConnection('anthropic')"
                class="w-full px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors duration-200">
                Test Connection
              </button>
            </div>
          </div>

          <!-- Google -->
          <div class="bg-white rounded-lg shadow-sm p-6">
            <div class="flex items-center mb-4">
              <img src="https://developers.google.com/static/site-assets/logo-google-developers.svg" alt="Google" class="w-8 h-8 mr-3">
              <h3 class="text-lg font-medium text-gray-900">Google</h3>
              <div class="ml-auto">
                <span :class="[
                  'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
                  providers.google.enabled ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'
                ]">
                  {{ providers.google.enabled ? 'Connected' : 'Disconnected' }}
                </span>
              </div>
            </div>
            <div class="space-y-3">
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">API Key</label>
                <input
                  v-model="providers.google.apiKey"
                  :type="showKeys.google ? 'text' : 'password'"
                  placeholder="AIza..."
                  class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm">
                <button
                  @click="showKeys.google = !showKeys.google"
                  class="mt-1 text-xs text-gray-500 hover:text-gray-700">
                  {{ showKeys.google ? 'Hide' : 'Show' }} Key
                </button>
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">Models</label>
                <div class="space-y-1">
                  <label v-for="model in providers.google.models" :key="model.id" class="flex items-center">
                    <input v-model="model.enabled" type="checkbox" class="rounded border-gray-300 text-blue-600 focus:ring-blue-500">
                    <span class="ml-2 text-sm text-gray-700">{{ model.name }}</span>
                  </label>
                </div>
              </div>
              <button
                @click="testConnection('google')"
                class="w-full px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors duration-200">
                Test Connection
              </button>
            </div>
          </div>

          <!-- Local LLM -->
          <div class="bg-white rounded-lg shadow-sm p-6">
            <div class="flex items-center mb-4">
              <div class="w-8 h-8 bg-gray-600 rounded mr-3 flex items-center justify-center">
                <svg class="w-5 h-5 text-white" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M3 5a2 2 0 012-2h10a2 2 0 012 2v8a2 2 0 01-2 2h-2.22l.123.489.804.804A1 1 0 0113 18H7a1 1 0 01-.707-1.707l.804-.804L7.22 15H5a2 2 0 01-2-2V5zm5.771 7H5V5h10v7H8.771z" clip-rule="evenodd"/>
                </svg>
              </div>
              <h3 class="text-lg font-medium text-gray-900">Local LLM</h3>
              <div class="ml-auto">
                <span :class="[
                  'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
                  providers.local.enabled ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'
                ]">
                  {{ providers.local.enabled ? 'Connected' : 'Disconnected' }}
                </span>
              </div>
            </div>
            <div class="space-y-3">
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">Endpoint URL</label>
                <input
                  v-model="providers.local.endpoint"
                  type="text"
                  placeholder="http://localhost:11434"
                  class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm">
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">Available Models</label>
                <div class="space-y-1">
                  <label v-for="model in providers.local.models" :key="model.id" class="flex items-center">
                    <input v-model="model.enabled" type="checkbox" class="rounded border-gray-300 text-blue-600 focus:ring-blue-500">
                    <span class="ml-2 text-sm text-gray-700">{{ model.name }}</span>
                  </label>
                </div>
              </div>
              <button
                @click="testConnection('local')"
                class="w-full px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors duration-200">
                Test Connection
              </button>
            </div>
          </div>
        </div>

        <!-- Save Configuration -->
        <div class="bg-white rounded-lg shadow-sm p-6">
          <div class="flex items-center justify-between">
            <div>
              <h3 class="text-lg font-medium text-gray-900">Configuration</h3>
              <p class="text-sm text-gray-600 mt-1">Save your AI provider configuration</p>
            </div>
            <button
              @click="saveProviderConfig"
              class="px-6 py-2 bg-green-600 text-white rounded-md hover:bg-green-700 transition-colors duration-200">
              Save Configuration
            </button>
          </div>
        </div>
      </div>

      <!-- Success/Error Messages -->
      <div v-if="message" class="fixed top-4 right-4 z-50">
        <div :class="[
          'px-4 py-3 rounded-md shadow-lg',
          message.type === 'success' ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'
        ]">
          {{ message.text }}
          <button @click="message = null" class="ml-2 text-sm underline">Close</button>
        </div>
      </div>
    </div>
  </DashboardLayout>
</template>

<script>
import { ref, onMounted, computed } from 'vue'
import DashboardLayout from '../components/DashboardLayout.vue'
import { apiService } from '../services/api'

export default {
  name: 'Integrations',
  components: {
    DashboardLayout
  },
  setup() {
    const activeTab = ref('api-keys')
    const message = ref(null)

    // API Keys
    const apiKeys = ref([])
    const newApiKey = ref({
      name: '',
      permissions: 'read',
      expiryDate: ''
    })

    // AI Providers
    const providers = ref({
      spg: {
        enabled: false,
        apiKey: '',
        endpoint: '',
        models: []
      },
      openai: {
        enabled: false,
        apiKey: '',
        models: [
          { id: 'gpt-4', name: 'GPT-4', enabled: false },
          { id: 'gpt-3.5-turbo', name: 'GPT-3.5 Turbo', enabled: true },
          { id: 'gpt-4-turbo', name: 'GPT-4 Turbo', enabled: false }
        ]
      },
      anthropic: {
        enabled: false,
        apiKey: '',
        models: [
          { id: 'claude-3-haiku', name: 'Claude 3 Haiku', enabled: true },
          { id: 'claude-3-sonnet', name: 'Claude 3 Sonnet', enabled: false },
          { id: 'claude-3-opus', name: 'Claude 3 Opus', enabled: false }
        ]
      },
      google: {
        enabled: false,
        apiKey: '',
        models: [
          { id: 'gemini-pro', name: 'Gemini Pro', enabled: true },
          { id: 'gemini-pro-vision', name: 'Gemini Pro Vision', enabled: false }
        ]
      },
      local: {
        enabled: false,
        endpoint: 'http://localhost:11434',
        models: [
          { id: 'llama2', name: 'Llama 2', enabled: true },
          { id: 'mistral', name: 'Mistral', enabled: false },
          { id: 'codellama', name: 'Code Llama', enabled: false }
        ]
      }
    })

    const showKeys = ref({
      spg: false,
      openai: false,
      anthropic: false,
      google: false
    })

    const today = computed(() => {
      return new Date().toISOString().split('T')[0]
    })

    // Load existing data
    const loadApiKeys = async () => {
      try {
        const response = await apiService.getApiKeys()
        apiKeys.value = response.api_keys || []
      } catch (error) {
        console.error('Failed to load API keys:', error)
        apiKeys.value = []
      }
    }

    const loadProviderConfig = async () => {
      try {
        const response = await apiService.getProviderConfig()
        if (response.providers) {
          providers.value = { ...providers.value, ...response.providers }
        }
      } catch (error) {
        console.error('Failed to load provider config:', error)
      }
    }


    // API Key operations
    const generateApiKey = async () => {
      try {
        const response = await apiService.createApiKey(newApiKey.value)
        showMessage('API Key generated successfully!', 'success')

        // Add to list
        apiKeys.value.unshift({
          id: response.id || `key_${Date.now()}`,
          name: newApiKey.value.name,
          masked_key: response.masked_key || 'spg_••••••••••••••••••••••••••••' + Math.random().toString(36).slice(-4),
          permissions: newApiKey.value.permissions,
          created_at: new Date(),
          expires_at: newApiKey.value.expiryDate ? new Date(newApiKey.value.expiryDate) : null,
          status: 'active'
        })

        // Reset form
        newApiKey.value = {
          name: '',
          permissions: 'read',
          expiryDate: ''
        }
      } catch (error) {
        showMessage('Failed to generate API key', 'error')
      }
    }

    const revokeApiKey = async (keyId) => {
      try {
        await apiService.revokeApiKey(keyId)
        const key = apiKeys.value.find(k => k.id === keyId)
        if (key) key.status = 'revoked'
        showMessage('API Key revoked successfully', 'success')
      } catch (error) {
        showMessage('Failed to revoke API key', 'error')
      }
    }

    const deleteApiKey = async (keyId) => {
      try {
        await apiService.deleteApiKey(keyId)
        apiKeys.value = apiKeys.value.filter(k => k.id !== keyId)
        showMessage('API Key deleted successfully', 'success')
      } catch (error) {
        showMessage('Failed to delete API key', 'error')
      }
    }

    const copyToClipboard = async (keyId) => {
      try {
        // Get the full API key from server (only returned during copy operation for security)
        const response = await apiService.getFullApiKey(keyId)

        if (response.api_key) {
          // Use the modern Clipboard API if available
          if (navigator.clipboard && window.isSecureContext) {
            await navigator.clipboard.writeText(response.api_key)
            showMessage('API key copied to clipboard successfully!', 'success')
          } else {
            // Fallback for older browsers or non-HTTPS contexts
            const textArea = document.createElement('textarea')
            textArea.value = response.api_key
            textArea.style.position = 'fixed'
            textArea.style.left = '-999999px'
            textArea.style.top = '-999999px'
            document.body.appendChild(textArea)
            textArea.focus()
            textArea.select()

            try {
              document.execCommand('copy')
              showMessage('API key copied to clipboard successfully!', 'success')
            } catch (err) {
              showMessage('Failed to copy API key. Please copy it manually.', 'error')
              console.error('Fallback clipboard copy failed:', err)
            } finally {
              document.body.removeChild(textArea)
            }
          }
        } else {
          showMessage('Failed to retrieve API key for copying', 'error')
        }
      } catch (error) {
        console.error('Failed to copy API key:', error)
        showMessage('Failed to copy API key. Please try again.', 'error')
      }
    }

    // Provider operations
    const testConnection = async (providerName) => {
      try {
        const response = await apiService.testProviderConnection(providerName, providers.value[providerName])
        if (response.success) {
          providers.value[providerName].enabled = true
          showMessage(`${providerName} connection successful!`, 'success')
        } else {
          providers.value[providerName].enabled = false
          showMessage(`${providerName} connection failed: ${response.error}`, 'error')
        }
      } catch (error) {
        providers.value[providerName].enabled = false
        showMessage(`${providerName} connection failed`, 'error')
      }
    }

    const saveProviderConfig = async () => {
      try {
        await apiService.saveProviderConfig({ providers: providers.value })
        showMessage('Provider configuration saved successfully!', 'success')
      } catch (error) {
        showMessage('Failed to save provider configuration', 'error')
      }
    }

    // Utility functions
    const formatDate = (date) => {
      return new Date(date).toLocaleDateString()
    }

    const getPermissionColor = (permission) => {
      const colors = {
        'read': 'bg-blue-100 text-blue-800',
        'write': 'bg-yellow-100 text-yellow-800',
        'admin': 'bg-red-100 text-red-800'
      }
      return colors[permission] || 'bg-gray-100 text-gray-800'
    }

    const getStatusColor = (status) => {
      const colors = {
        'active': 'bg-green-100 text-green-800',
        'revoked': 'bg-red-100 text-red-800',
        'expired': 'bg-gray-100 text-gray-800'
      }
      return colors[status] || 'bg-gray-100 text-gray-800'
    }

    const showMessage = (text, type = 'info') => {
      message.value = { text, type }
      setTimeout(() => {
        message.value = null
      }, 5000)
    }

    onMounted(() => {
      loadApiKeys()
      loadProviderConfig()
    })

    return {
      activeTab,
      message,
      apiKeys,
      newApiKey,
      providers,
      showKeys,
      today,
      generateApiKey,
      revokeApiKey,
      deleteApiKey,
      copyToClipboard,
      testConnection,
      saveProviderConfig,
      formatDate,
      getPermissionColor,
      getStatusColor
    }
  }
}
</script>
