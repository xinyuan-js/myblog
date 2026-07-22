<script setup lang="ts">
import { onBeforeUnmount, onMounted, watch } from 'vue'
import { useTheme } from '@/composables/useTheme'

const props = defineProps<{ pageKey: string; pageTitle: string }>()
const { isDark } = useTheme()
const mockMode = import.meta.env.VITE_USE_MOCK_API !== 'false'
let instance: { setDarkMode: (value: boolean) => void; destroy: () => void } | null = null

onMounted(async () => {
  if (mockMode) return
  const [{ default: Artalk }] = await Promise.all([
    import('artalk'),
    import('artalk/dist/Artalk.css'),
  ])
  instance = Artalk.init({
    el: '#artalk-comments',
    pageKey: props.pageKey,
    pageTitle: props.pageTitle,
    server: import.meta.env.VITE_ARTALK_SERVER || '/comments',
    site: import.meta.env.VITE_ARTALK_SITE || '浮光',
    darkMode: isDark.value,
    locale: 'zh-CN',
  })
})

watch(isDark, (value) => instance?.setDarkMode(value))
onBeforeUnmount(() => instance?.destroy())
</script>

<template>
  <section class="comments-card card" aria-labelledby="comments-title">
    <h2 id="comments-title">评论</h2>
    <div v-if="mockMode" class="comments-preview">
      <strong>Artalk 评论区</strong>
      <p>当前使用模拟数据。连接部署后的 Artalk 服务后，这里将显示评论编辑器、回复和审核后的评论列表。</p>
    </div>
    <div v-else id="artalk-comments" />
  </section>
</template>

<style scoped>
.comments-card { padding: clamp(1.1rem, 3vw, 2rem); }
.comments-card h2 { margin: 0 0 1rem; color: var(--text-strong); font-size: 1.25rem; }
.comments-preview { padding: 1.4rem; border: 1px dashed var(--line-color); border-radius: 0.8rem; background: var(--card-muted); }
.comments-preview strong { color: var(--text-strong); }
.comments-preview p { margin: 0.35rem 0 0; color: var(--text-muted); }
</style>
