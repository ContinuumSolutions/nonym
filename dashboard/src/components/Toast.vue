<template>
  <Transition name="toast">
    <div
      v-if="visible"
      :class="[
        'fixed top-4 right-4 max-w-md w-full bg-white border border-gray-200 rounded-lg shadow-lg z-50',
        'transform transition-all duration-300 ease-in-out',
        typeClasses
      ]"
    >
      <div class="p-4">
        <div class="flex items-start">
          <div class="flex-shrink-0">
            <component :is="iconComponent" :class="['h-5 w-5', iconColorClasses]" />
          </div>
          <div class="ml-3 w-0 flex-1">
            <p class="text-sm font-medium text-gray-900">
              {{ title }}
            </p>
            <p v-if="message" class="mt-1 text-sm text-gray-500">
              {{ message }}
            </p>
          </div>
          <div class="ml-4 flex-shrink-0 flex">
            <button
              @click="close"
              class="inline-flex text-gray-400 hover:text-gray-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
            >
              <X class="h-5 w-5" />
            </button>
          </div>
        </div>
      </div>
      <div
        v-if="showProgress"
        :class="['h-1 bg-current opacity-20 transition-all duration-linear', progressColorClasses]"
        :style="{ width: `${progress}%` }"
      ></div>
    </div>
  </Transition>
</template>

<script>
import { ref, computed, onMounted } from 'vue'
import { CheckCircle, XCircle, AlertTriangle, Info, X } from 'lucide-vue-next'

export default {
  name: 'Toast',
  components: {
    CheckCircle,
    XCircle,
    AlertTriangle,
    Info,
    X
  },
  props: {
    type: {
      type: String,
      default: 'info',
      validator: (value) => ['success', 'error', 'warning', 'info'].includes(value)
    },
    title: {
      type: String,
      required: true
    },
    message: {
      type: String,
      default: ''
    },
    duration: {
      type: Number,
      default: 5000
    },
    showProgress: {
      type: Boolean,
      default: true
    }
  },
  emits: ['close'],
  setup(props, { emit }) {
    const visible = ref(false)
    const progress = ref(100)
    let timer = null
    let progressTimer = null

    const iconComponent = computed(() => {
      const icons = {
        success: CheckCircle,
        error: XCircle,
        warning: AlertTriangle,
        info: Info
      }
      return icons[props.type]
    })

    const typeClasses = computed(() => {
      const classes = {
        success: 'border-green-200 bg-green-50',
        error: 'border-red-200 bg-red-50',
        warning: 'border-yellow-200 bg-yellow-50',
        info: 'border-blue-200 bg-blue-50'
      }
      return classes[props.type]
    })

    const iconColorClasses = computed(() => {
      const classes = {
        success: 'text-green-400',
        error: 'text-red-400',
        warning: 'text-yellow-400',
        info: 'text-blue-400'
      }
      return classes[props.type]
    })

    const progressColorClasses = computed(() => {
      const classes = {
        success: 'text-green-400',
        error: 'text-red-400',
        warning: 'text-yellow-400',
        info: 'text-blue-400'
      }
      return classes[props.type]
    })

    const close = () => {
      visible.value = false
      if (timer) clearTimeout(timer)
      if (progressTimer) clearInterval(progressTimer)
      setTimeout(() => emit('close'), 300)
    }

    const show = () => {
      visible.value = true

      if (props.duration > 0) {
        // Progress bar animation
        if (props.showProgress) {
          const interval = 50
          const step = (interval / props.duration) * 100

          progressTimer = setInterval(() => {
            progress.value -= step
            if (progress.value <= 0) {
              clearInterval(progressTimer)
            }
          }, interval)
        }

        // Auto close
        timer = setTimeout(close, props.duration)
      }
    }

    onMounted(() => {
      show()
    })

    return {
      visible,
      progress,
      iconComponent,
      typeClasses,
      iconColorClasses,
      progressColorClasses,
      close
    }
  }
}
</script>

<style scoped>
.toast-enter-active {
  transition: all 0.3s ease-out;
}

.toast-leave-active {
  transition: all 0.3s ease-in;
}

.toast-enter-from {
  transform: translateX(100%);
  opacity: 0;
}

.toast-leave-to {
  transform: translateX(100%);
  opacity: 0;
}

@media (max-width: 640px) {
  .fixed.top-4.right-4 {
    top: 1rem;
    right: 1rem;
    left: 1rem;
    max-width: none;
  }
}
</style>