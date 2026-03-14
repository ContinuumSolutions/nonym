<template>
  <div class="auth-page">
    <!-- Background with animated gradients -->
    <div class="auth-background">
      <div class="gradient-orb orb-1"></div>
      <div class="gradient-orb orb-2"></div>
      <div class="gradient-orb orb-3"></div>
    </div>

    <!-- Main content -->
    <div class="auth-container">
      <div class="auth-card">
        <!-- Logo and header -->
        <div class="auth-header">
          <div class="logo-container">
            <div class="logo-icon">
              <ShieldCheck class="h-6 w-6 text-white" />
            </div>
            <div class="logo-glow"></div>
          </div>
          <h1 class="auth-title">
            {{ isSignup ? 'Join Privacy Gateway' : 'Welcome Back' }}
          </h1>
          <p class="auth-subtitle">
            {{ isSignup
              ? 'Create your account and start protecting your data'
              : 'Sign in to your Privacy Gateway dashboard'
            }}
          </p>
        </div>

        <!-- Form -->
        <form @submit.prevent="handleSubmit" class="auth-form" novalidate>
          <!-- Name fields for signup -->
          <div v-if="isSignup" class="form-row">
            <div class="form-group">
              <label class="form-label">First Name</label>
              <div class="input-wrapper">
                <input
                  v-model="form.firstName"
                  type="text"
                  class="form-input"
                  :class="{ 'error': errors.firstName }"
                  placeholder="John"
                  @blur="validateField('firstName')"
                  @input="clearError('firstName')"
                />
                <div v-if="errors.firstName" class="input-error">
                  {{ errors.firstName }}
                </div>
              </div>
            </div>
            <div class="form-group">
              <label class="form-label">Last Name</label>
              <div class="input-wrapper">
                <input
                  v-model="form.lastName"
                  type="text"
                  class="form-input"
                  :class="{ 'error': errors.lastName }"
                  placeholder="Doe"
                  @blur="validateField('lastName')"
                  @input="clearError('lastName')"
                />
                <div v-if="errors.lastName" class="input-error">
                  {{ errors.lastName }}
                </div>
              </div>
            </div>
          </div>

          <!-- Email -->
          <div class="form-group">
            <label class="form-label">Email Address</label>
            <div class="input-wrapper">
              <div class="input-with-icon">
                <Mail class="input-icon" />
                <input
                  v-model="form.email"
                  type="email"
                  class="form-input with-icon"
                  :class="{ 'error': errors.email }"
                  placeholder="Enter your email"
                  autocomplete="email"
                  @blur="validateField('email')"
                  @input="clearError('email')"
                />
              </div>
              <div v-if="errors.email" class="input-error">
                {{ errors.email }}
              </div>
            </div>
          </div>

          <!-- Organization for signup -->
          <div v-if="isSignup" class="form-group">
            <label class="form-label">Organization</label>
            <div class="input-wrapper">
              <div class="input-with-icon">
                <Building class="input-icon" />
                <input
                  v-model="form.organization"
                  type="text"
                  class="form-input with-icon"
                  :class="{ 'error': errors.organization }"
                  placeholder="Your Company Name"
                  @blur="validateField('organization')"
                  @input="clearError('organization')"
                />
              </div>
              <div v-if="errors.organization" class="input-error">
                {{ errors.organization }}
              </div>
            </div>
          </div>

          <!-- Password -->
          <div class="form-group">
            <label class="form-label">Password</label>
            <div class="input-wrapper">
              <div class="input-with-icon">
                <Lock class="input-icon" />
                <input
                  v-model="form.password"
                  :type="showPassword ? 'text' : 'password'"
                  class="form-input with-icon with-action"
                  :class="{ 'error': errors.password }"
                  placeholder="Enter your password"
                  autocomplete="current-password"
                  @blur="validateField('password')"
                  @input="clearError('password')"
                />
                <button
                  type="button"
                  @click="showPassword = !showPassword"
                  class="input-action"
                >
                  <Eye v-if="!showPassword" class="h-4 w-4" />
                  <EyeOff v-else class="h-4 w-4" />
                </button>
              </div>
              <div v-if="errors.password" class="input-error">
                {{ errors.password }}
              </div>
              <div v-if="isSignup" class="password-strength">
                <div class="strength-bar">
                  <div
                    class="strength-fill"
                    :class="`strength-${passwordStrength}`"
                    :style="{ width: `${(passwordStrength / 4) * 100}%` }"
                  ></div>
                </div>
                <span class="strength-text" :class="`text-strength-${passwordStrength}`">
                  {{ strengthText }}
                </span>
              </div>
            </div>
          </div>

          <!-- Additional options for login -->
          <div v-if="!isSignup" class="form-options">
            <label class="checkbox-label">
              <input
                v-model="form.rememberMe"
                type="checkbox"
                class="checkbox"
              />
              <span class="checkbox-text">Remember me</span>
            </label>
            <button type="button" class="forgot-link">
              Forgot password?
            </button>
          </div>

          <!-- Submit button -->
          <button
            type="submit"
            class="submit-button"
            :class="{ 'loading': loading }"
            :disabled="loading || !isFormValid"
          >
            <div class="button-content">
              <Loader2 v-if="loading" class="h-5 w-5 animate-spin" />
              <span v-else-if="isSignup">Create Account</span>
              <span v-else>Sign In</span>
            </div>
          </button>

          <!-- Terms for signup -->
          <div v-if="isSignup" class="terms-text">
            By creating an account, you agree to our
            <a href="#" class="terms-link">Terms of Service</a>
            and
            <a href="#" class="terms-link">Privacy Policy</a>
          </div>
        </form>

        <!-- Toggle mode -->
        <div class="auth-toggle">
          <span class="toggle-text">
            {{ isSignup ? 'Already have an account?' : 'Need an account?' }}
          </span>
          <button @click="toggleMode" class="toggle-button">
            {{ isSignup ? 'Sign In' : 'Create Account' }}
          </button>
        </div>

        <!-- Social divider for future features -->
        <div v-if="false" class="social-divider">
          <span>Or continue with</span>
        </div>
      </div>
    </div>

    <!-- Toast notifications -->
    <ToastContainer />
  </div>
