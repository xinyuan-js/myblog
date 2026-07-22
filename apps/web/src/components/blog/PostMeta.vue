<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { BookOpen, CalendarClock, CalendarDays, Tag as TagIcon } from '@lucide/vue'
import type { PostSummary } from '@/types/blog'

const props = withDefaults(defineProps<{ post: PostSummary; variant?: 'card' | 'detail' }>(), { variant: 'detail' })

function formatDate(value: string) {
  const date = new Date(value)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const publishedLabel = computed(() => {
  if (!props.post.publishedAt) return '未发布'
  return formatDate(props.post.publishedAt)
})
const updatedLabel = computed(() => formatDate(props.post.updatedAt))
const showUpdated = computed(() => {
  if (props.variant === 'card') return false
  if (!props.post.publishedAt) return true
  return new Date(props.post.updatedAt).getTime() !== new Date(props.post.publishedAt).getTime()
})
</script>

<template>
  <div class="post-meta">
    <div class="meta-item"><span class="meta-icon"><CalendarDays :size="18" /></span><time v-if="post.publishedAt" :datetime="post.publishedAt">{{ publishedLabel }}</time><span v-else>草稿</span></div>
    <div v-if="showUpdated" class="meta-item"><span class="meta-icon"><CalendarClock :size="18" /></span><time :datetime="post.updatedAt">{{ updatedLabel }}</time></div>
    <div class="meta-item"><span class="meta-icon"><BookOpen :size="18" /></span><RouterLink v-if="post.category" :to="`/categories/${post.category.slug}`">{{ post.category.name }}</RouterLink><span v-else>未分类</span></div>
    <div class="meta-item tag-meta"><span class="meta-icon"><TagIcon :size="18" /></span><span class="meta-tags"><RouterLink v-for="(tag, index) in post.tags" :key="tag.id" :to="`/tags/${tag.slug}`"><i v-if="index">/</i>{{ tag.name }}</RouterLink><span v-if="!post.tags.length">无标签</span></span></div>
  </div>
</template>

<style scoped>
.post-meta { display: flex; flex-wrap: wrap; align-items: center; column-gap: 1rem; row-gap: 0.5rem; color: var(--text-muted); font-size: 0.875rem; font-weight: 500; }
.meta-item { display: flex; min-width: 0; align-items: center; }
.meta-icon { display: grid; flex: none; width: 2rem; height: 2rem; margin-right: 0.5rem; place-items: center; border-radius: 0.5rem; color: var(--primary-strong); background: var(--button-bg); }
.meta-tags { display: flex; min-width: 0; align-items: center; white-space: nowrap; }
.meta-tags i { margin: 0 0.375rem; color: var(--meta-divider); font-style: normal; }
.post-meta a:hover { color: var(--primary-strong); }
@media (max-width: 767px) { .post-meta .tag-meta { display: none; } }
</style>
