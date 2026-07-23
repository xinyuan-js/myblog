<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ChevronRight, Search } from '@lucide/vue'
import { api } from '@/services/api'
import type { PostSummary } from '@/types/blog'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: []; open: [] }>()
const wrapper = ref<HTMLElement | null>(null)
const desktopInput = ref<HTMLInputElement | null>(null)
const mobileInput = ref<HTMLInputElement | null>(null)
const query = ref('')
const results = ref<PostSummary[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
let timer = 0
let requestId = 0

const keywordLength = computed(() => [...query.value.trim()].length)
const tooShort = computed(() => keywordLength.value > 0 && keywordLength.value < 2)
const hasPanelContent = computed(() => loading.value || Boolean(error.value) || results.value.length > 0)

function close() {
  emit('close')
}

function openSearch() {
  emit('open')
}

function toggleMobile() {
  if (props.open) close()
  else openSearch()
}

function highlightParts(value: string) {
  const keyword = query.value.trim()
  if (!keyword) return [{ text: value, match: false }]
  const lowerValue = value.toLocaleLowerCase()
  const lowerKeyword = keyword.toLocaleLowerCase()
  const parts: Array<{ text: string; match: boolean }> = []
  let offset = 0
  let index = lowerValue.indexOf(lowerKeyword)
  while (index >= 0) {
    if (index > offset) parts.push({ text: value.slice(offset, index), match: false })
    parts.push({ text: value.slice(index, index + keyword.length), match: true })
    offset = index + keyword.length
    index = lowerValue.indexOf(lowerKeyword, offset)
  }
  if (offset < value.length) parts.push({ text: value.slice(offset), match: false })
  return parts.length ? parts : [{ text: value, match: false }]
}

async function search(value: string) {
  const currentRequest = ++requestId
  loading.value = true
  error.value = null
  try {
    const result = await api.listPosts({ q: value, pageSize: 10 })
    if (currentRequest === requestId) results.value = result.items
  } catch (cause) {
    if (currentRequest === requestId) {
      error.value = cause instanceof Error ? cause.message : '搜索失败，请稍后重试'
      results.value = []
    }
  } finally {
    if (currentRequest === requestId) loading.value = false
  }
}

function handleOutsideClick(event: PointerEvent) {
  if (props.open && event.target instanceof Node && !wrapper.value?.contains(event.target)) close()
}

watch(() => props.open, async (open) => {
  if (!open) return
  await nextTick()
  if (window.matchMedia('(min-width: 1024px)').matches) desktopInput.value?.focus()
  else mobileInput.value?.focus()
})

watch(query, (value) => {
  window.clearTimeout(timer)
  const keyword = value.trim()
  if ([...keyword].length < 2) {
    requestId += 1
    results.value = []
    error.value = null
    loading.value = false
    return
  }
  timer = window.setTimeout(() => void search(keyword), 250)
})

onMounted(() => document.addEventListener('pointerdown', handleOutsideClick))
onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', handleOutsideClick)
  window.clearTimeout(timer)
  requestId += 1
})
</script>

<template>
  <div ref="wrapper" class="search-wrap">
    <label class="desktop-search" @focusin="openSearch">
      <Search :size="20" aria-hidden="true" />
      <input
        ref="desktopInput"
        v-model="query"
        type="search"
        maxlength="100"
        autocomplete="off"
        aria-label="搜索关键词"
        placeholder="搜索"
        @keydown.esc="close"
      />
    </label>

    <button class="icon-button mobile-search-button" type="button" aria-label="搜索文章" :aria-expanded="open" @click="toggleMobile">
      <Search :size="20" aria-hidden="true" />
    </button>

    <div class="search-panel card" :class="{ open, 'has-content': hasPanelContent }">
      <label class="mobile-search">
        <Search :size="20" aria-hidden="true" />
        <input
          ref="mobileInput"
          v-model="query"
          type="search"
          maxlength="100"
          autocomplete="off"
          aria-label="搜索关键词"
          placeholder="搜索"
          @keydown.esc="close"
        />
      </label>

      <div v-if="loading" class="search-state"><span />正在搜索…</div>
      <div v-else-if="error" class="search-state error" role="alert">{{ error }}</div>
      <div v-else-if="tooShort" class="search-state mobile-empty">至少输入 2 个字符</div>
      <div v-else-if="query.trim() && !results.length" class="search-state mobile-empty">没有找到相关文章</div>
      <RouterLink
        v-for="post in results"
        :key="post.id"
        class="search-result"
        :to="`/posts/${post.slug}`"
        @click="close"
      >
        <div class="result-title">
          <strong>{{ post.title }}</strong>
          <ChevronRight :size="12" aria-hidden="true" />
        </div>
        <p>
          <template v-for="(part, index) in highlightParts(post.excerpt || '这篇文章暂时没有摘要。')" :key="index">
            <mark v-if="part.match">{{ part.text }}</mark><template v-else>{{ part.text }}</template>
          </template>
        </p>
      </RouterLink>
    </div>
  </div>
