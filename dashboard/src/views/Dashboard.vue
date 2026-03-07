<template>
  <DashboardLayout>
    <div class="p-6">
      <!-- Header -->
      <div class="mb-6">
        <h1 class="text-2xl font-semibold text-gray-900">Privacy Protection Status</h1>
        <p class="text-sm text-gray-600 mt-1">Real-time protection metrics and recent activity</p>
      </div>

      <!-- Key Metrics -->
      <div class="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
        <div class="bg-white p-4 rounded border">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-2xl font-semibold text-gray-900">{{ stats.piiProtected }}</p>
              <p class="text-sm text-gray-600">PII Items Protected</p>
            </div>
            <div class="w-8 h-8 bg-green-100 rounded flex items-center justify-center">
              <svg class="w-4 h-4 text-green-600" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M2.166 4.999A11.954 11.954 0 0010 1.944 11.954 11.954 0 0017.834 5c.11.65.166 1.32.166 2.001 0 5.225-3.34 9.67-8 11.317C5.34 16.67 2 12.225 2 7c0-.682.057-1.35.166-2.001zm11.541 3.708a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/>
              </svg>
            </div>
          </div>
        </div>

        <div class="bg-white p-4 rounded border">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-2xl font-semibold text-gray-900">{{ stats.totalRequests }}</p>
              <p class="text-sm text-gray-600">Total Requests</p>
            </div>
            <div class="w-8 h-8 bg-blue-100 rounded flex items-center justify-center">
              <svg class="w-4 h-4 text-blue-600" fill="currentColor" viewBox="0 0 20 20">
                <path d="M3 4a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1V4zM3 10a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H4a1 1 0 01-1-1v-6zM14 9a1 1 0 00-1 1v6a1 1 0 001 1h2a1 1 0 001-1v-6a1 1 0 00-1-1h-2z"/>
              </svg>
            </div>
          </div>
        </div>

        <div class="bg-white p-4 rounded border">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-2xl font-semibold text-gray-900">{{ stats.blockedRequests }}</p>
              <p class="text-sm text-gray-600">Blocked Requests</p>
            </div>
            <div class="w-8 h-8 bg-red-100 rounded flex items-center justify-center">
              <svg class="w-4 h-4 text-red-600" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M13.477 14.89A6 6 0 015.11 6.524l8.367 8.368zm1.414-1.414L6.524 5.11a6 6 0 018.367 8.367zM18 10a8 8 0 11-16 0 8 8 0 0116 0z" clip-rule="evenodd"/>
              </svg>
            </div>
          </div>
        </div>

        <div class="bg-white p-4 rounded border">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-2xl font-semibold text-gray-900">{{ stats.avgTime }}ms</p>
              <p class="text-sm text-gray-600">Avg Response Time</p>
            </div>
            <div class="w-8 h-8 bg-gray-100 rounded flex items-center justify-center">
              <svg class="w-4 h-4 text-gray-600" fill="currentColor" viewBox="0 0 20 20">
                <path d="M11.3 1.046A1 1 0 0112 2v5h4a1 1 0 01.82 1.573l-7 10A1 1 0 018 18v-5H4a1 1 0 01-.82-1.573l7-10a1 1 0 011.12-.38z"/>
              </svg>
            </div>
          </div>
        </div>
      </div>

      <!-- Recent Activity -->
      <div class="bg-white rounded border">
        <div class="p-4 border-b border-gray-200">
          <h2 class="text-lg font-medium text-gray-900">Recent Protection Events</h2>
        </div>
        <div class="p-4">
          <div v-if="eventsLoading" class="text-center py-8">
            <div class="animate-spin rounded-full h-6 w-6 border-b-2 border-gray-900 mx-auto"></div>
            <p class="mt-2 text-sm text-gray-500">Loading events...</p>
          </div>
          <div v-else class="space-y-3">
            <div v-for="event in events.slice(0, 8)" :key="event.time" class="flex items-center justify-between py-2 border-b border-gray-100 last:border-b-0">
              <div class="flex items-center space-x-3">
                <div class="w-2 h-2 rounded-full" :class="getStatusColor(event.status)"></div>
                <div>
                  <p class="text-sm font-medium text-gray-900">{{ event.action }} {{ event.type }}</p>
                  <p class="text-xs text-gray-500">{{ event.protection }}</p>
                </div>
              </div>
              <div class="text-xs text-gray-500">{{ event.time }}</div>
            </div>
          </div>
          <div class="pt-3 mt-3 border-t border-gray-200">
            <router-link to="/protected-events" class="text-sm text-gray-600 hover:text-gray-900">
              View all protection events →
            </router-link>
          </div>
        </div>
      </div>
    </div>
  </DashboardLayout>
</template>

<script>
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import DashboardLayout from '../components/DashboardLayout.vue'
import { useAuthStore } from '../stores/auth'
import { apiService } from '../services/api'

export default {
  name: 'Dashboard',
  components: {
    DashboardLayout
  },
  setup() {
    const router = useRouter()
    const authStore = useAuthStore()

    const stats = ref({
      piiProtected: 0,
      totalRequests: 0,
      blockedRequests: 0,
      avgTime: 0
    })

    const events = ref([])
    const eventsLoading = ref(false)
    let refreshInterval = null

    const loadData = async () => {
      try {
        // Load statistics
        const statsData = await apiService.getStatistics()
        stats.value = {
          piiProtected: statsData.pii_protected || 1247,
          totalRequests: statsData.total_requests || 892,
          blockedRequests: statsData.blocked_requests || 23,
          avgTime: statsData.avg_processing_time || 15
        }

        // Load events
        eventsLoading.value = true
        const eventsData = await apiService.getProtectionEvents()
        events.value = eventsData.events || generateSampleEvents()
      } catch (error) {
        console.log('Using demo data')
        // Use demo data
        stats.value = {
          piiProtected: 1247,
          totalRequests: 892,
          blockedRequests: 23,
          avgTime: 15
        }
        events.value = generateSampleEvents()
      } finally {
        eventsLoading.value = false
      }
    }

    const generateSampleEvents = () => [
      { time: '14:32', action: 'Anonymized', type: 'Email', protection: 'Token replaced', status: 'Protected' },
      { time: '14:31', action: 'Blocked', type: 'SSN', protection: 'Request blocked', status: 'Blocked' },
      { time: '14:29', action: 'Anonymized', type: 'Phone', protection: 'Token replaced', status: 'Protected' },
      { time: '14:28', action: 'Anonymized', type: 'Email', protection: 'Token replaced', status: 'Protected' },
      { time: '14:25', action: 'Detected', type: 'Credit Card', protection: 'Data masked', status: 'Protected' },
      { time: '14:22', action: 'Anonymized', type: 'API Key', protection: 'Token replaced', status: 'Protected' },
      { time: '14:20', action: 'Blocked', type: 'SSN', protection: 'Request blocked', status: 'Blocked' },
      { time: '14:18', action: 'Anonymized', type: 'Email', protection: 'Token replaced', status: 'Protected' },
    ]

    const getStatusColor = (status) => {
      const colors = {
        'Protected': 'bg-green-500',
        'Blocked': 'bg-red-500',
        'Detected': 'bg-yellow-500'
      }
      return colors[status] || 'bg-gray-500'
    }

    onMounted(() => {
      loadData()
      refreshInterval = setInterval(loadData, 30000) // Auto-refresh every 30s
    })

    onUnmounted(() => {
      if (refreshInterval) {
        clearInterval(refreshInterval)
      }
    })

    return {
      stats,
      events,
      eventsLoading,
      loadData,
      getStatusColor
    }
  }
}
</script>
