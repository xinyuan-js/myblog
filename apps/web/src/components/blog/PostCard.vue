<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { ChevronRight } from '@lucide/vue'
import type { PostSummary } from '@/types/blog'
import PostMeta from './PostMeta.vue'

defineProps<{ post: PostSummary }>()
</script>

<template>
  <article class="post-card card" :class="{ 'has-cover': post.coverUrl }">
    <div class="post-card-body">
      <RouterLink class="post-title" :to="`/posts/${post.slug}`">
        <span>{{ post.title }}</span>
        <ChevronRight class="title-arrow" :size="32" aria-hidden="true" />
      </RouterLink>
      <PostMeta :post="post" variant="card" />
      <p>{{ post.excerpt }}</p>
      <div class="reading-stats"><span>{{ post.wordCount }} 字</span><i>|</i><span>{{ post.readingTimeMinutes }} 分钟</span></div>
    </div>

    <RouterLink v-if="post.coverUrl" class="post-cover" :to="`/posts/${post.slug}`" :aria-label="post.title">
      <img :src="post.coverUrl" :alt="`${post.title}封面`" loading="lazy" />
      <span aria-hidden="true"><ChevronRight :size="48" /></span>
    </RouterLink>
    <RouterLink v-else class="enter-button" :to="`/posts/${post.slug}`" :aria-label="`阅读${post.title}`"><ChevronRight :size="36" aria-hidden="true" /></RouterLink>
  </article>
</template>

<style scoped>
.post-card { position: relative; display: flex; min-height: 11.2rem; }
.post-card-body { width: calc(100% - 4rem); padding: 1.75rem 0.5rem 1.5rem 2.25rem; }
.post-card.has-cover .post-card-body { width: calc(72% - 0.75rem); }

.post-title {
  position: relative;
  display: inline-flex;
  align-items: center;
  max-width: 100%;
  margin-bottom: 0.75rem;
  color: var(--text-strong);
  font-size: 1.875rem;
  font-weight: 700;
  line-height: 1.2;
  transition: color 150ms ease;
}

.post-title::before {
  content: '';
  position: absolute;
  top: 0.3rem;
  left: -1.15rem;
  width: 0.28rem;
  height: 1.35rem;
  border-radius: 999px;
  background: var(--primary);
}

.post-title:hover { color: var(--primary-strong); }
.title-arrow { position: absolute; top: 0.1rem; right: -2rem; color: var(--primary); opacity: 0; transform: translateX(-0.4rem); transition: opacity 150ms ease, transform 150ms ease; }
.post-title:hover .title-arrow { opacity: 1; transform: translateX(0); }
.post-card p { display: -webkit-box; margin: 1rem 1rem 0.875rem 0; overflow: hidden; color: var(--text-main); -webkit-box-orient: vertical; -webkit-line-clamp: 1; }
.reading-stats { display: flex; gap: 1rem; color: var(--text-faint); font-size: 0.875rem; }
.reading-stats i { font-style: normal; }

.enter-button,
.post-cover {
  position: absolute;
  top: 0.75rem;
  right: 0.75rem;
  bottom: 0.75rem;
  display: grid;
  overflow: hidden;
  border-radius: 0.75rem;
  background: var(--button-bg);
  transition: transform 150ms ease, background-color 150ms ease;
}

.enter-button { width: 3.25rem; place-items: center; color: var(--primary); }
.enter-button:hover { background: var(--button-hover); transform: scale(0.98); }
.post-cover { width: 28%; }
.post-cover::after { content: ''; position: absolute; inset: 0; background: transparent; transition: background-color 150ms ease; }
.post-cover:hover::after { background: oklch(0 0 0 / 0.18); }
.post-cover img { width: 100%; height: 100%; object-fit: cover; transition: transform 280ms ease; }
.post-cover:hover img { transform: scale(1.04); }
.post-cover span { position: absolute; z-index: 1; inset: 0; display: grid; place-items: center; color: white; font-size: 3rem; opacity: 0; transition: opacity 150ms ease; }
.post-cover:hover span { opacity: 1; }

.post-card { animation: card-enter 300ms both; }
@keyframes card-enter { from { opacity: 0; transform: translateY(2rem); } to { opacity: 1; transform: translateY(0); } }

@media (max-width: 767px) {
  .post-card { flex-direction: column-reverse; min-height: 0; box-shadow: none; }
  .post-card-body,
  .post-card.has-cover .post-card-body { width: 100%; padding: 1.5rem; }
  .post-title::before { display: none; }
  .post-title { font-size: 1.875rem; }
  .title-arrow { opacity: 1; transform: none; }
  .enter-button { display: none; }
  .post-card p { margin-top: 1rem; -webkit-line-clamp: 2; }
  .post-cover { position: relative; inset: auto; width: auto; height: min(20vh, 12.5rem); margin: 1rem 1rem -0.5rem; }
}
</style>