</template>

<script>
import { ref, computed, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import {
  ShieldCheck,
  Mail,
  Lock,
  Building,
  Eye,
  EyeOff,
  Loader2
} from 'lucide-vue-next'
import { useAuthStore } from '../stores/auth'
import { useToast } from '../services/toast'
import ToastContainer from '../components/ToastContainer.vue'

export default {
  name: 'LoginNew',
  components: {
    ShieldCheck,
    Mail,
    Lock,
    Building,
    Eye,
    EyeOff,
    Loader2,
    ToastContainer
  },
  props: {
    mode: {
      type: String,
      default: 'login'
    }
  },
  setup(props) {
    const router = useRouter()
    const route = useRoute()
    const authStore = useAuthStore()
    const { success, error: showError } = useToast()

    const currentMode = ref(props.mode || 'login')
    const loading = ref(false)
    const showPassword = ref(false)
    const errors = ref({})

    const form = ref({
      email: '',
      password: '',
      firstName: '',
      lastName: '',
      organization: '',
      rememberMe: false
    })

    const isSignup = computed(() => currentMode.value === 'signup')

    // Password strength calculation
    const passwordStrength = computed(() => {
      const password = form.value.password
      if (!password) return 0

      let strength = 0
      if (password.length >= 8) strength++
      if (/[a-z]/.test(password)) strength++
      if (/[A-Z]/.test(password)) strength++
      if (/[0-9]/.test(password)) strength++
      if (/[^A-Za-z0-9]/.test(password)) strength++

      return Math.min(strength, 4)
    })

    const strengthText = computed(() => {
      const texts = ['', 'Very Weak', 'Weak', 'Good', 'Strong']
      return texts[passwordStrength.value] || ''
    })

    // Form validation
    const validateField = (field) => {
      const value = form.value[field]?.trim()

      switch (field) {
        case 'email':
          if (!value) {
            errors.value.email = 'Email is required'
          } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)) {
            errors.value.email = 'Please enter a valid email'
          }
          break

        case 'password':
          if (!value) {
            errors.value.password = 'Password is required'
          } else if (isSignup.value && value.length < 8) {
            errors.value.password = 'Password must be at least 8 characters'
          }
          break

        case 'firstName':
          if (isSignup.value && !value) {
            errors.value.firstName = 'First name is required'
          }
          break

        case 'lastName':
          if (isSignup.value && !value) {
            errors.value.lastName = 'Last name is required'
          }
          break

        case 'organization':
          if (isSignup.value && !value) {
            errors.value.organization = 'Organization name is required'
          }
          break
      }
    }

    const clearError = (field) => {
      if (errors.value[field]) {
        delete errors.value[field]
        errors.value = { ...errors.value }
      }
    }

    const validateForm = () => {
      errors.value = {}

      const fieldsToValidate = ['email', 'password']
      if (isSignup.value) {
        fieldsToValidate.push('firstName', 'lastName', 'organization')
      }

      fieldsToValidate.forEach(validateField)
      return Object.keys(errors.value).length === 0
    }

    const isFormValid = computed(() => {
      const required = ['email', 'password']
      if (isSignup.value) {
        required.push('firstName', 'lastName', 'organization')
      }

      return required.every(field => form.value[field]?.trim()) &&
             Object.keys(errors.value).length === 0
    })

    const toggleMode = () => {
      currentMode.value = currentMode.value === 'login' ? 'signup' : 'login'
      errors.value = {}

      // Reset form
      form.value = {
        email: form.value.email, // Keep email
        password: '',
        firstName: '',
        lastName: '',
        organization: '',
        rememberMe: false
      }

      router.push(currentMode.value === 'signup' ? '/signup' : '/login')
    }

    const handleSubmit = async () => {
      if (!validateForm()) {
        showError(
          'Please fix the errors below',
          'Check all fields and try again'
        )
        return
      }

      loading.value = true

      try {
        if (isSignup.value) {
          // Prepare signup data
          const userData = {
            email: form.value.email.trim(),
            password: form.value.password,
            name: `${form.value.firstName.trim()} ${form.value.lastName.trim()}`.trim(),
            organization: form.value.organization.trim()
          }

          await authStore.signup(userData)

          success(
            'Account Created Successfully!',
            'Please sign in with your new credentials'
          )

          // Switch to login mode and pre-fill email
          currentMode.value = 'login'
          form.value.password = ''
          form.value.firstName = ''
          form.value.lastName = ''
          form.value.organization = ''

          router.push('/login')
        } else {
          // Login
          await authStore.login(form.value.email.trim(), form.value.password)

          success(
            'Welcome Back!',
            'Successfully signed in to your account'
          )

          router.push('/dashboard')
        }
      } catch (err) {
        let title = isSignup.value ? 'Signup Failed' : 'Login Failed'
        let message = 'Please check your details and try again'

        // Handle specific error messages
        if (err.message) {
          if (err.message.includes('already registered') || err.message.includes('duplicate')) {
            title = 'Email Already Registered'
            message = 'This email is already associated with an account. Try signing in instead.'
          } else if (err.message.includes('invalid email or password')) {
            title = 'Invalid Credentials'
            message = 'Please check your email and password and try again'
          } else if (err.message.includes('validation')) {
            title = 'Validation Error'
            message = err.message
          } else {
            message = err.message
          }
        }

        showError(title, message)
      } finally {
        loading.value = false
      }
    }

    // Set mode based on route
    watch(() => route.path, (newPath) => {
      if (newPath === '/signup') {
        currentMode.value = 'signup'
      } else if (newPath === '/login') {
        currentMode.value = 'login'
      }
    }, { immediate: true })

    return {
      form,
      loading,
      showPassword,
      errors,
      isSignup,
      passwordStrength,
      strengthText,
      isFormValid,
      validateField,
      clearError,
      toggleMode,
      handleSubmit
    }
  }
}
</script>

