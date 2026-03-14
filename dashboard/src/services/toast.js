import { ref } from 'vue'

const toasts = ref([])
let toastId = 0

const createToast = (options) => {
  const id = ++toastId
  const toast = {
    id,
    type: 'info',
    title: '',
    message: '',
    duration: 5000,
    showProgress: true,
    ...options
  }

  toasts.value.push(toast)

  return id
}

const removeToast = (id) => {
  const index = toasts.value.findIndex(toast => toast.id === id)
  if (index > -1) {
    toasts.value.splice(index, 1)
  }
}

const clearToasts = () => {
  toasts.value = []
}

// Convenience methods
const success = (title, message = '', options = {}) => {
  return createToast({ type: 'success', title, message, ...options })
}

const error = (title, message = '', options = {}) => {
  return createToast({ type: 'error', title, message, duration: 8000, ...options })
}

const warning = (title, message = '', options = {}) => {
  return createToast({ type: 'warning', title, message, duration: 6000, ...options })
}

const info = (title, message = '', options = {}) => {
  return createToast({ type: 'info', title, message, ...options })
}

// Handle API errors gracefully
const handleApiError = (error, defaultMessage = 'Something went wrong') => {
  let title = defaultMessage
  let message = ''

  if (error.response?.data) {
    const data = error.response.data

    // Handle different error formats
    if (typeof data.error === 'string') {
      title = data.error
    } else if (data.message) {
      title = data.message
    } else if (data.errors) {
      // Handle validation errors
      if (Array.isArray(data.errors)) {
        title = 'Validation Error'
        message = data.errors.join(', ')
      } else if (typeof data.errors === 'object') {
        title = 'Validation Error'
        message = Object.values(data.errors).flat().join(', ')
      }
    }
  } else if (error.message) {
    title = error.message
  }

  return error({
    title,
    message,
    duration: 8000
  })
}

// Format success messages
const handleApiSuccess = (response, defaultMessage = 'Success') => {
  let title = defaultMessage
  let message = ''

  if (response?.data?.message) {
    title = response.data.message
  } else if (response?.message) {
    title = response.message
  }

  return success({
    title,
    message,
    duration: 4000
  })
}

export const useToast = () => {
  return {
    toasts,
    success,
    error,
    warning,
    info,
    removeToast,
    clearToasts,
    handleApiError,
    handleApiSuccess
  }
}