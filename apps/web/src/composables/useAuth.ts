import { readonly, ref } from 'vue'
import { api } from '@/services/api'
import type { AuthState } from '@/types/blog'

const state = ref<AuthState | null>(null)
let refreshPromise: Promise<AuthState> | null = null
let authRevision = 0
let lastRefreshAt = 0
const authFreshnessMs = 5_000
export const authLogoutStorageKey = 'myblog:logout'

function loggedOutState(): AuthState {
  return { authenticated: false, user: null, csrfToken: null }
}

async function refreshAuth(force = false) {
  if (refreshPromise) return refreshPromise
  if (!force && state.value && Date.now() - lastRefreshAt < authFreshnessMs) {
    return state.value
  }
  const revision = authRevision
  refreshPromise = api.getAuthState()
    .then((next) => {
      // A logout in this or another tab invalidates any /auth/me response
      // that was already in flight before the logout completed.
      if (revision !== authRevision) return state.value ?? loggedOutState()
      state.value = next
      lastRefreshAt = Date.now()
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
  await api.logout()
  authRevision += 1
  lastRefreshAt = Date.now()
  localStorage.removeItem('ArtalkUser')
  localStorage.setItem(authLogoutStorageKey, String(Date.now()))
  state.value = loggedOutState()
  window.dispatchEvent(new CustomEvent('myblog:auth-changed'))
}

if (typeof window !== 'undefined') {
  window.addEventListener('storage', (event) => {
    if (event.key !== authLogoutStorageKey) return
    authRevision += 1
    lastRefreshAt = Date.now()
    localStorage.removeItem('ArtalkUser')
    state.value = loggedOutState()
    window.dispatchEvent(new CustomEvent('myblog:auth-changed'))
  })
}

export function useAuth() {
  return {
    authState: readonly(state),
    refreshAuth,
    logout,
  }
}
