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
  }
}