<style scoped>
/* Base styles */
.auth-page {
  @apply min-h-screen flex items-center justify-center p-4 relative overflow-hidden;
}

/* Animated background */
.auth-background {
  @apply absolute inset-0 -z-10;
  background: linear-gradient(135deg, #f8fafc 0%, #f1f5f9 100%);
}

.gradient-orb {
  @apply absolute rounded-full opacity-60 animate-pulse;
  filter: blur(40px);
}

.orb-1 {
  @apply w-96 h-96 bg-gradient-to-r from-purple-400 to-pink-400;
  top: -10%;
  right: -10%;
  animation: float 20s ease-in-out infinite;
}

.orb-2 {
  @apply w-80 h-80 bg-gradient-to-r from-blue-400 to-purple-400;
  bottom: -10%;
  left: -10%;
  animation: float 25s ease-in-out infinite reverse;
}

.orb-3 {
  @apply w-64 h-64 bg-gradient-to-r from-indigo-400 to-blue-400;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  animation: float 30s ease-in-out infinite;
}

@keyframes float {
  0%, 100% { transform: translate(0, 0) rotate(0deg); }
  33% { transform: translate(30px, -30px) rotate(120deg); }
  66% { transform: translate(-20px, 20px) rotate(240deg); }
}

/* Container */
.auth-container {
  @apply w-full max-w-md relative z-10;
}

.auth-card {
  @apply bg-white/80 backdrop-blur-xl rounded-3xl shadow-2xl border border-white/20 p-8;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.25);
}

/* Header */
.auth-header {
  @apply text-center mb-8;
}

.logo-container {
  @apply relative inline-block mb-6;
}

.logo-icon {
  @apply w-16 h-16 rounded-2xl flex items-center justify-center mx-auto relative z-10;
  background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%);
}

.logo-glow {
  @apply absolute inset-0 rounded-2xl opacity-50;
  background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%);
  filter: blur(8px);
  animation: glow 2s ease-in-out infinite alternate;
}

@keyframes glow {
  from { transform: scale(1); opacity: 0.5; }
  to { transform: scale(1.05); opacity: 0.8; }
}

