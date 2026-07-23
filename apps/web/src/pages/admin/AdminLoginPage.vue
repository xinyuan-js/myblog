<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { ArrowLeft, GitFork, ShieldCheck } from '@lucide/vue'
import { useDocumentMeta } from '@/composables/useDocumentMeta'
import { api, githubLoginUrl, mockApiEnabled, sanitizeReturnTo } from '@/services/api'

useDocumentMeta('登录')

const route = useRoute()
const router = useRouter()
const loggingIn = ref(false)
const localError = ref<string | null>(null)

const returnTo = computed(() => sanitizeReturnTo(route.query.returnTo))
const loginHref = computed(() => githubLoginUrl(returnTo.value))
const errorMessage = computed(() => {
  if (localError.value) return localError.value
  const code = typeof route.query.error === 'string' ? route.query.error : ''
  return {
    access_denied: '你取消了 GitHub 授权，请重新登录。',
    state_invalid: '登录请求已失效，请重新发起登录。',
    oauth_failed: 'GitHub 登录暂时没有完成，请稍后重试。',
    session_check_failed: '暂时无法确认登录状态，请检查后端服务后重试。',
  }[code] ?? null
})

async function enterMockSession() {
  loggingIn.value = true
  localError.value = null
  try {
    if (!api.createMockSession) throw new Error('模拟登录不可用')
    await api.createMockSession()
    await router.replace(returnTo.value)
  } catch (cause) {
    localError.value = cause instanceof Error ? cause.message : '登录失败'
  } finally {
    loggingIn.value = false
  }
}
</script>

<template>
  <main class="login-page">
    <RouterLink class="back-home" to="/"><ArrowLeft :size="18" />返回博客</RouterLink>
    <section class="login-card card">
      <RouterLink class="login-brand" to="/"><img src="/brand-logo.png" alt="" />MyBlog</RouterLink>
      <div class="login-icon"><ShieldCheck :size="34" aria-hidden="true" /></div>
      <h1>登录 MyBlog</h1>
      <p class="description">任何 GitHub 用户都可以登录并参与评论，管理员账号会自动获得内容管理权限。</p>

      <p v-if="errorMessage" class="login-message error" role="alert">{{ errorMessage }}</p>
      <p v-else-if="route.query.loggedOut === '1'" class="login-message success">已经安全退出。</p>

      <button v-if="mockApiEnabled" class="button primary login-button" type="button" :disabled="loggingIn" :aria-busy="loggingIn" @click="enterMockSession">
        <GitFork :size="20" aria-hidden="true" />{{ loggingIn ? '正在进入…' : '进入模拟账号' }}
      </button>
      <a v-else class="button primary login-button" :href="loginHref">
        <GitFork :size="20" aria-hidden="true" />使用 GitHub 登录
      </a>

      <div class="security-note">
        <strong>一个账号，一次登录</strong>
        <span>评论和管理共用本站会话；管理入口只对站点管理员显示。退出本站不会退出 GitHub。</span>
      </div>
    </section>
  </main>
</template>

<style scoped>
.login-page { position: relative; display: grid; min-height: 100vh; place-items: center; padding: 5rem 1rem 2rem; background: var(--page-bg); }
.back-home { position: absolute; top: 1.5rem; left: 1.5rem; display: flex; height: 2.5rem; align-items: center; gap: 0.4rem; padding: 0 0.75rem; border-radius: 0.6rem; color: var(--text-muted); font-size: 0.875rem; font-weight: 600; }
.back-home:hover { color: var(--primary); background: var(--plain-hover); }
.login-card { width: min(100%, 28rem); padding: 2rem; }
.login-brand { display: inline-flex; align-items: center; gap: 0.65rem; color: var(--text-strong); font-weight: 800; }
.login-brand img { width: 2rem; height: 2rem; object-fit: contain; }
:global(:root.dark) .login-brand img { filter: brightness(0) invert(1); }
.login-icon { display: grid; width: 3.25rem; height: 3.25rem; margin-top: 2.5rem; place-items: center; border-radius: 0.8rem; color: var(--primary); background: var(--button-bg); }
.login-card h1 { margin: 1.25rem 0 0; color: var(--text-strong); font-size: 2rem; }
.description { margin: 0.75rem 0 0; color: var(--text-muted); }
.login-message { margin: 1rem 0 0; padding: 0.7rem 0.8rem; border-radius: 0.6rem; font-size: 0.875rem; }
.login-message.error { color: oklch(0.55 0.18 25); background: oklch(0.94 0.04 25); }
.login-message.success { color: oklch(0.48 0.13 150); background: oklch(0.94 0.04 150); }
.login-button { width: 100%; margin-top: 1.25rem; gap: 0.55rem; }
.security-note { display: grid; gap: 0.25rem; margin-top: 1rem; padding-top: 1rem; border-top: 1px dashed var(--line-color); }
.security-note strong { color: var(--text-main); font-size: 0.8rem; }
.security-note span { color: var(--text-faint); font-size: 0.75rem; line-height: 1.6; }
:global(:root.dark) .login-message.error { color: oklch(0.78 0.14 25); background: oklch(0.28 0.05 25); }
:global(:root.dark) .login-message.success { color: oklch(0.78 0.12 150); background: oklch(0.28 0.04 150); }
@media (max-width: 520px) { .login-card { padding: 1.5rem; } .back-home { top: 1rem; left: 0.75rem; } }
</style>
