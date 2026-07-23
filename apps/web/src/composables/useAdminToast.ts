import { ref } from 'vue'

export type AdminToastType = 'success' | 'error' | 'info'

export interface AdminToast {
  id: number
  message: string
  type: AdminToastType
}

const toasts = ref<AdminToast[]>([])
let nextToastId = 1

function dismissToast(id: number) {
  toasts.value = toasts.value.filter((toast) => toast.id !== id)
}

function showToast(message: string, type: AdminToastType = 'info', duration?: number) {
  const id = nextToastId++
  toasts.value.push({ id, message, type })
  window.setTimeout(() => dismissToast(id), duration ?? (type === 'error' ? 5000 : 3000))
  return id
}

export function useAdminToast() {
  return {
    toasts,
    dismissToast,
    showToast,
    success: (message: string) => showToast(message, 'success'),
    error: (message: string) => showToast(message, 'error'),
    info: (message: string) => showToast(message, 'info'),
  }
}
