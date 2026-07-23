<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import LoadingCard from '@/components/common/LoadingCard.vue'
import PostList from '@/components/blog/PostList.vue'
import { useDocumentMeta } from '@/composables/useDocumentMeta'
import { useSite } from '@/composables/useSite'
import { api } from '@/services/api'
import type { PostSummary } from '@/types/blog'

const route = useRoute()
const { tags, categories, loadSite, refreshTaxonomies } = useSite()
const posts = ref<PostSummary[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const kind = computed(() => (route.name === 'tag-posts' ? 'tag' : 'category'))
const slug = computed(() => String(route.params.slug))
const item = computed(() => kind.value === 'tag' ? tags.value.find((value) => value.slug === slug.value) : categories.value.find((value) => value.slug === slug.value))
const title = computed(() => item.value?.name ?? (kind.value === 'tag' ? '标签' : '分类'))
const description = computed(() => {
  if (item.value && 'description' in item.value && item.value.description) return item.value.description
  return `这个${kind.value === 'tag' ? '标签' : '分类'}下的文章。`
})
useDocumentMeta(title)

async function load() {
  loading.value = true
  error.value = null
  await loadSite()
  await refreshTaxonomies()
  try {
    const result = await api.listPosts(kind.value === 'tag' ? { tag: slug.value, pageSize: 50 } : { category: slug.value, pageSize: 50 })
    posts.value = result.items
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '文章加载失败'
  } finally {
    loading.value = false
  }
}

watch([kind, slug], load, { immediate: true })
</script>

<template>
  <header class="section-heading taxonomy-heading card">
    <div><h1>{{ kind === 'tag' ? '#' : '' }}{{ title }}</h1><p>{{ description }}</p></div>
    <span class="pill">{{ item?.postCount ?? posts.length }} 篇</span>
  </header>
  <LoadingCard v-if="loading" />
  <section v-else-if="error" class="error-state card"><h2>加载失败</h2><p>{{ error }}</p></section>
  <PostList v-else :posts="posts" />
</template>

<style scoped>
.taxonomy-heading { padding: 1.15rem 1.25rem; }
@media (max-width: 520px) {
  .taxonomy-heading { align-items: flex-start; padding: 1rem; }
}
</style>
