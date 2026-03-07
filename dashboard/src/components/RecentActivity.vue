<template>
  <div class="card">
    <div class="px-6 py-4 border-b border-neutral-200">
      <div class="flex items-center justify-between">
        <div class="flex items-center space-x-3">
          <ShieldAlert class="h-6 w-6 text-primary" />
          <h3 class="text-xl font-semibold text-neutral-800">Recent Protection Events</h3>
        </div>
        <span class="text-sm text-neutral-500">Last 24 hours</span>
      </div>
    </div>
    <div class="overflow-x-auto">
      <div v-if="loading" class="text-center py-12">
        <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary mx-auto mb-4"></div>
        <p class="text-neutral-500">Loading protection events...</p>
      </div>
      <table v-else class="w-full">
        <thead class="bg-neutral-50">
          <tr>
            <th class="px-6 py-4 text-left text-sm font-medium text-neutral-600">Time</th>
            <th class="px-6 py-4 text-left text-sm font-medium text-neutral-600">Action</th>
            <th class="px-6 py-4 text-left text-sm font-medium text-neutral-600">Data Type</th>
            <th class="px-6 py-4 text-left text-sm font-medium text-neutral-600">Protection</th>
            <th class="px-6 py-4 text-left text-sm font-medium text-neutral-600">Status</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-neutral-200">
          <tr
            v-for="event in events"
            :key="`${event.time}-${event.type}`"
            class="hover:bg-neutral-50 transition-colors"
          >
            <td class="px-6 py-4 text-sm text-neutral-800">{{ event.time }}</td>
            <td class="px-6 py-4 text-sm font-medium text-neutral-800">{{ event.action }}</td>
            <td class="px-6 py-4 text-sm text-neutral-600">{{ event.type }}</td>
            <td class="px-6 py-4 text-sm text-neutral-600">{{ event.protection }}</td>
            <td class="px-6 py-4">
              <span :class="getStatusClass(event.status)">{{ event.status }}</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script>
import { ShieldAlert } from 'lucide-vue-next'

export default {
  name: 'RecentActivity',
  components: {
    ShieldAlert
  },
  props: {
    events: {
      type: Array,
      default: () => []
    },
    loading: {
      type: Boolean,
      default: false
    }
  },
  setup() {
    const getStatusClass = (status) => {
      const baseClasses = 'px-3 py-1 rounded-full text-xs font-medium'
      if (status === 'Protected') {
        return `${baseClasses} bg-success/10 text-success`
      } else if (status === 'Blocked') {
        return `${baseClasses} bg-warning/10 text-warning`
      } else {
        return `${baseClasses} bg-neutral-100 text-neutral-600`
      }
    }

    return {
      getStatusClass
    }
  }
}
</script>
