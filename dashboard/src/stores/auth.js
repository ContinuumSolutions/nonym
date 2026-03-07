import { defineStore } from 'pinia'
import { apiService } from '../services/api'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null,
    token: localStorage.getItem('auth_token'),
    isAuthenticated: !!localStorage.getItem('auth_token')
  }),

  actions: {
    async login(email, password) {
      try {
        const response = await apiService.login(email, password)

        this.token = response.token
        this.user = response.user
        this.isAuthenticated = true

        localStorage.setItem('auth_token', response.token)
        localStorage.setItem('user', JSON.stringify(response.user))

        return response
      } catch (error) {
        throw new Error(error.response?.data?.message || 'Login failed')
      }
    },

    async signup(userData) {
      try {
        const response = await apiService.signup(userData)
        return response
      } catch (error) {
        throw new Error(error.response?.data?.message || 'Signup failed')
      }
    },

    async checkAuth() {
      if (!this.token) return false

      try {
        const response = await apiService.checkAuth()
        this.user = response.user
        this.isAuthenticated = true
        return true
      } catch (error) {
        this.logout()
        return false
      }
    },

    logout() {
      this.user = null
      this.token = null
      this.isAuthenticated = false

      localStorage.removeItem('auth_token')
      localStorage.removeItem('user')
    },

    initializeAuth() {
      const token = localStorage.getItem('auth_token')
      const user = localStorage.getItem('user')

      if (token && user) {
        this.token = token
        this.user = JSON.parse(user)
        this.isAuthenticated = true
      }
    }
  }
})
