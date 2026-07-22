<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { ChevronLeft, ChevronRight, Clock3, FileText } from '@lucide/vue'
import ArtalkComments from '@/components/comments/ArtalkComments.vue'
import LoadingCard from '@/components/common/LoadingCard.vue'
import MarkdownRenderer from '@/components/blog/MarkdownRenderer.vue'
import PostMeta from '@/components/blog/PostMeta.vue'
import TableOfContents from '@/components/blog/TableOfContents.vue'
import { useDocumentMeta } from '@/composables/useDocumentMeta'
import { api } from '@/services/api'
import type { HeadingItem, PostDetail } from '@/types/blog'

const route = useRoute()
const post = ref<PostDetail | null>(null)
const headings = ref<HeadingItem[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const slug = computed(() => String(route.params.slug))

useDocumentMeta(() => post.value?.title ?? '文章', () => post.value?.excerpt)

async function load() {
  loading.value = true
  error.value = null
  headings.value = []
  window.scrollTo({ top: 0 })
  try {
    post.value = await api.getPost(slug.value)
  } catch (cause) {
    post.value = null
    error.value = cause instanceof Error ? cause.message : '文章加载失败'
  } finally {
    loading.value = false
  }
}

watch(slug, load, { immediate: true })
</script>

<template>
  <LoadingCard v-if="loading" />
  <section v-else-if="error" class="error-state card">
    <h2>文章没有找到</h2>
    <p>{{ error }}</p>
    <RouterLink class="button" to="/">返回首页</RouterLink>
  </section>
  <template v-else-if="post">
    <article class="article-card card">
      <header class="article-header">
        <div class="article-stats"><span><i><FileText :size="16" /></i>{{ post.wordCount }} 字</span><span><i><Clock3 :size="16" /></i>{{ post.readingTimeMinutes }} 分钟</span></div>
        <h1>{{ post.title }}</h1>
        <PostMeta :post="post" />
      </header>
      <div v-if="!post.coverUrl" class="article-divider" />
      <img v-else class="article-cover" :src="post.coverUrl" :alt="`${post.title}封面`" />
      <MarkdownRenderer :source="post.contentMarkdown" strip-leading-h1 @headings="headings = $event" />
    </article>
    <nav class="post-neighbors" aria-label="相邻文章">
      <RouterLink v-if="post.previousPost" :to="`/posts/${post.previousPost.slug}`"><ChevronLeft :size="28" /><span>{{ post.previousPost.title }}</span></RouterLink><span v-else />
      <RouterLink v-if="post.nextPost" class="next" :to="`/posts/${post.nextPost.slug}`"><span>{{ post.nextPost.title }}</span><ChevronRight :size="28" /></RouterLink>
    </nav>
    <TableOfContents :headings="headings" />
    <ArtalkComments :page-key="`/posts/${post.slug}`" :page-title="post.title" />
  </template>
</template>

<style scoped>
.article-card { padding: 1.5rem 2.25rem 1rem; overflow: visible; }
.article-header { text-align: left; }
.article-stats { display: flex; gap: 1.25rem; margin-bottom: 0.75rem; color: var(--text-faint); font-size: 0.875rem; }
.article-stats span { display: flex; align-items: center; gap: 0.45rem; }
.article-stats i { display: grid; width: 1.5rem; height: 1.5rem; place-items: center; border-radius: 0.38rem; color: var(--text-muted); background: var(--button-bg); }
.article-header h1 { position: relative; margin: 0 0 0.75rem; color: var(--text-strong); font-size: 2.25rem; font-weight: 700; line-height: 2.75rem; }
.article-header h1::before { content: ''; position: absolute; top: 0.75rem; left: -1.15rem; width: 0.28rem; height: 1.3rem; border-radius: 999px; background: var(--primary); }
.article-divider { margin: 1.25rem 0; border-bottom: 1px dashed var(--line-divider); }
.article-cover { width: 100%; max-height: 30rem; margin: 1.5rem 0 2rem; border-radius: 0.75rem; object-fit: cover; }
.post-neighbors { display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; }
.post-neighbors a { display: flex; min-width: 0; height: 3.75rem; align-items: center; gap: 0.8rem; padding: 0 1rem; overflow: hidden; border-radius: var(--radius-large); background: var(--card-bg); font-weight: 750; transition: transform 150ms ease, background-color 150ms ease; }
.post-neighbors a:hover { color: var(--primary-strong); background: var(--card-hover); }
.post-neighbors a:active { transform: scale(0.97); }
.post-neighbors a svg { flex: none; color: var(--primary); }
.post-neighbors span { overflow: hidden; color: var(--text-main); text-overflow: ellipsis; white-space: nowrap; }
.post-neighbors .next { justify-content: flex-end; text-align: right; }
.error-state .button { margin-top: 1rem; }
@media (max-width: 767px) { .article-card { padding: 1.5rem 1.5rem 1rem; } .article-header h1 { font-size: 1.875rem; line-height: 2.25rem; } .article-header h1::before { display: none; } .post-neighbors { grid-template-columns: 1fr; } }
</style>
