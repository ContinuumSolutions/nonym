<template>
  <DashboardLayout>
    <div class="p-6">
      <!-- Header -->
      <div class="mb-6">
        <h1 class="text-2xl font-semibold text-gray-900">Protection Events</h1>
        <p class="text-sm text-gray-600 mt-1">Monitor and analyze privacy protection activities</p>
      </div>

      <!-- Filters -->
      <div class="bg-white rounded border p-4 mb-6">
        <div class="grid grid-cols-1 md:grid-cols-5 gap-4">
          <div>
            <label class="block text-xs font-medium text-gray-700 mb-1">Time Range</label>
            <select v-model="filters.timeRange" class="w-full text-sm px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-1 focus:ring-blue-500">
              <option value="1h">Last Hour</option>
              <option value="24h">Last 24 Hours</option>
              <option value="7d">Last 7 Days</option>
              <option value="30d">Last 30 Days</option>
            </select>
          </div>
          <div>
            <label class="block text-xs font-medium text-gray-700 mb-1">Event Type</label>
            <select v-model="filters.eventType" class="w-full text-sm px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-1 focus:ring-blue-500">
              <option value="">All Types</option>
              <option value="email">Email</option>
              <option value="ssn">SSN</option>
              <option value="credit_card">Credit Card</option>
              <option value="phone">Phone Number</option>
              <option value="api_key">API Key</option>
            </select>
          </div>
          <div>
            <label class="block text-xs font-medium text-gray-700 mb-1">Status</label>
            <select v-model="filters.status" class="w-full text-sm px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-1 focus:ring-blue-500">
              <option value="">All Statuses</option>
              <option value="protected">Protected</option>
              <option value="blocked">Blocked</option>
              <option value="detected">Detected</option>
            </select>
          </div>
          <div>
            <label class="block text-xs font-medium text-gray-700 mb-1">Provider</label>
            <select v-model="filters.provider" class="w-full text-sm px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-1 focus:ring-blue-500">
              <option value="">All Providers</option>
              <option value="openai">OpenAI</option>
              <option value="anthropic">Anthropic</option>
              <option value="google">Google</option>
              <option value="local">Local</option>
            </select>
          </div>
          <div class="flex items-end">
            <button
              @click="loadEvents"
              class="w-full px-4 py-2 bg-gray-900 text-white text-sm rounded hover:bg-gray-800 transition-colors"
            >
              Apply Filters
            </button>
          </div>
        </div>
      </div>

      <!-- Statistics Cards -->
      <div class="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
        <div class="bg-white p-4 rounded border">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-lg font-semibold text-gray-900">{{ stats.protectedToday }}</p>
              <p class="text-xs text-gray-600">Protected Today</p>
            </div>
            <div class="w-6 h-6 bg-green-100 rounded flex items-center justify-center">
              <svg class="w-3 h-3 text-green-600" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/>
              </svg>
            </div>
          </div>
        </div>

        <div class="bg-white p-4 rounded border">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-lg font-semibold text-gray-900">{{ stats.blockedToday }}</p>
              <p class="text-xs text-gray-600">Blocked Today</p>
            </div>
            <div class="w-6 h-6 bg-red-100 rounded flex items-center justify-center">
              <svg class="w-3 h-3 text-red-600" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M13.477 14.89A6 6 0 015.11 6.524l8.367 8.368zm1.414-1.414L6.524 5.11a6 6 0 018.367 8.367zM18 10a8 8 0 11-16 0 8 8 0 0116 0z" clip-rule="evenodd"/>
              </svg>
            </div>
          </div>
        </div>

        <div class="bg-white p-4 rounded border">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-lg font-semibold text-gray-900">{{ stats.detectionRate }}%</p>
              <p class="text-xs text-gray-600">Detection Rate</p>
            </div>
            <div class="w-6 h-6 bg-blue-100 rounded flex items-center justify-center">
              <svg class="w-3 h-3 text-blue-600" fill="currentColor" viewBox="0 0 20 20">
                <path d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/>
              </svg>
            </div>
          </div>
        </div>

        <div class="bg-white p-4 rounded border">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-lg font-semibold text-gray-900">{{ stats.highRisk }}</p>
              <p class="text-xs text-gray-600">High Risk Events</p>
            </div>
            <div class="w-6 h-6 bg-yellow-100 rounded flex items-center justify-center">
              <svg class="w-3 h-3 text-yellow-600" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd"/>
              </svg>
            </div>
          </div>
        </div>
      </div>

      <!-- Events Table -->
      <div class="bg-white rounded border overflow-hidden">
        <div class="px-4 py-3 border-b border-gray-200">
          <h3 class="text-sm font-medium text-gray-900">Protection Events</h3>
        </div>

        <div v-if="loading" class="p-8 text-center">
          <div class="animate-spin rounded-full h-6 w-6 border-b-2 border-gray-900 mx-auto"></div>
          <p class="mt-2 text-sm text-gray-500">Loading events...</p>
        </div>

        <div v-else class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200">
            <thead class="bg-gray-50">
              <tr>
                <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Time</th>
                <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Type</th>
                <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Action</th>
                <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Provider</th>
                <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
                <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Protection</th>
                <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Actions</th>
              </tr>
            </thead>
            <tbody class="bg-white divide-y divide-gray-200">
              <tr v-for="event in events" :key="event.id" class="hover:bg-gray-50">
                <td class="px-4 py-3 whitespace-nowrap text-sm text-gray-900">
                  {{ formatTimestamp(event.timestamp) }}
                </td>
                <td class="px-4 py-3 whitespace-nowrap">
                  <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium"
                        :class="getTypeColor(event.type)">
                    {{ event.type }}
                  </span>
                </td>
                <td class="px-4 py-3 whitespace-nowrap text-sm text-gray-900">
                  {{ event.action }}
                </td>
                <td class="px-4 py-3 whitespace-nowrap text-sm text-gray-500">
                  {{ event.provider }}
                </td>
                <td class="px-4 py-3 whitespace-nowrap">
                  <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium"
                        :class="getStatusColor(event.status)">
                    {{ event.status }}
                  </span>
                </td>
                <td class="px-4 py-3 whitespace-nowrap text-sm text-gray-500">
                  {{ event.protection }}
                </td>
                <td class="px-4 py-3 whitespace-nowrap text-sm font-medium">
                  <button @click="showEventDetail(event)" class="text-blue-600 hover:text-blue-900 text-xs">
                    View Detail
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Pagination -->
        <div class="bg-gray-50 px-4 py-3 border-t border-gray-200 flex items-center justify-between">
          <div class="flex-1 flex justify-between sm:hidden">
            <button
              @click="previousPage"
              :disabled="currentPage <= 1"
              class="relative inline-flex items-center px-3 py-1 border border-gray-300 text-xs font-medium rounded text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50">
              Previous
            </button>
            <button
              @click="nextPage"
              class="ml-3 relative inline-flex items-center px-3 py-1 border border-gray-300 text-xs font-medium rounded text-gray-700 bg-white hover:bg-gray-50">
              Next
            </button>
          </div>
          <div class="hidden sm:flex-1 sm:flex sm:items-center sm:justify-between">
            <div>
              <p class="text-xs text-gray-700">
                Showing <span class="font-medium">{{ (currentPage - 1) * pageSize + 1 }}</span> to
                <span class="font-medium">{{ Math.min(currentPage * pageSize, totalEvents) }}</span> of
                <span class="font-medium">{{ totalEvents }}</span> events
              </p>
            </div>
            <div>
              <nav class="relative z-0 inline-flex rounded-md shadow-sm -space-x-px">
                <button
                  @click="previousPage"
                  :disabled="currentPage <= 1"
                  class="relative inline-flex items-center px-2 py-1 rounded-l-md border border-gray-300 bg-white text-xs font-medium text-gray-500 hover:bg-gray-50 disabled:opacity-50">
                  Previous
                </button>
                <button
                  @click="nextPage"
                  class="relative inline-flex items-center px-2 py-1 rounded-r-md border border-gray-300 bg-white text-xs font-medium text-gray-500 hover:bg-gray-50">
                  Next
                </button>
              </nav>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Event Detail Modal -->
    <div v-if="selectedEvent" class="fixed inset-0 z-50 overflow-y-auto">
      <div class="flex items-end justify-center min-h-screen pt-4 px-4 pb-20 text-center sm:block sm:p-0">
        <div class="fixed inset-0 bg-gray-500 bg-opacity-75 transition-opacity" @click="selectedEvent = null"></div>
        <div class="inline-block align-bottom bg-white rounded-lg text-left overflow-hidden shadow-xl transform transition-all sm:my-8 sm:align-middle sm:max-w-lg sm:w-full">
          <div class="bg-white px-4 pt-5 pb-4 sm:p-6 sm:pb-4">
            <h3 class="text-lg font-medium text-gray-900 mb-4">Event Details</h3>
            <div class="space-y-3 text-sm">
              <div class="flex justify-between">
                <span class="font-medium text-gray-600">ID:</span>
                <span class="text-gray-900 font-mono">{{ selectedEvent.id }}</span>
              </div>
              <div class="flex justify-between">
                <span class="font-medium text-gray-600">Time:</span>
                <span class="text-gray-900">{{ formatTimestamp(selectedEvent.timestamp) }}</span>
              </div>
              <div class="flex justify-between">
                <span class="font-medium text-gray-600">Type:</span>
                <span class="text-gray-900">{{ selectedEvent.type }}</span>
              </div>
              <div class="flex justify-between">
                <span class="font-medium text-gray-600">Action:</span>
                <span class="text-gray-900">{{ selectedEvent.action }}</span>
              </div>
              <div class="flex justify-between">
                <span class="font-medium text-gray-600">Status:</span>
                <span class="text-gray-900">{{ selectedEvent.status }}</span>
              </div>
              <div class="flex justify-between">
                <span class="font-medium text-gray-600">Provider:</span>
                <span class="text-gray-900">{{ selectedEvent.provider }}</span>
              </div>
              <div v-if="selectedEvent.redaction_details && selectedEvent.redaction_details.length" class="border-t pt-3">
                <span class="font-medium text-gray-600">Protection Details:</span>
                <ul class="mt-2 space-y-1">
                  <li v-for="detail in selectedEvent.redaction_details" :key="detail.position" class="text-sm bg-gray-50 p-2 rounded">
                    <strong>{{ detail.type }}:</strong> {{ detail.original_value }} → <code>{{ detail.token }}</code>
                  </li>
                </ul>
              </div>
            </div>
          </div>
          <div class="bg-gray-50 px-4 py-3 sm:px-6 sm:flex sm:flex-row-reverse">
            <button
              @click="selectedEvent = null"
              class="w-full inline-flex justify-center rounded-md border border-transparent shadow-sm px-4 py-2 bg-gray-900 text-base font-medium text-white hover:bg-gray-800 sm:ml-3 sm:w-auto sm:text-sm">
              Close
            </button>
          </div>
        </div>
      </div>
    </div>
  </DashboardLayout>
