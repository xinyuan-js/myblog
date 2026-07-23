<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { Contrast, LogOut, Menu, Moon, Palette, RotateCcw, Settings, Sun, X } from '@lucide/vue'
import { useTheme } from '@/composables/useTheme'
import { useAuth } from '@/composables/useAuth'
import SearchDialog from './SearchDialog.vue'

defineProps<{ title: string }>()

const menuOpen = ref(false)
const paletteOpen = ref(false)
const themeMenuOpen = ref(false)
const accountMenuOpen = ref(false)
const searchOpen = ref(false)
const loggingOut = ref(false)
const { authState, refreshAuth, logout } = useAuth()
const { scheme, hue, toggleTheme, setTheme, setHue } = useTheme()
const defaultHue = 250

const links = computed(() => {
  const items = [
    { label: '首页', to: '/' },
    { label: '归档', to: '/archive' },
    { label: '标签', to: '/tags' },
    { label: '分类', to: '/categories' },
    { label: '关于', to: '/about' },
  ]
  if (authState.value !== null) {
    if (!authState.value.authenticated) items.push({ label: '登录', to: '/login' })
  }
  return items
})

async function refreshAuthState() {
  try {
    await refreshAuth()
  } catch {
    // 导航仍可正常使用，稍后在聚焦页面时重试。
  }
}

async function logoutCurrentUser() {
  if (loggingOut.value) return
  loggingOut.value = true
  try {
    await logout()
    menuOpen.value = false
    accountMenuOpen.value = false
  } finally {
    loggingOut.value = false
  }
}

function refreshAuthWhenVisible() {
  if (document.visibilityState === 'visible') void refreshAuthState()
}

function handleSearchShortcut(event: KeyboardEvent) {
  const target = event.target as HTMLElement | null
  const isTyping = target?.matches('input, textarea, select, [contenteditable="true"]')
  if ((event.key === '/' && !isTyping) || ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k')) {
    event.preventDefault()
    searchOpen.value = true
  } else if (event.key === 'Escape') {
    searchOpen.value = false
  }
}

onMounted(() => {
  void refreshAuthState()
  window.addEventListener('focus', refreshAuthState)
  window.addEventListener('keydown', handleSearchShortcut)
  document.addEventListener('visibilitychange', refreshAuthWhenVisible)
})

onBeforeUnmount(() => {
  window.removeEventListener('focus', refreshAuthState)
  window.removeEventListener('keydown', handleSearchShortcut)
  document.removeEventListener('visibilitychange', refreshAuthWhenVisible)
})
</script>

