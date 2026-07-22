<script setup lang="ts">
import type { PostSummary } from '@/types/blog'
import PostCard from './PostCard.vue'

defineProps<{ posts: PostSummary[] }>()
</script>

<template>
  <div v-if="posts.length" class="post-list">
    <PostCard v-for="(post, index) in posts" :key="post.id" :post="post" :style="{ animationDelay: `${150 + index * 50}ms` }" />
  </div>
  <section v-else class="empty-state card">
    <h2>这里还没有文章</h2>
    <p>换一个标签或分类看看吧。</p>
  </section>
</template>

<style scoped>
.post-list { display: grid; gap: 1rem; }
@media (max-width: 767px) {
  .post-list { gap: 0; padding: 0.25rem 0; overflow: hidden; border-radius: var(--radius-large); background: var(--card-bg); }
  .post-list :deep(.post-card) { border-radius: 0; }
  .post-list :deep(.post-card + .post-card::before) { content: ''; position: absolute; z-index: 2; top: 0; right: 1.5rem; left: 1.5rem; border-top: 1px dashed var(--line-color); }
}
</style>
