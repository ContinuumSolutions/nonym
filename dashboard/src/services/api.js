import axios from 'axios'

// Create axios instance
const api = axios.create({
  baseURL: '/api/v1',
  headers: {
    'Content-Type': 'application/json'
  }
})

// Add auth token to requests
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('auth_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// Handle auth errors
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('auth_token')
      localStorage.removeItem('user')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

export const apiService = {
  // Authentication
  async login(email, password) {
    const response = await api.post('/auth/login', { email, password })
    return response.data
  },

  async signup(userData) {
    const response = await api.post('/auth/register', userData)
    return response.data
  },

  async checkAuth() {
    const response = await api.get('/auth/me')
    return response.data
  },

  async logout() {
    const response = await api.post('/auth/logout')
    return response.data
  },

  // Dashboard data
  async getStatistics() {
    const response = await api.get('/statistics')
    return response.data
  },

  async getProtectionEvents(limit = 10) {
    const response = await api.get(`/protection-events?limit=${limit}`)
    return response.data
  },

  async getTransactions(limit = 10) {
    const response = await api.get(`/transactions?limit=${limit}`)
    return response.data
  },

  // Health check
  async getHealth() {
    const response = await api.get('/health')
    return response.data
  },

  // Protected Events
  async getProtectionEvents(params = {}) {
    const query = new URLSearchParams(params).toString()
    const response = await api.get(`/protection-events?${query}`)
    return response.data
  },

  async getProtectionStats() {
    const response = await api.get('/protection-stats')
    return response.data
  },

  // API Keys Management
  async getApiKeys() {
    const response = await api.get('/api-keys')
    return response.data
  },

  async createApiKey(keyData) {
    const response = await api.post('/api-keys', keyData)
    return response.data
  },

  async revokeApiKey(keyId) {
    const response = await api.patch(`/api-keys/${keyId}/revoke`)
    return response.data
  },

  async deleteApiKey(keyId) {
    const response = await api.delete(`/api-keys/${keyId}`)
    return response.data
  },

  async getFullApiKey(keyId) {
    const response = await api.get(`/api-keys/${keyId}/full`)
    return response.data
  },

  // Provider Configuration
  async getProviderConfig() {
    const response = await api.get('/provider-config')
    return response.data
  },

  async saveProviderConfig(config) {
    const response = await api.put('/provider-config', config)
    return response.data
  },

  async testProviderConnection(provider, config) {
    const response = await api.post(`/providers/${provider}/test`, config)
    return response.data
  },

  // Organization Management
  async getOrganization() {
    const response = await api.get('/organization')
    return response.data
  },

  async updateOrganization(orgData) {
    const response = await api.put('/organization', orgData)
    return response.data
  },

  // Team Management
  async getTeamMembers() {
    const response = await api.get('/team/members')
    return response.data
  },

  async inviteTeamMember(memberData) {
    const response = await api.post('/team/members', memberData)
    return response.data
  },

  async removeTeamMember(memberId) {
    const response = await api.delete(`/team/members/${memberId}`)
    return response.data
  },

  // Security Settings
  async updateTwoFactor(settings) {
    const response = await api.put('/security/2fa', settings)
    return response.data
  },

  async terminateSession(sessionId) {
    const response = await api.delete(`/security/sessions/${sessionId}`)
    return response.data
  },

  async updateSecuritySettings(settings) {
    const response = await api.put('/security/settings', settings)
    return response.data
  }
}
