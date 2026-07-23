<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { useTheme } from '@/composables/useTheme'
import { useAdminToast } from '@/composables/useAdminToast'
import { useAuth } from '@/composables/useAuth'
import type { AuthUser } from '@/types/blog'
import AdminToastHost from './AdminToastHost.vue'

const logoutStorageKey = 'myblog:admin-logout'
const router = useRouter()
const user = ref<AuthUser | null>(null)
const menuOpen = ref(false)
const loggingOut = ref(false)
const { isDark, toggleTheme } = useTheme()
const toast = useAdminToast()
const { refreshAuth, logout: logoutSession } = useAuth()

function redirectToLogin(error?: string) {
  const params = new URLSearchParams({ returnTo: router.currentRoute.value.fullPath })
  if (error) params.set('error', error)
  window.location.replace(`/login?${params}`)
}

async function verifySession() {
  try {
    const auth = await refreshAuth()
    if (!auth.authenticated || !auth.user?.isAdmin) {
      redirectToLogin()
      return
    }
    user.value = auth.user
  } catch {
    redirectToLogin('session_check_failed')
  }
}

function verifyWhenVisible() {
  if (document.visibilityState === 'visible') void verifySession()
}

function handleLogoutFromAnotherTab(event: StorageEvent) {
  if (event.key === logoutStorageKey) window.location.replace('/login?loggedOut=1')
}

onMounted(() => {
  void verifySession()
  window.addEventListener('focus', verifySession)
  window.addEventListener('pageshow', verifySession)
  window.addEventListener('storage', handleLogoutFromAnotherTab)
  document.addEventListener('visibilitychange', verifyWhenVisible)
})

onBeforeUnmount(() => {
  window.removeEventListener('focus', verifySession)
  window.removeEventListener('pageshow', verifySession)
  window.removeEventListener('storage', handleLogoutFromAnotherTab)
  document.removeEventListener('visibilitychange', verifyWhenVisible)
})

async function logout() {
  if (loggingOut.value) return
  loggingOut.value = true
  try {
    await logoutSession()
    localStorage.setItem(logoutStorageKey, String(Date.now()))
    window.location.replace('/login?loggedOut=1')
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : '退出失败，请重试')
  } finally {
    loggingOut.value = false
  }
}
</script>

<template>
  <div class="admin-layout">
    <AdminToastHost />
    <aside class="admin-sidebar" :class="{ open: menuOpen }">
      <RouterLink class="admin-brand" to="/admin"><img src="/brand-logo.png" alt="" />MyBlog 管理</RouterLink>
      <nav aria-label="管理导航">
        <RouterLink to="/admin">概览</RouterLink>
        <RouterLink to="/admin/posts">文章</RouterLink>
        <RouterLink to="/admin/posts/new">写文章</RouterLink>
        <RouterLink to="/admin/taxonomies">标签与分类</RouterLink>
        <RouterLink to="/admin/media">媒体库</RouterLink>
        <RouterLink to="/admin/site">站点设置</RouterLink>
        <RouterLink to="/admin/users">用户管理</RouterLink>
        <RouterLink v-if="user?.isOwner" to="/admin/administrators">管理员权限</RouterLink>
        <a href="/" target="_blank">查看博客 ↗</a>
      </nav>
      <div class="admin-account">
        <div class="admin-avatar"><img v-if="user?.avatarUrl" :src="user.avatarUrl" :alt="user.name" /><span v-else>{{ user?.name?.slice(0, 1) ?? '管' }}</span></div>
        <div><strong>{{ user?.name ?? '管理员' }}</strong><small>@{{ user?.login ?? 'loading' }}</small></div>
        <button type="button" :disabled="loggingOut" @click="logout">{{ loggingOut ? '退出中' : '退出' }}</button>
      </div>
    </aside>
    <div class="admin-main">
      <header class="admin-topbar">
        <button class="menu-toggle" type="button" :aria-expanded="menuOpen" @click="menuOpen = !menuOpen">{{ menuOpen ? '×' : '≡' }}</button>
        <span>内容管理</span>
        <button type="button" :aria-label="isDark ? '切换到浅色模式' : '切换到深色模式'" @click="toggleTheme">{{ isDark ? '☼' : '☾' }}</button>
      </header>
      <main class="admin-content" @click="menuOpen = false"><slot /></main>
    </div>
  </div>
</template>

