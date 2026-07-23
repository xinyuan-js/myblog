<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import LoadingCard from '@/components/common/LoadingCard.vue'
import PaginationBar from '@/components/blog/PaginationBar.vue'
import PostList from '@/components/blog/PostList.vue'
import { useDocumentMeta } from '@/composables/useDocumentMeta'
import { useSite } from '@/composables/useSite'
import { api } from '@/services/api'
import type { Paginated, PostSummary } from '@/types/blog'
import { isCanonicalPageQuery, parsePageQuery, withPageQuery } from '@/utils/pagination'

const route = useRoute()
const router = useRouter()
const { tags, categories, loadSite, refreshTaxonomies } = useSite()
const result = ref<Paginated<PostSummary> | null>(null)
const posts = computed(() => result.value?.items ?? [])
const loading = ref(true)
const error = ref<string | null>(null)
const page = computed(() => parsePageQuery(route.query.page))
const kind = computed(() => (route.name === 'tag-posts' ? 'tag' : 'category'))
const slug = computed(() => String(route.params.slug))
const item = computed(() => kind.value === 'tag' ? tags.value.find((value) => value.slug === slug.value) : categories.value.find((value) => value.slug === slug.value))
const title = computed(() => item.value?.name ?? (kind.value === 'tag' ? '标签' : '分类'))
const description = computed(() => {
  if (item.value && 'description' in item.value && item.value.description) return item.value.description
  return `这个${kind.value === 'tag' ? '标签' : '分类'}下的文章。`
})
useDocumentMeta(title)
let requestId = 0
let loadedTaxonomy = ''

async function load() {
  const currentRequest = ++requestId
  const taxonomyKey = `${kind.value}:${slug.value}`
  if (loadedTaxonomy !== taxonomyKey) {
    result.value = null
    loadedTaxonomy = taxonomyKey
  }
  if (!isCanonicalPageQuery(route.query.page, page.value)) {
    await router.replace({ query: withPageQuery(route.query, page.value) })
    return
  }
  loading.value = true
  error.value = null
  try {
    await loadSite()
    await refreshTaxonomies()
    if (currentRequest !== requestId) return
    const nextResult = await api.listPosts(kind.value === 'tag'
      ? { tag: slug.value, page: page.value, pageSize: 10 }
      : { category: slug.value, page: page.value, pageSize: 10 })
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

watch([kind, slug, page], load, { immediate: true })
onBeforeUnmount(() => { requestId += 1 })
</script>

<template>
  <header class="section-heading taxonomy-heading card">
    <div><h1>{{ kind === 'tag' ? '#' : '' }}{{ title }}</h1><p>{{ description }}</p></div>
    <span class="pill">{{ item?.postCount ?? posts.length }} 篇</span>
  </header>
  <LoadingCard v-if="loading && !result" />
  <section v-else-if="error && !result" class="error-state card"><h2>加载失败</h2><p>{{ error }}</p><button class="button" type="button" @click="load">重试</button></section>
  <div v-else-if="result" :class="{ refreshing: loading }" :aria-busy="loading">
    <PostList :posts="posts" />
    <PaginationBar :pagination="result.pagination" />
  </div>
  <p v-if="error && result" class="inline-error" role="alert">{{ error }} <button type="button" @click="load">重试</button></p>
</template>

<style scoped>
.taxonomy-heading { padding: 1.15rem 1.25rem; }
.refreshing { opacity: 0.58; pointer-events: none; }
.error-state .button { margin-top: 1rem; }
.inline-error { color: oklch(0.55 0.18 25); text-align: center; }
.inline-error button { border: 0; color: inherit; background: transparent; text-decoration: underline; cursor: pointer; }
@media (max-width: 520px) {
  .taxonomy-heading { align-items: flex-start; padding: 1rem; }
}
</style>