.auth-title {
  @apply text-3xl font-bold text-gray-900 mb-3;
  background: linear-gradient(135deg, #1f2937 0%, #4b5563 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.auth-subtitle {
  @apply text-gray-600 text-sm leading-relaxed;
}

/* Form */
.auth-form {
  @apply space-y-6;
}

.form-row {
  @apply grid grid-cols-2 gap-4;
}

.form-group {
  @apply space-y-2;
}

.form-label {
  @apply block text-sm font-semibold text-gray-700;
}

.input-wrapper {
  @apply space-y-2;
}

.input-with-icon {
  @apply relative;
}

.input-icon {
  @apply absolute left-4 top-1/2 transform -translate-y-1/2 h-5 w-5 text-gray-400;
}

.form-input {
  @apply w-full px-4 py-4 border border-gray-200 rounded-xl text-gray-900 placeholder-gray-500;
  @apply focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent;
  @apply transition-all duration-200 bg-white/50 backdrop-blur-sm;
}

.form-input.with-icon {
  @apply pl-12;
}

.form-input.with-action {
  @apply pr-12;
}

.form-input.error {
  @apply border-red-300 ring-1 ring-red-300;
}

.form-input:focus {
  background: white;
  box-shadow: 0 10px 25px -5px rgba(99, 102, 241, 0.1);
}

.input-action {
  @apply absolute right-4 top-1/2 transform -translate-y-1/2 text-gray-400 hover:text-gray-600;
  @apply focus:outline-none transition-colors;
}

.input-error {
  @apply text-red-600 text-xs font-medium;
}

/* Password strength */
.password-strength {
  @apply flex items-center space-x-3;
}

.strength-bar {
  @apply flex-1 h-2 bg-gray-200 rounded-full overflow-hidden;
}

.strength-fill {
  @apply h-full transition-all duration-300 rounded-full;
}

.strength-1 { @apply bg-red-400; }
.strength-2 { @apply bg-yellow-400; }
.strength-3 { @apply bg-blue-400; }
.strength-4 { @apply bg-green-400; }

.strength-text {
  @apply text-xs font-medium;
}

.text-strength-1 { @apply text-red-500; }
.text-strength-2 { @apply text-yellow-500; }
.text-strength-3 { @apply text-blue-500; }
.text-strength-4 { @apply text-green-500; }

/* Form options */
.form-options {
  @apply flex items-center justify-between;
}

.checkbox-label {
  @apply flex items-center space-x-2 cursor-pointer;
}

.checkbox {
  @apply w-4 h-4 text-indigo-600 bg-gray-100 border-gray-300 rounded focus:ring-indigo-500;
}

.checkbox-text {
  @apply text-sm text-gray-600;
}

.forgot-link {
  @apply text-sm text-indigo-600 hover:text-indigo-800 font-medium transition-colors;
}

/* Submit button */
.submit-button {
  @apply w-full py-4 px-6 rounded-xl font-semibold text-white transition-all duration-200;
  @apply focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500;
  background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%);
}

.submit-button:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 20px 25px -5px rgba(99, 102, 241, 0.4);
}

.submit-button:disabled {
  @apply opacity-50 cursor-not-allowed;
  transform: none;
  box-shadow: none;
}

.submit-button.loading {
  @apply cursor-wait;
}

.button-content {
  @apply flex items-center justify-center space-x-2;
}

/* Terms text */
.terms-text {
  @apply text-xs text-gray-500 text-center leading-relaxed;
}

.terms-link {
  @apply text-indigo-600 hover:text-indigo-800 transition-colors;
}

/* Auth toggle */
.auth-toggle {
  @apply mt-8 pt-6 border-t border-gray-200 text-center;
}

.toggle-text {
  @apply text-sm text-gray-600;
}

.toggle-button {
  @apply ml-2 text-sm font-semibold text-indigo-600 hover:text-indigo-800 transition-colors;
}

/* Social divider */
.social-divider {
  @apply mt-6 pt-6 border-t border-gray-200 text-center;
}

.social-divider span {
  @apply text-sm text-gray-500;
}

/* Mobile responsiveness */
@media (max-width: 640px) {
  .auth-card {
    @apply p-6 mx-2;
  }

  .auth-title {
    @apply text-2xl;
  }

  .form-row {
    @apply grid-cols-1 gap-6;
  }

  .orb-1, .orb-2, .orb-3 {
    @apply w-64 h-64;
  }
}

@media (max-width: 480px) {
  .auth-page {
    @apply p-2;
  }

  .auth-card {
    @apply p-4 rounded-2xl;
  }

  .form-input {
    @apply py-3;
  }

  .submit-button {
    @apply py-3;
  }
}
</style>