<style>
.admin-layout { min-height: 100vh; background: var(--page-bg); }
.admin-sidebar { position: fixed; z-index: 60; inset: 0 auto 0 0; display: flex; flex-direction: column; width: 16rem; padding: 1rem; color: var(--text-main); background: var(--card-bg); border-right: 1px solid var(--line-color); }
.admin-brand { display: flex; align-items: center; gap: 0.65rem; padding: 0.65rem; color: var(--text-strong); font-size: 1.1rem; font-weight: 850; }
.admin-brand img { width: 2rem; height: 2rem; object-fit: contain; }
:root.dark .admin-brand img { filter: brightness(0) invert(1); }
.admin-sidebar nav { display: grid; gap: 0.25rem; margin-top: 1.5rem; }
.admin-sidebar nav a { padding: 0.7rem 0.8rem; border-radius: 0.65rem; font-size: 0.9rem; font-weight: 700; }
.admin-sidebar nav a:hover,
.admin-sidebar nav a.router-link-exact-active { color: var(--primary-strong); background: var(--button-bg); }
.admin-account { display: grid; grid-template-columns: 2.4rem 1fr auto; align-items: center; gap: 0.55rem; margin-top: auto; padding-top: 1rem; border-top: 1px dashed var(--line-color); }
.admin-avatar { display: grid; width: 2.4rem; height: 2.4rem; place-items: center; border-radius: 0.65rem; color: white; background: var(--primary); font-weight: 850; }
.admin-avatar { overflow: hidden; }
.admin-avatar img { width: 100%; height: 100%; object-fit: cover; }
.admin-account strong,
.admin-account small { display: block; max-width: 7rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.admin-account strong { color: var(--text-strong); font-size: 0.82rem; }
.admin-account small { color: var(--text-muted); font-size: 0.7rem; }
.admin-account button,
.admin-topbar button { border: 0; color: var(--text-muted); background: transparent; cursor: pointer; }
.admin-account button:hover,
.admin-topbar button:hover { color: var(--primary-strong); }
.admin-main { min-height: 100vh; margin-left: 16rem; }
.admin-topbar { position: sticky; z-index: 40; top: 0; display: flex; align-items: center; justify-content: space-between; min-height: 4rem; padding: 0 1.4rem; border-bottom: 1px solid var(--line-color); background: var(--float-panel-bg); backdrop-filter: blur(16px); }
.admin-topbar span { color: var(--text-muted); font-size: 0.82rem; font-weight: 750; }
.admin-topbar button { width: 2.5rem; height: 2.5rem; border-radius: 0.65rem; font-size: 1.25rem; }
.menu-toggle { visibility: hidden; }
.admin-content { width: min(calc(100% - 2rem), 76rem); margin: 0 auto; padding: 2rem 0 4rem; }
.admin-page-header { display: flex; align-items: flex-end; justify-content: space-between; gap: 1rem; margin-bottom: 1.4rem; }
.admin-page-header h1 { margin: 0; color: var(--text-strong); font-size: 1.8rem; }
.admin-page-header p { margin: 0.3rem 0 0; color: var(--text-muted); }
.admin-panel { padding: 1.2rem; }
.admin-field { display: grid; gap: 0.4rem; }
.admin-field label { color: var(--text-muted); font-size: 0.8rem; font-weight: 750; }
.admin-input,
.admin-textarea,
.admin-select { width: 100%; border: 1px solid var(--line-color); border-radius: 0.65rem; color: var(--text-main); background: var(--page-bg); outline: none; }
.admin-input,
.admin-select { min-height: 2.7rem; padding: 0.45rem 0.7rem; }
.admin-textarea { min-height: 8rem; padding: 0.7rem; resize: vertical; }
.admin-input:focus,
.admin-textarea:focus,
.admin-select:focus { border-color: var(--primary); box-shadow: 0 0 0 3px oklch(0.72 0.14 var(--hue) / 0.12); }
.admin-actions { display: flex; flex-wrap: wrap; gap: 0.6rem; }
.admin-table-wrap { overflow-x: auto; }
.admin-table { width: 100%; border-collapse: collapse; }
.admin-table th,
.admin-table td { padding: 0.8rem 0.7rem; border-bottom: 1px solid var(--line-color); text-align: left; }
.admin-table th { color: var(--text-muted); font-size: 0.75rem; }
.admin-table td { font-size: 0.88rem; }
.admin-table td strong { color: var(--text-strong); }
.status-badge { display: inline-flex; padding: 0.2rem 0.55rem; border-radius: 99px; color: var(--text-muted); background: var(--button-bg); font-size: 0.72rem; font-weight: 800; }
.status-badge.published { color: oklch(0.52 0.14 150); background: oklch(0.92 0.05 150); }
:root.dark .status-badge.published { color: oklch(0.78 0.13 150); background: oklch(0.28 0.05 150); }
@media (max-width: 820px) {
  .admin-sidebar { transform: translateX(-100%); transition: transform 180ms ease; }
  .admin-sidebar.open { transform: translateX(0); box-shadow: var(--shadow-float); }
  .admin-main { margin-left: 0; }
  .menu-toggle { visibility: visible; }
}
@media (max-width: 560px) {
  .admin-content { width: min(calc(100% - 1rem), 76rem); padding-top: 1rem; }
  .admin-page-header { align-items: flex-start; flex-direction: column; }
}
</style>