</template>

<style scoped>
.search-wrap { position: static; }
.desktop-search {
  position: relative;
  display: none;
  height: 2.75rem;
  align-items: center;
  border-radius: 0.65rem;
  color: var(--text-faint);
  background: rgb(0 0 0 / 0.04);
  transition: background-color 160ms ease;
}
.desktop-search:hover,
.desktop-search:focus-within { background: rgb(0 0 0 / 0.06); }
:global(:root.dark) .desktop-search { background: rgb(255 255 255 / 0.05); }
:global(:root.dark) .desktop-search:hover,
:global(:root.dark) .desktop-search:focus-within { background: rgb(255 255 255 / 0.1); }
.desktop-search svg,
.mobile-search svg { position: absolute; z-index: 1; left: 0.75rem; pointer-events: none; }
.desktop-search input {
  width: 10rem;
  height: 100%;
  padding: 0 0.75rem 0 2.5rem;
  border: 0;
  outline: 0;
  color: var(--text-muted);
  background: transparent;
  font-size: 0.82rem;
  transition: width 180ms ease;
}
.desktop-search input:focus { width: 15rem; }
.desktop-search input::-webkit-search-cancel-button,
.mobile-search input::-webkit-search-cancel-button { opacity: 0.45; }
.mobile-search-button {
  display: grid;
  width: 2.75rem;
  height: 2.75rem;
  padding: 0;
  place-items: center;
  border: 0;
  border-radius: 0.75rem;
  color: var(--text-main);
  background: transparent;
  cursor: pointer;
  transition: color 150ms ease, background-color 150ms ease, transform 150ms ease;
}
.mobile-search-button:hover { color: var(--primary-strong); background: var(--plain-hover); }
.mobile-search-button:active { transform: scale(0.9); }
.search-panel {
  position: absolute;
  z-index: 80;
  top: 4.8rem;
  right: 1rem;
  width: min(30rem, calc(100vw - 2rem));
  max-height: calc(100vh - 6.25rem);
  padding: 0.5rem;
  overflow-y: auto;
  border: 1px solid var(--line-color);
  background: var(--float-panel-bg);
  box-shadow: var(--shadow-float);
  opacity: 0;
  transform: translateY(-0.25rem);
  pointer-events: none;
  transition: opacity 160ms ease, transform 160ms ease;
}
.search-panel.open { opacity: 1; transform: translateY(0); pointer-events: auto; }
.mobile-search {
  position: relative;
  display: flex;
  height: 2.75rem;
  align-items: center;
  margin-bottom: 0.5rem;
  border-radius: 0.7rem;
  color: var(--text-faint);
  background: rgb(0 0 0 / 0.04);
}
:global(:root.dark) .mobile-search { background: rgb(255 255 255 / 0.05); }
.mobile-search input {
  width: 100%;
  height: 100%;
  padding: 0 0.75rem 0 2.5rem;
  border: 0;
  outline: 0;
  color: var(--text-muted);
  background: transparent;
  font-size: 0.82rem;
}
.search-result {
  display: block;
  padding: 0.6rem 0.75rem;
  border-radius: 0.7rem;
  transition: background-color 150ms ease;
}
.search-result:hover { background: var(--plain-hover); }
.result-title { display: inline-flex; align-items: center; gap: 0.2rem; color: var(--text-strong); }
.result-title strong { font-size: 1rem; }
.result-title svg { color: var(--primary); transition: transform 150ms ease; }
.search-result:hover .result-title { color: var(--primary-strong); }
.search-result:hover .result-title svg { transform: translateX(0.15rem); }
.search-result p {
  display: -webkit-box;
  margin: 0.1rem 0 0;
  overflow: hidden;
  color: var(--text-muted);
  font-size: 0.78rem;
  line-height: 1.55;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}
.search-result mark { color: var(--primary-strong); background: transparent; }
.search-state { display: flex; min-height: 4.5rem; align-items: center; justify-content: center; gap: 0.5rem; color: var(--text-muted); font-size: 0.82rem; }
.search-state span { width: 0.85rem; height: 0.85rem; border: 2px solid var(--line-color); border-top-color: var(--primary); border-radius: 50%; animation: search-spin 650ms linear infinite; }
.search-state.error { color: oklch(0.58 0.18 25); }
@keyframes search-spin { to { transform: rotate(1turn); } }

@media (min-width: 1024px) {
  .desktop-search { display: flex; }
  .mobile-search-button,
  .mobile-search { display: none; }
  .search-panel { top: 4.8rem; right: 1rem; }
  .search-panel.open:not(.has-content),
  .search-panel:not(.open) { opacity: 0; transform: translateY(-0.25rem); pointer-events: none; }
  .search-panel.open.has-content { opacity: 1; transform: translateY(0); pointer-events: auto; }
  .mobile-empty { display: none; }
}
</style>
