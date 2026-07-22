<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api } from '@/services/api'
import type { PostStatus, PostSummary } from '@/types/blog'
import { useDocumentMeta } from '@/composables/useDocumentMeta'

const posts = ref<PostSummary[]>([])
const status = ref<PostStatus | ''>('')
const loading = ref(true)
const error = ref<string | null>(null)
useDocumentMeta('文章管理')

async function load() {
  loading.value = true
  error.value = null
  try {
    posts.value = (await api.listAdminPosts({ pageSize: 100, status: status.value || undefined })).items
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '文章加载失败'
  } finally {
    loading.value = false
  }
}

async function remove(post: PostSummary) {
  if (!window.confirm(`确定删除《${post.title}》吗？此操作不可撤销。`)) return
  await api.deletePost(post.id)
  await load()
}

onMounted(load)
</script>

<template>
  <header class="admin-page-header">
    <div><h1>文章</h1><p>管理草稿、定时发布和公开文章。</p></div>
    <RouterLink class="button primary" to="/admin/posts/new">写新文章</RouterLink>
  </header>
  <section class="card admin-panel">
    <div class="posts-toolbar">
      <label for="status-filter">状态</label>
      <select id="status-filter" v-model="status" class="admin-select" @change="load">
        <option value="">全部</option><option value="published">已发布</option><option value="draft">草稿</option><option value="scheduled">定时发布</option>
      </select>
      <span>{{ posts.length }} 篇</span>
    </div>
    <p v-if="error" class="admin-error">{{ error }}</p>
    <div v-if="loading" class="loading-state">正在加载…</div>
    <div v-else class="admin-table-wrap">
      <table class="admin-table">
        <thead><tr><th>文章</th><th>状态</th><th>分类</th><th>更新时间</th><th>操作</th></tr></thead>
        <tbody>
          <tr v-for="post in posts" :key="post.id">
            <td><strong>{{ post.title }}</strong><small>/{{ post.slug }}</small></td>
            <td><span class="status-badge" :class="post.status">{{ post.status === 'published' ? '已发布' : post.status === 'draft' ? '草稿' : '定时发布' }}</span></td>
            <td>{{ post.category?.name ?? '未分类' }}</td>
            <td>{{ new Date(post.updatedAt).toLocaleDateString('zh-CN') }}</td>
            <td><div class="row-actions"><RouterLink :to="`/admin/posts/${post.id}/edit`">编辑</RouterLink><button type="button" @click="remove(post)">删除</button></div></td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<style scoped>
.posts-toolbar { display: flex; align-items: center; gap: 0.6rem; margin-bottom: 0.8rem; }
.posts-toolbar label,
.posts-toolbar span { color: var(--text-muted); font-size: 0.8rem; }
.posts-toolbar .admin-select { width: 10rem; }
.posts-toolbar span { margin-left: auto; }
.admin-table td:first-child small { display: block; color: var(--text-faint); }
.row-actions { display: flex; gap: 0.75rem; }
.row-actions a,
.row-actions button { border: 0; padding: 0; color: var(--primary-strong); background: transparent; cursor: pointer; font-size: 0.82rem; }
.row-actions button { color: oklch(0.58 0.18 25); }
</style>
