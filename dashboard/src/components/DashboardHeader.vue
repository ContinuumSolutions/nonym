<template>
  <header class="bg-white border-b border-neutral-200 shadow-sm">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
      <div class="flex justify-between items-center h-16">
        <div class="flex items-center space-x-4">
          <div class="gradient-bg w-10 h-10 rounded-xl flex items-center justify-center">
            <ShieldCheck class="h-6 w-6 text-white" />
          </div>
          <div>
            <h1 class="text-xl font-bold text-neutral-800">Privacy Gateway</h1>
            <div class="flex items-center space-x-2 text-sm text-neutral-600">
              <div class="w-2 h-2 bg-success rounded-full animate-pulse"></div>
              <span>Live Protection Active</span>
            </div>
          </div>
        </div>
        <div class="flex items-center space-x-4">
          <button
            @click="$emit('refresh')"
            class="flex items-center space-x-2 px-3 py-2 text-neutral-600 hover:text-neutral-800 transition"
          >
            <RefreshCw class="h-4 w-4" />
            <span>Refresh</span>
          </button>
          <div class="relative">
            <button
              @click="showUserMenu = !showUserMenu"
              class="flex items-center space-x-2"
            >
              <div class="w-8 h-8 bg-primary rounded-full flex items-center justify-center">
                <User class="h-4 w-4 text-white" />
              </div>
              <ChevronDown class="h-4 w-4 text-neutral-600" />
            </button>
            <div
              v-show="showUserMenu"
              class="absolute right-0 mt-2 w-48 bg-white rounded-xl shadow-lg border border-neutral-200 py-2 z-10"
            >
              <button
                @click="$emit('logout')"
                class="w-full flex items-center px-4 py-2 text-sm text-neutral-700 hover:bg-neutral-50"
              >
                <LogOut class="h-4 w-4 mr-3" />
                Sign Out
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </header>
</template>

<script>
import { ref } from 'vue'
import { ShieldCheck, RefreshCw, User, ChevronDown, LogOut } from 'lucide-vue-next'

export default {
  name: 'DashboardHeader',
  components: {
    ShieldCheck,
    RefreshCw,
    User,
    ChevronDown,
    LogOut
  },
  emits: ['refresh', 'logout'],
  setup() {
    const showUserMenu = ref(false)

    return {
      showUserMenu
    }
  }
}
</script>