<template>
  <header class="nav-shell page-shell">
    <nav class="navbar card" aria-label="主导航">
      <RouterLink class="brand-button" to="/" @click="menuOpen = false">
        <img src="/brand-logo.png" alt="" />
        <span class="sr-only">{{ title }}</span>
      </RouterLink>

      <div class="desktop-links">
        <RouterLink v-for="link in links" :key="link.to" :to="link.to">{{ link.label }}</RouterLink>
        <div v-if="authState?.authenticated && authState.user" class="account-wrap">
          <button
            class="account-avatar"
            type="button"
            :aria-expanded="accountMenuOpen"
            aria-label="打开账号菜单"
            @click="accountMenuOpen = !accountMenuOpen"
          >
            <img :src="authState.user.avatarUrl" :alt="`${authState.user.name} 的 GitHub 头像`" />
          </button>
          <div v-if="accountMenuOpen" class="account-menu card">
            <div class="account-summary">
              <strong>{{ authState.user.name }}</strong>
              <span>@{{ authState.user.login }}</span>
            </div>
            <RouterLink v-if="authState.user.isAdmin" to="/admin" @click="accountMenuOpen = false">
              <Settings :size="17" aria-hidden="true" />进入管理
            </RouterLink>
            <button type="button" :disabled="loggingOut" @click="logoutCurrentUser">
              <LogOut :size="17" aria-hidden="true" />{{ loggingOut ? '退出中…' : '退出登录' }}
            </button>
          </div>
        </div>
      </div>

      <div class="nav-actions">
        <SearchDialog :open="searchOpen" @open="searchOpen = true" @close="searchOpen = false" />
        <div class="palette-wrap">
          <button class="icon-button" type="button" aria-label="调整主题色" @click="paletteOpen = !paletteOpen">
            <Palette :size="20" aria-hidden="true" />
          </button>
          <div v-if="paletteOpen" class="palette-popover card">
            <div class="palette-title"><label for="hue-range">主题色</label><button v-if="hue !== defaultHue" type="button" aria-label="恢复默认主题色" @click="setHue(defaultHue)"><RotateCcw :size="14" /></button><span>{{ hue }}</span></div>
            <input
              id="hue-range"
              :value="hue"
              type="range"
              min="0"
              max="360"
              step="5"
              @input="setHue(Number(($event.target as HTMLInputElement).value))"
            />
          </div>
        </div>
        <div class="theme-wrap" @mouseenter="themeMenuOpen = true" @mouseleave="themeMenuOpen = false">
          <button class="icon-button" type="button" aria-label="切换浅色、深色或跟随系统" @click="toggleTheme">
            <Sun v-if="scheme === 'light'" :size="20" aria-hidden="true" />
            <Moon v-else-if="scheme === 'dark'" :size="20" aria-hidden="true" />
            <Contrast v-else :size="20" aria-hidden="true" />
          </button>
          <div v-if="themeMenuOpen" class="theme-menu card" role="menu">
            <button type="button" :class="{ active: scheme === 'light' }" @click="setTheme('light')"><Sun :size="20" /><span>浅色模式</span></button>
            <button type="button" :class="{ active: scheme === 'dark' }" @click="setTheme('dark')"><Moon :size="20" /><span>深色模式</span></button>
            <button type="button" :class="{ active: scheme === 'auto' }" @click="setTheme('auto')"><Contrast :size="20" /><span>跟随系统</span></button>
          </div>
        </div>
        <button class="icon-button mobile-menu-button" type="button" aria-label="打开导航菜单" :aria-expanded="menuOpen" @click="menuOpen = !menuOpen">
          <X v-if="menuOpen" :size="22" aria-hidden="true" /><Menu v-else :size="22" aria-hidden="true" />
        </button>
      </div>

      <div v-if="menuOpen" class="mobile-links card">
        <RouterLink v-for="link in links" :key="link.to" :to="link.to" @click="menuOpen = false">{{ link.label }}</RouterLink>
        <div v-if="authState?.authenticated && authState.user" class="mobile-account">
          <img :src="authState.user.avatarUrl" alt="" />
          <span><strong>{{ authState.user.name }}</strong><small>@{{ authState.user.login }}</small></span>
        </div>
        <RouterLink v-if="authState?.user?.isAdmin" to="/admin" @click="menuOpen = false">
          <Settings :size="18" aria-hidden="true" />进入管理
        </RouterLink>
        <button v-if="authState?.authenticated" type="button" :disabled="loggingOut" @click="logoutCurrentUser">
          <LogOut :size="18" aria-hidden="true" />{{ loggingOut ? '退出中…' : '退出登录' }}
        </button>
      </div>
    </nav>
  </header>
</template>

<style scoped>
.nav-shell {
  position: relative;
  animation: navbar-enter 300ms both;
}

.navbar {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 4.5rem;
  height: 4.5rem;
  padding: 0 1rem;
  border-top-left-radius: 0;
  border-top-right-radius: 0;
  border: 1px solid rgb(255 255 255 / 0.28);
  border-top: 0;
  background: rgb(255 255 255 / 0.72);
  box-shadow: 0 8px 30px rgb(20 25 45 / 0.08);
  backdrop-filter: blur(18px) saturate(1.25);
  -webkit-backdrop-filter: blur(18px) saturate(1.25);
  overflow: visible;
}

:global(:root.dark) .navbar {
  border-color: rgb(255 255 255 / 0.08);
  border-top: 0;
  background: oklch(0.18 0.015 var(--hue) / 0.72);
  box-shadow: 0 8px 30px rgb(0 0 0 / 0.2);
}

