<template>
  <div class="h-screen w-64 bg-white border-r border-gray-200 flex flex-col">
    <!-- Logo/Brand -->
    <div class="p-6 border-b border-gray-200">
      <div class="flex items-center">
        <div class="w-8 h-8 bg-blue-600 rounded-lg flex items-center justify-center">
          <svg class="w-5 h-5 text-white" fill="currentColor" viewBox="0 0 20 20">
            <path d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/>
          </svg>
        </div>
        <h1 class="ml-3 text-xl font-semibold text-gray-900">SPG</h1>
      </div>
    </div>

    <!-- Navigation -->
    <nav class="flex-1 px-4 py-6 space-y-1">
      <router-link
        v-for="item in menuItems"
        :key="item.name"
        :to="item.path"
        class="group flex items-center px-3 py-2 text-sm font-medium rounded-md transition-colors duration-150"
        :class="[
          $route.path === item.path
            ? 'bg-blue-50 border-r-2 border-blue-500 text-blue-700'
            : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900'
        ]"
      >
        <component
          :is="item.icon"
          class="mr-3 h-5 w-5"
          :class="[
            $route.path === item.path ? 'text-blue-500' : 'text-gray-400 group-hover:text-gray-500'
          ]"
        />
        {{ item.name }}
        <span
          v-if="item.badge"
          class="ml-auto bg-red-100 text-red-800 text-xs px-2 py-0.5 rounded-full"
        >
          {{ item.badge }}
        </span>
      </router-link>
    </nav>

    <!-- User Info -->
    <div class="p-4 border-t border-gray-200">
      <div class="flex items-center">
        <div class="w-8 h-8 bg-gray-300 rounded-full flex items-center justify-center">
          <svg class="w-4 h-4 text-gray-600" fill="currentColor" viewBox="0 0 20 20">
            <path fill-rule="evenodd" d="M10 9a3 3 0 100-6 3 3 0 000 6zm-7 9a7 7 0 1114 0H3z" clip-rule="evenodd"/>
          </svg>
        </div>
        <div class="ml-3 flex-1">
          <p class="text-sm font-medium text-gray-900">{{ user?.name || 'User' }}</p>
          <p class="text-xs text-gray-500">{{ user?.email }}</p>
        </div>
        <button
          @click="$emit('logout')"
          class="ml-2 p-1 text-gray-400 hover:text-gray-600"
          title="Logout"
        >
          <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
            <path fill-rule="evenodd" d="M3 3a1 1 0 00-1 1v12a1 1 0 102 0V4a1 1 0 00-1-1zm10.293 9.293a1 1 0 001.414 1.414l3-3a1 1 0 000-1.414l-3-3a1 1 0 10-1.414 1.414L14.586 9H7a1 1 0 100 2h7.586l-1.293 1.293z" clip-rule="evenodd"/>
          </svg>
        </button>
      </div>
    </div>
  </div>
</template>

<script>
import { computed } from 'vue'
import { useAuthStore } from '../stores/auth'

// Icon components
const DashboardIcon = {
  template: `<svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
    <path d="M3 4a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1V4zM3 10a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H4a1 1 0 01-1-1v-6zM14 9a1 1 0 00-1 1v6a1 1 0 001 1h2a1 1 0 001-1v-6a1 1 0 00-1-1h-2z"/>
  </svg>`
}

const ShieldIcon = {
  template: `<svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
    <path fill-rule="evenodd" d="M2.166 4.999A11.954 11.954 0 0010 1.944 11.954 11.954 0 0017.834 5c.11.65.166 1.32.166 2.001 0 5.225-3.34 9.67-8 11.317C5.34 16.67 2 12.225 2 7c0-.682.057-1.35.166-2.001zm11.541 3.708a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/>
  </svg>`
}

const CogIcon = {
  template: `<svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
    <path fill-rule="evenodd" d="M11.49 3.17c-.38-1.56-2.6-1.56-2.98 0a1.532 1.532 0 01-2.286.948c-1.372-.836-2.942.734-2.106 2.106.54.886.061 2.042-.947 2.287-1.561.379-1.561 2.6 0 2.978a1.532 1.532 0 01.947 2.287c-.836 1.372.734 2.942 2.106 2.106a1.532 1.532 0 012.287.947c.379 1.561 2.6 1.561 2.978 0a1.533 1.533 0 012.287-.947c1.372.836 2.942-.734 2.106-2.106a1.533 1.533 0 01.947-2.287c1.561-.379 1.561-2.6 0-2.978a1.532 1.532 0 01-.947-2.287c.836-1.372-.734-2.942-2.106-2.106a1.532 1.532 0 01-2.287-.947zM10 13a3 3 0 100-6 3 3 0 000 6z" clip-rule="evenodd"/>
  </svg>`
}

const UserIcon = {
  template: `<svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
    <path fill-rule="evenodd" d="M10 2a4 4 0 00-4 4v1H5a1 1 0 00-.994.89l-1 9A1 1 0 004 18h12a1 1 0 00.994-1.11l-1-9A1 1 0 0015 7h-1V6a4 4 0 00-4-4zm2 5V6a2 2 0 10-4 0v1h4zm-6 3a1 1 0 112 0 1 1 0 01-2 0zm7-1a1 1 0 100 2 1 1 0 000-2z" clip-rule="evenodd"/>
  </svg>`
}

export default {
  name: 'Sidebar',
  emits: ['logout'],
  components: {
    DashboardIcon,
    ShieldIcon,
    CogIcon,
    UserIcon
  },
  setup() {
    const authStore = useAuthStore()

    const user = computed(() => authStore.user)

    const menuItems = [
      { name: 'Dashboard', path: '/dashboard', icon: 'DashboardIcon' },
      { name: 'Protected Events', path: '/protected-events', icon: 'ShieldIcon' },
      { name: 'Integrations', path: '/integrations', icon: 'CogIcon' },
      { name: 'Account', path: '/account', icon: 'UserIcon' }
    ]

    return {
      user,
      menuItems
    }
  }
}
</script>
