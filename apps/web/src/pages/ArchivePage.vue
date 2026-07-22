<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import LoadingCard from '@/components/common/LoadingCard.vue'
import { useDocumentMeta } from '@/composables/useDocumentMeta'
import { api } from '@/services/api'
import type { PostSummary } from '@/types/blog'

const posts = ref<PostSummary[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
useDocumentMeta('归档')

const groups = computed(() => {
  const result = new Map<number, PostSummary[]>()
  posts.value.forEach((post) => {
    if (!post.publishedAt) return
    const year = new Date(post.publishedAt).getFullYear()
    result.set(year, [...(result.get(year) ?? []), post])
  })
  return [...result.entries()].sort(([a], [b]) => b - a)
})

function formatArchiveDate(value: string) {
  const date = new Date(value)
  return `${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

onMounted(async () => {
  try {
    posts.value = (await api.listPosts({ pageSize: 100 })).items
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '归档加载失败'
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <LoadingCard v-if="loading" />
  <section v-else-if="error" class="error-state card"><h2>加载失败</h2><p>{{ error }}</p></section>
  <section v-else class="archive card">
    <div v-for="[year, yearPosts] in groups" :key="year" class="archive-year">
      <div class="year-row"><strong>{{ year }}</strong><i /><span>{{ yearPosts.length }} 篇文章</span></div>
      <div class="timeline">
        <RouterLink v-for="post in yearPosts" :key="post.id" :to="`/posts/${post.slug}`">
          <time :datetime="post.publishedAt ?? undefined">{{ formatArchiveDate(post.publishedAt!) }}</time>
          <i /><strong>{{ post.title }}</strong><small>{{ post.tags.map((tag) => `#${tag.name}`).join(' ') }}</small>
        </RouterLink>
      </div>
    </div>
  </section>
</template>

<style scoped>
.archive { padding: 1.5rem 2rem; }
.archive-year + .archive-year { margin-top: 0.5rem; }
.year-row { display: grid; grid-template-columns: 10% 10% 80%; min-height: 3.75rem; align-items: center; }
.year-row strong { color: var(--text-main); text-align: right; font-size: 1.45rem; }
.year-row i { width: 0.75rem; height: 0.75rem; margin: auto; border: 3px solid var(--primary); border-radius: 50%; }
.year-row span { color: var(--text-muted); }
.timeline { display: grid; }
.timeline a { display: grid; grid-template-columns: 10% 10% minmax(0, 65%) 15%; min-height: 2.5rem; align-items: center; border-radius: 0.5rem; }
.timeline a:hover { color: var(--primary-strong); background: var(--button-bg); }
.timeline time { color: var(--text-muted); text-align: right; font-size: 0.8rem; }
.timeline i { position: relative; z-index: 1; width: 0.25rem; height: 0.25rem; margin: auto; border-radius: 50%; background: oklch(0.5 0.05 var(--hue)); box-shadow: 0 0 0 4px var(--card-bg); }
.timeline i::before { content: ''; position: absolute; z-index: -1; top: -1.2rem; left: calc(50% - 1px); height: 2.5rem; border-left: 2px dashed var(--line-color); }
.timeline a:hover i { height: 1.25rem; border-radius: 999px; background: var(--primary); box-shadow: 0 0 0 4px var(--plain-hover); }
.timeline strong { overflow: hidden; padding: 0 0.75rem; color: var(--text-main); text-overflow: ellipsis; white-space: nowrap; }
.timeline a:hover strong { color: var(--primary-strong); transform: translateX(0.2rem); }
.timeline small { overflow: hidden; color: var(--text-faint); text-overflow: ellipsis; white-space: nowrap; }
@media (max-width: 767px) {
  .year-row { grid-template-columns: 15% 15% 70%; }
  .timeline a { grid-template-columns: 15% 15% 70%; }
  .timeline small { display: none; }
}
</style>