.brand-button,
.desktop-links a,
.icon-button {
  border-radius: 0.75rem;
  transition: color 150ms ease, background-color 150ms ease, transform 150ms ease;
}

.brand-button {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  height: 3.25rem;
  padding: 0 1.25rem;
  color: var(--primary-strong);
  font-weight: 800;
}

.brand-button img { width: 2.65rem; height: 2.65rem; object-fit: contain; }
:global(:root.dark) .brand-button img { filter: brightness(0) invert(1); }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; overflow: hidden; clip: rect(0, 0, 0, 0); white-space: nowrap; border: 0; }

.desktop-links {
  display: flex;
  align-items: center;
}

.desktop-links a {
  display: flex;
  height: 2.75rem;
  align-items: center;
  padding: 0 1.25rem;
  font-size: 1rem;
  font-weight: 700;
}

.desktop-links a:hover,
.brand-button:hover,
.icon-button:hover {
  color: var(--primary-strong);
  background: var(--plain-hover);
}

.account-wrap { position: relative; display: grid; place-items: center; margin-left: 0.35rem; }
.account-avatar { width: 2.5rem; height: 2.5rem; padding: 0; overflow: hidden; border: 2px solid color-mix(in srgb, var(--primary) 35%, var(--line-color)); border-radius: 50%; background: var(--button-bg); cursor: pointer; }
.account-avatar:hover { border-color: var(--primary); transform: translateY(-1px); }
.account-avatar img { width: 100%; height: 100%; object-fit: cover; }
.account-menu { position: absolute; z-index: 70; top: 3.2rem; right: 0; display: grid; width: 13rem; padding: 0.5rem; background: var(--float-panel-bg); box-shadow: var(--shadow-float); }
.account-menu .account-summary { display: grid; padding: 0.55rem 0.7rem 0.75rem; border-bottom: 1px solid var(--line-color); }
.account-summary strong { overflow: hidden; color: var(--text-strong); text-overflow: ellipsis; white-space: nowrap; }
.account-summary span { overflow: hidden; color: var(--text-muted); text-overflow: ellipsis; white-space: nowrap; font-size: 0.78rem; }
.account-menu a,
.account-menu button { display: flex; min-height: 2.5rem; align-items: center; gap: 0.55rem; margin-top: 0.25rem; padding: 0 0.7rem; border: 0; border-radius: 0.55rem; color: var(--text-main); background: transparent; font: inherit; font-size: 0.88rem; font-weight: 700; cursor: pointer; }
.account-menu a:hover,
.account-menu button:hover { color: var(--primary-strong); background: var(--plain-hover); }

.nav-actions {
  display: flex;
  align-items: center;
  gap: 0.15rem;
}

.icon-button {
  display: grid;
  width: 2.75rem;
  height: 2.75rem;
  padding: 0;
  place-items: center;
  border: 0;
  color: var(--text-main);
  background: transparent;
  cursor: pointer;
}

.mobile-menu-button { display: none; }
.palette-wrap { position: relative; }
.theme-wrap { position: relative; z-index: 50; }
.theme-menu { position: absolute; top: 2.75rem; right: -0.5rem; display: grid; width: 9.5rem; padding: 1.75rem 0.5rem 0.5rem; background: var(--float-panel-bg); box-shadow: var(--shadow-float); }
.theme-menu button { display: flex; height: 2.25rem; align-items: center; gap: 0.75rem; padding: 0 0.75rem; border: 0; border-radius: 0.5rem; color: var(--text-main); background: transparent; font-weight: 500; cursor: pointer; }
.theme-menu button:hover,
.theme-menu button.active { color: var(--primary); background: var(--plain-hover); }

.palette-popover {
  position: absolute;
  top: 3.4rem;
  right: 0;
  width: 20rem;
  padding: 1rem;
  box-shadow: var(--shadow-float);
}

