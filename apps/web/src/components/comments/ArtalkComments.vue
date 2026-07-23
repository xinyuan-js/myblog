<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useTheme } from '@/composables/useTheme'
import { useAuth } from '@/composables/useAuth'
import { api, githubLoginUrl, mockApiEnabled } from '@/services/api'

const props = defineProps<{ pageKey: string; pageTitle: string }>()
const { isDark } = useTheme()
const container = ref<HTMLElement | null>(null)
const loadState = ref<'loading' | 'ready' | 'error'>('loading')
const identityState = ref<'checking' | 'guest' | 'authenticated' | 'error'>('checking')
const identityError = ref<string | null>(null)
const { refreshAuth } = useAuth()
const loginHref = computed(() => githubLoginUrl(`${window.location.pathname}${window.location.search}`))
type ArtalkInstance = {
  setDarkMode: (value: boolean) => void
  destroy: () => void
  on: (name: 'list-fetched' | 'list-failed', handler: () => void) => void
}
type ArtalkModule = {
  init: (config: Record<string, unknown>) => ArtalkInstance
}

declare global {
  interface Window {
    Artalk?: ArtalkModule
  }
}

let instance: ArtalkInstance | null = null
let initSequence = 0
let artalkLoader: Promise<ArtalkModule> | null = null

function loadArtalk(server: string): Promise<ArtalkModule> {
  if (window.Artalk?.init) return Promise.resolve(window.Artalk)
  if (artalkLoader) return artalkLoader

  const baseURL = server.replace(/\/+$/, '')
  if (!document.querySelector('link[data-artalk-style]')) {
    const style = document.createElement('link')
    style.rel = 'stylesheet'
    style.href = `${baseURL}/dist/Artalk.css`
    style.dataset.artalkStyle = ''
    document.head.append(style)
  }

  const loader = new Promise<ArtalkModule>((resolve, reject) => {
    const script = document.createElement('script')
    script.src = `${baseURL}/dist/Artalk.js`
    script.dataset.artalkScript = ''
    script.onload = () => {
      if (window.Artalk?.init) resolve(window.Artalk)
      else reject(new Error('Artalk 脚本未提供 init 方法'))
    }
    script.onerror = () => reject(new Error('Artalk 脚本加载失败'))
    document.head.append(script)
  }).catch((error) => {
    artalkLoader = null
    document.querySelector('script[data-artalk-script]')?.remove()
    throw error
  })

  artalkLoader = loader
  return loader
}

function destroyArtalk() {
  instance?.destroy()
  instance = null
}

async function synchronizeIdentity() {
  identityState.value = 'checking'
  identityError.value = null
  localStorage.removeItem('ArtalkUser')
  try {
    const auth = await refreshAuth()
    if (!auth.authenticated) {
      identityState.value = 'guest'
      return
    }
    const session = await api.createArtalkSession()
    localStorage.setItem('ArtalkUser', JSON.stringify({ ...session.user, token: session.token }))
    identityState.value = 'authenticated'
  } catch (error) {
    console.error('评论身份同步失败', error)
    identityState.value = 'error'
    identityError.value = error instanceof Error ? error.message : '登录状态同步失败，评论暂时不可发表。'
  }
}

async function initArtalk() {
  if (mockApiEnabled) return
  const sequence = ++initSequence
  destroyArtalk()
  loadState.value = 'loading'

  try {
    await synchronizeIdentity()
    if (sequence !== initSequence || !container.value) return
    const server = import.meta.env.VITE_ARTALK_SERVER || '/comments'
    const Artalk = await loadArtalk(server)
    if (sequence !== initSequence || !container.value) return

    const nextInstance = Artalk.init({
      el: container.value,
      pageKey: props.pageKey,
      pageTitle: props.pageTitle,
      server,
      site: import.meta.env.VITE_ARTALK_SITE || 'MyBlog',
      darkMode: isDark.value,
      locale: 'zh-CN',
    })
    instance = nextInstance
    nextInstance.on('list-fetched', () => {
      if (sequence === initSequence) loadState.value = 'ready'
    })
    nextInstance.on('list-failed', () => {
      if (sequence === initSequence) loadState.value = 'error'
    })
  } catch (error) {
    if (sequence !== initSequence) return
    console.error('评论区加载失败', error)
    loadState.value = 'error'
  }
}

