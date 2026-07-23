import { readonly, ref } from 'vue'
import { api } from '@/services/api'
import type { AuthState } from '@/types/blog'

const state = ref<AuthState | null>(null)
let refreshPromise: Promise<AuthState> | null = null

async function refreshAuth() {
  if (refreshPromise) return refreshPromise
  refreshPromise = api.getAuthState()
    .then((next) => {
      state.value = next
      if (!next.authenticated) localStorage.removeItem('ArtalkUser')
      return next
    })
    .finally(() => {
      refreshPromise = null
    })
  return refreshPromise
}

async function logout() {
  if (!state.value?.csrfToken) await refreshAuth()
  try {
    await api.logout()
  } finally {
    localStorage.removeItem('ArtalkUser')
    state.value = { authenticated: false, user: null, csrfToken: null }
    window.dispatchEvent(new CustomEvent('myblog:auth-changed'))
  }
}

export function useAuth() {
  return {
    authState: readonly(state),
    refreshAuth,
    logout,
  }
}