.palette-title { display: flex; align-items: center; gap: 0.45rem; margin-bottom: 0.7rem; }
.palette-title label { color: var(--text-strong); font-size: 1rem; font-weight: 800; }
.palette-title button { display: grid; width: 1.75rem; height: 1.75rem; padding: 0; place-items: center; border: 0; border-radius: 0.4rem; color: var(--primary-strong); background: var(--button-bg); cursor: pointer; }
.palette-title span { min-width: 2.5rem; margin-left: auto; padding: 0.2rem 0.45rem; border-radius: 0.4rem; color: var(--primary-strong); background: var(--button-bg); text-align: center; font-size: 0.78rem; font-weight: 800; }

.palette-popover input { width: 100%; height: 1.5rem; margin: 0; appearance: none; border-radius: 0.25rem; background: linear-gradient(to right, oklch(0.8 0.1 0), oklch(0.8 0.1 30), oklch(0.8 0.1 60), oklch(0.8 0.1 90), oklch(0.8 0.1 120), oklch(0.8 0.1 150), oklch(0.8 0.1 180), oklch(0.8 0.1 210), oklch(0.8 0.1 240), oklch(0.8 0.1 270), oklch(0.8 0.1 300), oklch(0.8 0.1 330), oklch(0.8 0.1 360)); }
.palette-popover input::-webkit-slider-thumb { width: 0.5rem; height: 1rem; appearance: none; border: 0; border-radius: 0.125rem; background: rgb(255 255 255 / 0.7); }
.palette-popover input::-moz-range-thumb { width: 0.5rem; height: 1rem; border: 0; border-radius: 0.125rem; background: rgb(255 255 255 / 0.7); }
:global(:root.dark) .palette-popover input { background: linear-gradient(to right, oklch(0.7 0.1 0), oklch(0.7 0.1 30), oklch(0.7 0.1 60), oklch(0.7 0.1 90), oklch(0.7 0.1 120), oklch(0.7 0.1 150), oklch(0.7 0.1 180), oklch(0.7 0.1 210), oklch(0.7 0.1 240), oklch(0.7 0.1 270), oklch(0.7 0.1 300), oklch(0.7 0.1 330), oklch(0.7 0.1 360)); }

.mobile-links {
  position: absolute;
  top: 4.8rem;
  right: 0;
  display: grid;
  width: min(16rem, calc(100vw - 1rem));
  padding: 0.5rem;
  box-shadow: var(--shadow-float);
}

.mobile-links a,
.mobile-links button {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  padding: 0.75rem 0.9rem;
  border: 0;
  border-radius: 0.65rem;
  color: var(--text-main);
  background: transparent;
  font-weight: 700;
  cursor: pointer;
}

.mobile-links a:hover,
.mobile-links button:hover { color: var(--primary-strong); background: var(--plain-hover); }
.mobile-account { display: grid; grid-template-columns: 2.4rem 1fr; align-items: center; gap: 0.65rem; margin: 0.25rem 0; padding: 0.75rem 0.9rem; border-top: 1px solid var(--line-color); border-bottom: 1px solid var(--line-color); }
.mobile-account img { width: 2.4rem; height: 2.4rem; border-radius: 50%; object-fit: cover; }
.mobile-account span,
.mobile-account strong,
.mobile-account small { display: block; min-width: 0; }
.mobile-account strong,
.mobile-account small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mobile-account strong { color: var(--text-strong); }
.mobile-account small { color: var(--text-muted); font-size: 0.75rem; }

@media (max-width: 1023px) {
  .theme-menu { display: none; }
}

@media (max-width: 767px) {
  .desktop-links { display: none; }
  .mobile-menu-button { display: grid; }
}

@media (max-width: 640px) {
  .navbar { height: 4.5rem; padding: 0 0.5rem; }
  .brand-button { height: 3.25rem; padding: 0 1rem; }
  .nav-shell { width: 100%; }
  .palette-popover { right: -5.5rem; width: min(20rem, calc(100vw - 1rem)); }
}

@keyframes navbar-enter { from { opacity: 0; transform: translateY(2rem); } to { opacity: 1; transform: translateY(0); } }
</style>