function handleAuthChange() {
  void initArtalk()
}

watch(isDark, (value) => instance?.setDarkMode(value))
watch(
  () => [props.pageKey, props.pageTitle],
  () => void initArtalk(),
  { flush: 'post' },
)

onMounted(() => {
  void initArtalk()
  window.addEventListener('myblog:auth-changed', handleAuthChange)
})
onBeforeUnmount(() => {
  initSequence += 1
  destroyArtalk()
  window.removeEventListener('myblog:auth-changed', handleAuthChange)
})
</script>

<template>
  <section class="comments-card card" aria-labelledby="comments-title">
    <h2 id="comments-title">评论</h2>
    <div v-if="mockApiEnabled" class="comments-preview">
      <strong>Artalk 评论区</strong>
      <p>当前使用模拟数据。连接部署后的 Artalk 服务后，这里将显示评论编辑器、回复和审核后的评论列表。</p>
    </div>
    <template v-else>
      <div v-if="identityState === 'guest'" class="comments-login">
        <div>
          <strong>使用 GitHub 参与评论</strong>
          <p>登录本站后即可发表评论，无需再次登录评论系统。</p>
        </div>
        <a class="button primary" :href="loginHref">使用 GitHub 登录</a>
      </div>
      <p v-else-if="identityError" class="comments-identity-error" role="alert">{{ identityError }}</p>
      <p v-if="loadState === 'loading'" class="comments-status" role="status">评论加载中…</p>
      <div v-if="loadState === 'error'" class="comments-error" role="alert">
        <p>评论区加载失败，请检查网络后重试。</p>
        <button type="button" @click="initArtalk">重新加载评论</button>
      </div>
      <div
        ref="container"
        class="artalk-comments"
        :class="{ 'editor-locked': identityState !== 'authenticated' }"
        :aria-busy="loadState === 'loading'"
      />
    </template>
  </section>
</template>

<style scoped>
.comments-card { padding: clamp(1.1rem, 3vw, 2rem); }
.comments-card h2 { margin: 0 0 1rem; color: var(--text-strong); font-size: 1.25rem; }
.comments-preview { padding: 1.4rem; border: 1px dashed var(--line-color); border-radius: 0.8rem; background: var(--card-muted); }
.comments-preview strong { color: var(--text-strong); }
.comments-preview p { margin: 0.35rem 0 0; color: var(--text-muted); }
.comments-login { display: flex; align-items: center; justify-content: space-between; gap: 1rem; margin-bottom: 1rem; padding: 1rem; border: 1px solid var(--line-color); border-radius: 0.8rem; background: var(--card-muted); }
.comments-login strong { color: var(--text-strong); }
.comments-login p { margin: 0.25rem 0 0; color: var(--text-muted); font-size: 0.875rem; }
.comments-login a { flex: none; }
.comments-identity-error { margin: 0 0 1rem; padding: 0.8rem; border-radius: 0.7rem; color: #b42318; background: color-mix(in srgb, #ef4444 9%, var(--card-bg)); }
.comments-status { margin: 0; color: var(--text-muted); }
.comments-error { padding: 1rem; border: 1px solid color-mix(in srgb, #ef4444 36%, var(--line-color)); border-radius: 0.8rem; background: color-mix(in srgb, #ef4444 8%, var(--card-bg)); color: var(--text-muted); }
.comments-error p { margin: 0 0 0.75rem; }
.comments-error button { padding: 0.55rem 0.9rem; border: 1px solid var(--line-color); border-radius: 0.65rem; background: var(--card-bg); color: var(--text-strong); cursor: pointer; }
.comments-error button:hover { border-color: var(--primary); color: var(--primary); }
.editor-locked :deep(.atk-main-editor) { display: none; }
@media (max-width: 560px) {
  .comments-login { align-items: stretch; flex-direction: column; }
  .comments-login a { width: 100%; }
}
</style>
