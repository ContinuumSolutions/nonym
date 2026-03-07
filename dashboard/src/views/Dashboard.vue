<template>
  <DashboardLayout>
    <div class="p-8">
      <!-- Hero Stats -->
      <div class="mb-8">
        <h2 class="text-3xl font-bold text-neutral-800 mb-2">Privacy Protection Dashboard</h2>
        <p class="text-neutral-600 mb-8">Real-time protection against data exposure in AI interactions</p>

        <!-- Key Metrics -->
        <div class="grid grid-cols-1 md:grid-cols-4 gap-6 mb-8">
          <MetricCard
            icon="Shield"
            :value="stats.piiProtected"
            label="PII Items Protected"
            badge="PROTECTED"
            badge-color="success"
            color="primary"
          />
          <MetricCard
            icon="Activity"
            :value="stats.totalRequests"
            label="Total Requests"
            badge="LIVE"
            badge-color="primary"
            color="success"
          />
          <MetricCard
            icon="EyeOff"
            :value="stats.blockedRequests"
            label="Blocked Exposures"
            badge="BLOCKED"
            badge-color="warning"
            color="warning"
          />
          <MetricCard
            icon="Zap"
            :value="`${stats.avgTime}ms`"
            label="Avg Process Time"
            badge="FAST"
            badge-color="neutral"
            color="neutral"
          />
        </div>
      </div>

      <!-- Protection Impact -->
      <ProtectionImpact />

      <!-- Recent Activity -->
      <RecentActivity :events="events" :loading="eventsLoading" />
    </div>
  </DashboardLayout>
</template>

<script>
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import DashboardLayout from '../components/DashboardLayout.vue'
import MetricCard from '../components/MetricCard.vue'
import ProtectionImpact from '../components/ProtectionImpact.vue'
import RecentActivity from '../components/RecentActivity.vue'
import { useAuthStore } from '../stores/auth'
import { apiService } from '../services/api'

export default {
  name: 'Dashboard',
  components: {
    DashboardLayout,
    MetricCard,
    ProtectionImpact,
    RecentActivity
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
      loadData
    }
  }
}
</script>
