<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api } from '@/services/api'
import type { Category, PostSummary, Tag } from '@/types/blog'
import { useDocumentMeta } from '@/composables/useDocumentMeta'

const posts = ref<PostSummary[]>([])
const tags = ref<Tag[]>([])
const categories = ref<Category[]>([])
const loading = ref(true)
const publishedCount = computed(() => posts.value.filter((post) => post.status === 'published').length)
const draftCount = computed(() => posts.value.filter((post) => post.status === 'draft').length)
useDocumentMeta('管理概览')

onMounted(async () => {
  const [postResult, tagResult, categoryResult] = await Promise.all([
    api.listAdminPosts({ pageSize: 100 }),
    api.listAdminTags(),
    api.listAdminCategories(),
  ])
  posts.value = postResult.items
  tags.value = tagResult
  categories.value = categoryResult
  loading.value = false
})
</script>

<template>
  <header class="admin-page-header">
    <div><h1>管理概览</h1><p>今天也写点值得留下来的东西。</p></div>
    <RouterLink class="button primary" to="/admin/posts/new">写新文章</RouterLink>
  </header>
  <div v-if="loading" class="card loading-state">正在读取内容…</div>
  <template v-else>
    <section class="stats-grid">
      <div class="stat-card card"><span>已发布</span><strong>{{ publishedCount }}</strong><small>篇公开文章</small></div>
      <div class="stat-card card"><span>草稿</span><strong>{{ draftCount }}</strong><small>篇等待完成</small></div>
      <div class="stat-card card"><span>标签</span><strong>{{ tags.length }}</strong><small>个内容标签</small></div>
      <div class="stat-card card"><span>分类</span><strong>{{ categories.length }}</strong><small>个文章分类</small></div>
    </section>
    <section class="card admin-panel recent-panel">
      <div class="panel-title"><h2>最近更新</h2><RouterLink to="/admin/posts">查看全部</RouterLink></div>
      <RouterLink v-for="post in posts.slice(0, 5)" :key="post.id" class="recent-row" :to="`/admin/posts/${post.id}/edit`">
        <div><strong>{{ post.title }}</strong><small>{{ new Date(post.updatedAt).toLocaleString('zh-CN') }}</small></div>
        <span class="status-badge" :class="post.status">{{ post.status === 'published' ? '已发布' : post.status === 'draft' ? '草稿' : '定时发布' }}</span>
      </RouterLink>
    </section>
  </template>
</template>

<style scoped>
.stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 1rem; }
.stat-card { display: grid; padding: 1.2rem; }
.stat-card span { color: var(--text-muted); font-size: 0.8rem; font-weight: 750; }
.stat-card strong { margin: 0.45rem 0; color: var(--text-strong); font-size: 2rem; }
.stat-card small { color: var(--text-faint); }
.recent-panel { margin-top: 1rem; }
.panel-title { display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.6rem; }
.panel-title h2 { margin: 0; color: var(--text-strong); font-size: 1.1rem; }
.panel-title a { color: var(--primary-strong); font-size: 0.8rem; }
.recent-row { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding: 0.75rem; border-radius: 0.65rem; }
.recent-row:hover { background: var(--button-bg); }
.recent-row strong,
.recent-row small { display: block; }
.recent-row strong { color: var(--text-strong); }
.recent-row small { margin-top: 0.15rem; color: var(--text-faint); font-size: 0.72rem; }
@media (max-width: 960px) { .stats-grid { grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 520px) { .stats-grid { grid-template-columns: 1fr; } }
</style>