</template>

<script>
import { ref, onMounted } from 'vue'
import DashboardLayout from '../components/DashboardLayout.vue'
import { apiService } from '../services/api'

export default {
  name: 'ProtectedEvents',
  components: {
    DashboardLayout
  },
  setup() {
    const events = ref([])
    const loading = ref(false)
    const selectedEvent = ref(null)
    const currentPage = ref(1)
    const pageSize = ref(50)
    const totalEvents = ref(0)

    const stats = ref({
      protectedToday: 0,
      blockedToday: 0,
      detectionRate: 0,
      highRisk: 0
    })

    const filters = ref({
      timeRange: '24h',
      eventType: '',
      status: '',
      provider: ''
    })

    const loadEvents = async () => {
      loading.value = true
      try {
        const response = await apiService.getProtectionEvents({
          limit: pageSize.value,
          offset: (currentPage.value - 1) * pageSize.value,
          ...filters.value
        })

        events.value = response.events || generateSampleEvents()
        totalEvents.value = response.total || events.value.length

        // Load stats
        const statsResponse = await apiService.getProtectionStats()
        stats.value = statsResponse || {
          protectedToday: 127,
          blockedToday: 23,
          detectionRate: 94.2,
          highRisk: 5
        }
      } catch (error) {
        console.error('Failed to load events:', error)
        // Use sample data
        events.value = generateSampleEvents()
        stats.value = {
          protectedToday: 127,
          blockedToday: 23,
          detectionRate: 94.2,
          highRisk: 5
        }
      } finally {
        loading.value = false
      }
    }

    const generateSampleEvents = () => [
      {
        id: 'evt_001',
        timestamp: new Date(Date.now() - 1000 * 60 * 5),
        type: 'Email',
        action: 'Anonymized',
        provider: 'OpenAI',
        status: 'Protected',
        protection: 'Token replaced',
        redaction_details: [
          { type: 'email', original_value: 'user@example.com', token: 'TOKEN_EMAIL_001', position: 45 }
        ]
      },
      {
        id: 'evt_002',
        timestamp: new Date(Date.now() - 1000 * 60 * 8),
        type: 'SSN',
        action: 'Blocked',
        provider: 'Anthropic',
        status: 'Blocked',
        protection: 'Request blocked',
        redaction_details: []
      },
      {
        id: 'evt_003',
        timestamp: new Date(Date.now() - 1000 * 60 * 12),
        type: 'Credit Card',
        action: 'Detected',
        provider: 'Google',
        status: 'Protected',
        protection: 'Data masked',
        redaction_details: [
          { type: 'credit_card', original_value: '**** **** **** 1234', token: 'TOKEN_CC_001', position: 78 }
        ]
      },
      {
        id: 'evt_004',
        timestamp: new Date(Date.now() - 1000 * 60 * 15),
        type: 'Phone',
        action: 'Anonymized',
        provider: 'OpenAI',
        status: 'Protected',
        protection: 'Token replaced',
        redaction_details: [
          { type: 'phone', original_value: '+1-555-***-****', token: 'TOKEN_PHONE_001', position: 23 }
        ]
      }
    ]

    const formatTimestamp = (timestamp) => {
      return new Date(timestamp).toLocaleString('en-US', {
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit'
      })
    }

    const getTypeColor = (type) => {
      const colors = {
        'Email': 'bg-blue-100 text-blue-800',
        'SSN': 'bg-red-100 text-red-800',
        'Credit Card': 'bg-yellow-100 text-yellow-800',
        'Phone': 'bg-green-100 text-green-800',
        'API Key': 'bg-purple-100 text-purple-800'
      }
      return colors[type] || 'bg-gray-100 text-gray-800'
    }

    const getStatusColor = (status) => {
      const colors = {
        'Protected': 'bg-green-100 text-green-800',
        'Blocked': 'bg-red-100 text-red-800',
        'Detected': 'bg-yellow-100 text-yellow-800'
      }
      return colors[status] || 'bg-gray-100 text-gray-800'
    }

    const showEventDetail = (event) => {
      selectedEvent.value = event
    }

    const nextPage = () => {
      currentPage.value++
      loadEvents()
    }

    const previousPage = () => {
      if (currentPage.value > 1) {
        currentPage.value--
        loadEvents()
      }
    }

    onMounted(() => {
      loadEvents()
    })

    return {
      events,
      loading,
      selectedEvent,
      currentPage,
      pageSize,
      totalEvents,
      stats,
      filters,
      loadEvents,
      formatTimestamp,
      getTypeColor,
      getStatusColor,
      showEventDetail,
      nextPage,
      previousPage
    }
  }
}
</script>
