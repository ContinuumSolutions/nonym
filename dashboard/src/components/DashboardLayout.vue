<template>
  <div class="flex h-screen bg-gray-50">
    <Sidebar @logout="handleLogout" />
    <div class="flex-1 overflow-hidden">
      <!-- Main content area -->
      <div class="h-full overflow-auto">
        <slot />
      </div>
    </div>
  </div>
</template>

<script>
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import Sidebar from './Sidebar.vue'

export default {
  name: 'DashboardLayout',
  components: {
    Sidebar
  },
  setup() {
    const router = useRouter()
    const authStore = useAuthStore()

    const handleLogout = () => {
      authStore.logout()
      router.push('/login')
    }

    return {
      handleLogout
    }
  }
}
</script>
