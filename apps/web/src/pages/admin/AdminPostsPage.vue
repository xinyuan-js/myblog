<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api } from '@/services/api'
import type { PostStatus, PostSummary } from '@/types/blog'
import { useDocumentMeta } from '@/composables/useDocumentMeta'
import { useAdminToast } from '@/composables/useAdminToast'

const posts = ref<PostSummary[]>([])
const status = ref<PostStatus | ''>('')
const view = ref<'active' | 'trash'>('active')
const page = ref(1)
const total = ref(0)
const totalPages = ref(1)
const loading = ref(true)
const toast = useAdminToast()
useDocumentMeta('文章管理')

async function load(resetPage = false) {
  if (resetPage) page.value = 1
  loading.value = true
  try {
    const result = view.value === 'trash'
      ? await api.listTrashedPosts({ page: page.value, pageSize: 20 })
      : await api.listAdminPosts({
          page: page.value,
          pageSize: 20,
          status: status.value || undefined,
        })
    if (result.pagination.total > 0 && page.value > result.pagination.totalPages) {
      page.value = result.pagination.totalPages
      await load()
      return
    }
    posts.value = result.items
    total.value = result.pagination.total
    totalPages.value = result.pagination.totalPages
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : '文章加载失败')
  } finally {
    loading.value = false
  }
}

function movePage(offset: number) {
  const next = page.value + offset
  if (next < 1 || next > totalPages.value || loading.value) return
  page.value = next
  void load()
}

async function remove(post: PostSummary) {
  if (!window.confirm(`确定将《${post.title}》移入回收站吗？文章及其分类、标签和图片引用会保留。`)) return
  try {
    await api.deletePost(post.id)
    await load()
    toast.success('文章已移入回收站')
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : '文章删除失败')
  }
}

async function restore(post: PostSummary) {
  try {
    await api.restorePost(post.id)
    await load()
    toast.success('文章及其关联已恢复')
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : '文章恢复失败')
  }
}

async function removePermanent(post: PostSummary) {
  if (!window.confirm(`永久删除《${post.title}》？这会释放分类、标签和图片引用，且无法从管理端恢复。`)) return
  try {
    await api.deletePostPermanent(post.id)
    await load()
    toast.success('文章已永久删除')
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : '文章永久删除失败')
  }
}

function switchView(next: 'active' | 'trash') {
  if (view.value === next) return
  view.value = next
  status.value = ''
  void load(true)
}

onMounted(() => void load())
</script>

<template>
  <header class="admin-page-header">
    <div><h1>文章</h1><p>管理草稿、定时发布、公开文章和回收站。</p></div>
    <RouterLink v-if="view === 'active'" class="button primary" to="/admin/posts/new">写新文章</RouterLink>
  </header>
  <section class="card admin-panel">
    <div class="posts-toolbar">
      <div class="view-switch" role="group" aria-label="文章视图">
        <button type="button" :class="{ active: view === 'active' }" @click="switchView('active')">文章</button>
        <button type="button" :class="{ active: view === 'trash' }" @click="switchView('trash')">回收站</button>
      </div>
      <label v-if="view === 'active'" for="status-filter">状态</label>
      <select v-if="view === 'active'" id="status-filter" v-model="status" class="admin-select" @change="load(true)">
        <option value="">全部</option><option value="published">已发布</option><option value="draft">草稿</option><option value="scheduled">定时发布</option>
      </select>
      <span>共 {{ total }} 篇</span>
    </div>
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
            <td>
              <div v-if="view === 'active'" class="row-actions">
                <RouterLink :to="`/admin/posts/${post.id}/edit`">编辑</RouterLink>
                <button type="button" @click="remove(post)">移入回收站</button>
              </div>
              <div v-else class="row-actions">
                <button class="restore" type="button" @click="restore(post)">恢复</button>
                <button type="button" @click="removePermanent(post)">永久删除</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <nav v-if="totalPages > 1" class="post-pagination" aria-label="文章分页">
      <button class="button" type="button" :disabled="page <= 1 || loading" @click="movePage(-1)">上一页</button>
      <span>{{ page }} / {{ totalPages }}</span>
      <button class="button" type="button" :disabled="page >= totalPages || loading" @click="movePage(1)">下一页</button>
    </nav>
  </section>
</template>

<style scoped>
.posts-toolbar { display: flex; align-items: center; gap: 0.6rem; margin-bottom: 0.8rem; }
.posts-toolbar label,
.posts-toolbar span { color: var(--text-muted); font-size: 0.8rem; }
.posts-toolbar .admin-select { width: 10rem; }
.posts-toolbar span { margin-left: auto; }
.view-switch { display: flex; gap: 0.25rem; margin-right: 0.4rem; padding: 0.2rem; border-radius: 0.65rem; background: var(--surface-soft); }
.view-switch button { border: 0; border-radius: 0.5rem; padding: 0.42rem 0.72rem; color: var(--text-muted); background: transparent; cursor: pointer; }
.view-switch button.active { color: var(--text-strong); background: var(--surface); box-shadow: 0 1px 4px color-mix(in srgb, var(--text-strong) 10%, transparent); }
.admin-table td:first-child small { display: block; color: var(--text-faint); }
.row-actions { display: flex; gap: 0.75rem; }
.row-actions a,
.row-actions button { border: 0; padding: 0; color: var(--primary-strong); background: transparent; cursor: pointer; font-size: 0.82rem; }
.row-actions button { color: oklch(0.58 0.18 25); }
.row-actions button.restore { color: var(--primary-strong); }
.post-pagination { display: flex; align-items: center; justify-content: flex-end; gap: 0.75rem; margin-top: 1rem; }
.post-pagination span { color: var(--text-muted); font-size: 0.8rem; }
</style>
