<template>
  <div class="card p-6">
    <div class="flex items-center justify-between mb-4">
      <div :class="iconClasses">
        <component :is="iconComponent" class="h-6 w-6" />
      </div>
      <span :class="badgeClasses">{{ badge }}</span>
    </div>
    <div class="text-3xl font-bold text-neutral-800 counter-up">{{ displayValue }}</div>
    <div class="text-sm text-neutral-600">{{ label }}</div>
  </div>
</template>

<script>
import { computed, onMounted, ref } from 'vue'
import { Shield, Activity, EyeOff, Zap } from 'lucide-vue-next'

export default {
  name: 'MetricCard',
  props: {
    icon: {
      type: String,
      required: true
    },
    value: {
      type: [String, Number],
      required: true
    },
    label: {
      type: String,
      required: true
    },
    badge: {
      type: String,
      required: true
    },
    badgeColor: {
      type: String,
      default: 'primary'
    },
    color: {
      type: String,
      default: 'primary'
    }
  },
  setup(props) {
    const displayValue = ref(0)

    const iconComponent = computed(() => {
      const icons = {
        Shield,
        Activity,
        EyeOff,
        Zap
      }
      return icons[props.icon] || Shield
    })

    const iconClasses = computed(() => {
      const baseClasses = 'w-12 h-12 rounded-xl flex items-center justify-center'
      const colorClasses = {
        primary: 'bg-primary/10 text-primary',
        success: 'bg-success/10 text-success',
        warning: 'bg-warning/10 text-warning',
        neutral: 'bg-neutral-600/10 text-neutral-600'
      }
      return `${baseClasses} ${colorClasses[props.color] || colorClasses.primary}`
    })

    const badgeClasses = computed(() => {
      const baseClasses = 'text-xs font-medium px-2 py-1 rounded-full'
      const colorClasses = {
        primary: 'text-primary bg-primary/10',
        success: 'text-success bg-success/10',
        warning: 'text-warning bg-warning/10',
        neutral: 'text-neutral-600 bg-neutral-100'
      }
      return `${baseClasses} ${colorClasses[props.badgeColor] || colorClasses.primary}`
    })

    // Animate counter if it's a number
    onMounted(() => {
      const numericValue = parseInt(props.value)
      if (!isNaN(numericValue)) {
        let current = 0
        const increment = numericValue / 50
        const timer = setInterval(() => {
          current += increment
          if (current >= numericValue) {
            displayValue.value = numericValue
            clearInterval(timer)
          } else {
            displayValue.value = Math.round(current)
          }
        }, 40)
      } else {
        displayValue.value = props.value
      }
    })

    return {
      displayValue,
      iconComponent,
      iconClasses,
      badgeClasses
    }
  }
}
</script>
