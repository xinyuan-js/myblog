<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import LoadingCard from '@/components/common/LoadingCard.vue'
import PaginationBar from '@/components/blog/PaginationBar.vue'
import PostList from '@/components/blog/PostList.vue'
import { useDocumentMeta } from '@/composables/useDocumentMeta'
import { api } from '@/services/api'
import type { Paginated, PostSummary } from '@/types/blog'
import { isCanonicalPageQuery, parsePageQuery, withPageQuery } from '@/utils/pagination'

const route = useRoute()
const router = useRouter()
const result = ref<Paginated<PostSummary> | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)
const page = computed(() => parsePageQuery(route.query.page))
let requestId = 0

useDocumentMeta('', '记录工程实践、阅读笔记和生活里值得停留的瞬间。')

async function load() {
  const currentRequest = ++requestId
  if (!isCanonicalPageQuery(route.query.page, page.value)) {
    await router.replace({ query: withPageQuery(route.query, page.value) })
    return
  }
  loading.value = true
  error.value = null
  try {
    const nextResult = await api.listPosts({ page: page.value, pageSize: 8 })
    if (currentRequest !== requestId) return
    const lastPage = Math.max(1, nextResult.pagination.totalPages)
    if (page.value > lastPage) {
      await router.replace({ query: withPageQuery(route.query, lastPage) })
      return
    }
    result.value = nextResult
  } catch (cause) {
    if (currentRequest !== requestId) return
    error.value = cause instanceof Error ? cause.message : '文章加载失败'
  } finally {
    if (currentRequest === requestId) loading.value = false
  }
}

watch(() => route.query.page, load, { immediate: true })
onBeforeUnmount(() => { requestId += 1 })
</script>

<template>
  <LoadingCard v-if="loading && !result" />
  <section v-else-if="error && !result" class="error-state card">
    <h2>没有加载成功</h2>
    <p>{{ error }}</p>
    <button class="button" type="button" @click="load">重新加载</button>
  </section>
  <template v-else-if="result">
    <div class="home-results" :class="{ refreshing: loading }" :aria-busy="loading">
      <PostList :key="result.pagination.page" :posts="result.items" />
      <div v-if="loading" class="page-loading" role="status"><span />正在加载第 {{ page }} 页…</div>
    </div>
    <p v-if="error" class="inline-error" role="alert">{{ error }} <button type="button" @click="load">重试</button></p>
    <PaginationBar :pagination="result.pagination" />
  </template>
</template>

<style scoped>
.home-results { position: relative; transition: opacity 180ms ease, transform 180ms ease; }
.home-results.refreshing { opacity: 0.58; transform: translateY(0.2rem); pointer-events: none; }
.page-loading { position: absolute; z-index: 5; top: 1rem; left: 50%; display: flex; align-items: center; gap: 0.5rem; padding: 0.55rem 0.8rem; border-radius: 999px; color: var(--text-main); background: var(--float-panel-bg); box-shadow: var(--shadow-float); transform: translateX(-50%); font-size: 0.8rem; font-weight: 700; }
.page-loading span { width: 0.8rem; height: 0.8rem; border: 2px solid var(--line-color); border-top-color: var(--primary); border-radius: 50%; animation: loading-spin 650ms linear infinite; }
.inline-error { margin: 0; padding: 0.7rem 0.9rem; border-radius: 0.65rem; color: oklch(0.55 0.18 25); background: oklch(0.94 0.04 25); font-size: 0.85rem; }
.inline-error button { border: 0; color: inherit; background: transparent; text-decoration: underline; cursor: pointer; }
.error-state .button { margin-top: 1rem; }
@keyframes loading-spin { to { transform: rotate(1turn); } }
</style>
