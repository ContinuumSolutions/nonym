<template>
  <div class="min-h-screen bg-gradient-to-br from-primary/10 via-neutral-50 to-success/5 flex items-center justify-center p-4">
    <div class="w-full max-w-md">
      <!-- Logo -->
      <div class="text-center mb-8">
        <div class="gradient-bg w-16 h-16 rounded-2xl flex items-center justify-center mx-auto mb-4 shadow-xl">
          <ShieldCheck class="h-8 w-8 text-white" />
        </div>
        <h1 class="text-3xl font-bold text-neutral-800 mb-2">
          {{ isSignup ? 'Join Privacy Gateway' : 'Privacy Gateway' }}
        </h1>
        <p class="text-neutral-600">
          {{ isSignup ? 'Protect your organization\'s data' : 'Protect your organization\'s sensitive data' }}
        </p>
      </div>

      <!-- Form -->
      <div class="card p-8">
        <form @submit.prevent="handleSubmit" class="space-y-6">
          <!-- Signup fields -->
          <div v-if="isSignup" class="grid grid-cols-2 gap-4">
            <div>
              <label class="block text-sm font-medium text-neutral-700 mb-2">First Name</label>
              <input
                v-model="form.firstName"
                type="text"
                required
                class="input-field"
                placeholder="John"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-neutral-700 mb-2">Last Name</label>
              <input
                v-model="form.lastName"
                type="text"
                required
                class="input-field"
                placeholder="Doe"
              />
            </div>
          </div>

          <div>
            <label class="block text-sm font-medium text-neutral-700 mb-2">Email</label>
            <div class="relative">
              <Mail class="absolute left-3 top-3 h-5 w-5 text-neutral-400" />
              <input
                v-model="form.email"
                type="email"
                required
                class="input-field pl-10"
                placeholder="Enter your email"
              />
            </div>
          </div>

          <div v-if="isSignup">
            <label class="block text-sm font-medium text-neutral-700 mb-2">Organization</label>
            <div class="relative">
              <Building class="absolute left-3 top-3 h-5 w-5 text-neutral-400" />
              <input
                v-model="form.organization"
                type="text"
                required
                class="input-field pl-10"
                placeholder="Company Name"
              />
            </div>
          </div>

          <div>
            <label class="block text-sm font-medium text-neutral-700 mb-2">Password</label>
            <div class="relative">
              <Lock class="absolute left-3 top-3 h-5 w-5 text-neutral-400" />
              <input
                v-model="form.password"
                type="password"
                required
                class="input-field pl-10"
                placeholder="Enter your password"
              />
            </div>
          </div>

          <div v-if="!isSignup" class="flex items-center justify-between">
            <label class="flex items-center">
              <input type="checkbox" class="h-4 w-4 text-primary focus:ring-primary border-neutral-300 rounded">
              <span class="ml-2 text-sm text-neutral-600">Remember me</span>
            </label>
            <a href="#" class="text-sm text-primary hover:text-primary-light">Forgot password?</a>
          </div>

          <button type="submit" class="w-full btn-primary py-3" :disabled="loading">
            <span v-if="loading">{{ isSignup ? 'Creating Account...' : 'Signing In...' }}</span>
            <span v-else>{{ isSignup ? 'Create Account' : 'Sign In' }}</span>
          </button>
        </form>

        <div class="mt-6 pt-6 border-t border-neutral-200 text-center">
          <p class="text-sm text-neutral-600">
            {{ isSignup ? 'Already have access?' : 'Need access?' }}
            <button
              @click="toggleMode"
              class="text-primary hover:text-primary-light font-medium ml-1"
            >
              {{ isSignup ? 'Sign in' : 'Create account' }}
            </button>
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ShieldCheck, Mail, Lock, Building } from 'lucide-vue-next'
import { useAuthStore } from '../stores/auth'

export default {
  name: 'Login',
  components: {
    ShieldCheck,
    Mail,
    Lock,
    Building
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

    const currentMode = ref(props.mode || 'login')
    const loading = ref(false)
    const form = ref({
      email: '',
      password: '',
      firstName: '',
      lastName: '',
      organization: ''
    })

    const isSignup = computed(() => currentMode.value === 'signup')

    const toggleMode = () => {
      currentMode.value = currentMode.value === 'login' ? 'signup' : 'login'
      router.push(currentMode.value === 'signup' ? '/signup' : '/login')
    }

    const handleSubmit = async () => {
      loading.value = true

      try {
        if (isSignup.value) {
          await authStore.signup(form.value)
          alert('Account created! Please sign in.')
          currentMode.value = 'login'
          router.push('/login')
        } else {
          await authStore.login(form.value.email, form.value.password)
          router.push('/dashboard')
        }
      } catch (error) {
        alert(error.message || `${isSignup.value ? 'Signup' : 'Login'} failed`)
      } finally {
        loading.value = false
      }
    }

    // Set mode based on route
    if (route.path === '/signup') {
      currentMode.value = 'signup'
    }

    return {
      form,
      loading,
      isSignup,
      toggleMode,
      handleSubmit
    }
  }
}
</script>
