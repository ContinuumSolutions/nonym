<template>
  <div class="card mb-8">
    <div class="gradient-bg px-6 py-4 rounded-t-2xl">
      <div class="flex items-center space-x-3">
        <TrendingUp class="h-6 w-6 text-white" />
        <div>
          <h3 class="text-xl font-semibold text-white">Protection Impact</h3>
          <p class="text-white/80 text-sm">How we're protecting your organization</p>
        </div>
      </div>
    </div>
    <div class="p-6">
      <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
        <div class="text-center">
          <div class="text-4xl font-bold text-success mb-2 counter-up">{{ protectionRate }}%</div>
          <div class="text-neutral-600 font-medium">Protection Rate</div>
          <div class="text-sm text-neutral-500">PII successfully anonymized</div>
        </div>
        <div class="text-center">
          <div class="text-4xl font-bold text-primary mb-2 counter-up">{{ dataTypes }}</div>
          <div class="text-neutral-600 font-medium">Data Types</div>
          <div class="text-sm text-neutral-500">Types of PII detected</div>
        </div>
        <div class="text-center">
          <div class="text-4xl font-bold text-warning mb-2 counter-up">{{ riskReduction }}%</div>
          <div class="text-neutral-600 font-medium">Risk Reduction</div>
          <div class="text-sm text-neutral-500">Exposure risk eliminated</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, onMounted } from 'vue'
import { TrendingUp } from 'lucide-vue-next'

export default {
  name: 'ProtectionImpact',
  components: {
    TrendingUp
  },
  setup() {
    const protectionRate = ref(0)
    const dataTypes = ref(0)
    const riskReduction = ref(0)

    onMounted(() => {
      // Animate counters
      animateValue(protectionRate, 99.8)
      animateValue(dataTypes, 8)
      animateValue(riskReduction, 94)
    })

    const animateValue = (ref, target) => {
      let current = 0
      const increment = target / 50
      const timer = setInterval(() => {
        current += increment
        if (current >= target) {
          ref.value = target
          clearInterval(timer)
        } else {
          ref.value = Math.round(current * 10) / 10
        }
      }, 40)
    }

    return {
      protectionRate,
      dataTypes,
      riskReduction
    }
  }
}
</script>
