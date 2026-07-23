<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api } from '@/services/api'
import type { MediaItem, UploadStatus } from '@/types/blog'
import { useDocumentMeta } from '@/composables/useDocumentMeta'
import { useAdminToast } from '@/composables/useAdminToast'

const items = ref<MediaItem[]>([])
const status = ref<UploadStatus>('active')
const usage = ref<'all' | 'used' | 'unused'>('all')
const search = ref('')
const page = ref(1)
const totalPages = ref(1)
const total = ref(0)
const loading = ref(false)
const uploading = ref(false)
const selected = ref<MediaItem | null>(null)
const copied = ref<number | null>(null)
const title = computed(() => status.value === 'active' ? '媒体库' : '媒体回收站')
useDocumentMeta('媒体库')
const toast = useAdminToast()

async function load(resetPage = false) {
  if (resetPage) page.value = 1
  loading.value = true
  try {
    const result = await api.listUploads({ page: page.value, pageSize: 24, status: status.value, usage: usage.value, q: search.value.trim() || undefined })
    items.value = result.items
    total.value = result.pagination.total
    totalPages.value = result.pagination.totalPages
    if (selected.value) selected.value = items.value.find((item) => item.id === selected.value?.id) ?? null
  } catch (cause) { toast.error(cause instanceof Error ? cause.message : '媒体加载失败') }
  finally { loading.value = false }
}

async function uploadFiles(event: Event) {
  const input = event.target as HTMLInputElement
  const files = [...(input.files ?? [])]
  if (!files.length || uploading.value) return
  uploading.value = true
  try {
    for (const file of files) await api.upload(file)
    status.value = 'active'; await load(true)
    toast.success(files.length === 1 ? '图片上传成功' : `${files.length} 张图片上传成功`)
  } catch (cause) { toast.error(cause instanceof Error ? cause.message : '上传失败') }
  finally { uploading.value = false; input.value = '' }
}

async function copyURL(item: MediaItem) {
  try {
    await navigator.clipboard.writeText(new URL(item.url, window.location.origin).href)
    copied.value = item.id
    toast.success('图片 URL 已复制')
    window.setTimeout(() => { if (copied.value === item.id) copied.value = null }, 1500)
  } catch {
    toast.error('复制失败，请检查浏览器剪贴板权限')
  }
}

async function trash(item: MediaItem) {
  if (item.usageCount > 0 || !window.confirm(`将“${item.filename}”移入回收站吗？`)) return
  try { await api.trashUpload(item.id); selected.value = null; await load(); toast.success('图片已移入回收站') }
  catch (cause) { toast.error(cause instanceof Error ? cause.message : '移入回收站失败') }
}

async function restore(item: MediaItem) {
  try { await api.restoreUpload(item.id); selected.value = null; await load(); toast.success('图片已恢复') }
  catch (cause) { toast.error(cause instanceof Error ? cause.message : '恢复失败') }
}

async function removePermanent(item: MediaItem) {
  if (!window.confirm(`永久删除“${item.filename}”及其 MinIO 对象？此操作不可恢复。`)) return
  try { await api.deleteUploadPermanent(item.id); selected.value = null; await load(); toast.success('图片已永久删除') }
  catch (cause) { toast.error(cause instanceof Error ? cause.message : '永久删除失败') }
}

async function movePage(direction: number) { page.value += direction; await load() }
onMounted(() => load())
</script>

<template>
  <header class="admin-page-header">
    <div><h1>{{ title }}</h1><p>MinIO 中共有 {{ total }} 个{{ status === 'active' ? '可用' : '待清理' }}媒体文件。</p></div>
    <label class="button primary upload-button"><input class="sr-only" type="file" multiple accept="image/jpeg,image/png,image/webp,image/gif" :disabled="uploading" @change="uploadFiles" />{{ uploading ? '上传中…' : '上传图片' }}</label>
  </header>
  <section class="card admin-panel media-toolbar">
    <input v-model="search" class="admin-input" type="search" maxlength="100" placeholder="搜索原始文件名" @keyup.enter="load(true)" />
    <select v-model="usage" class="admin-select" @change="load(true)"><option value="all">全部用途</option><option value="used">使用中</option><option value="unused">未使用</option></select>
    <div class="status-switch"><button type="button" :class="{ active: status === 'active' }" @click="status = 'active'; load(true)">媒体库</button><button type="button" :class="{ active: status === 'trashed' }" @click="status = 'trashed'; load(true)">回收站</button></div>
    <button class="button" type="button" @click="load(true)">筛选</button>
  </section>
  <div v-if="loading" class="card media-empty">正在读取媒体…</div>
  <div v-else-if="!items.length" class="card media-empty">{{ status === 'active' ? '还没有符合条件的图片。' : '回收站是空的。' }}</div>
  <section v-else class="media-grid" aria-label="媒体文件">
    <article v-for="item in items" :key="item.id" class="card media-card" @click="selected = item">
      <img :src="item.url" :alt="item.filename" loading="lazy" />
      <div><strong :title="item.filename">{{ item.filename }}</strong><small>{{ item.width }}×{{ item.height }} · {{ (item.size / 1024).toFixed(1) }} KiB</small><span :class="{ used: item.usageCount > 0 }">{{ item.usageCount ? `${item.usageCount} 处使用` : '未使用' }}</span></div>
    </article>
  </section>
  <nav v-if="totalPages > 1" class="media-pagination" aria-label="媒体分页"><button class="button" type="button" :disabled="page <= 1" @click="movePage(-1)">上一页</button><span>{{ page }} / {{ totalPages }}</span><button class="button" type="button" :disabled="page >= totalPages" @click="movePage(1)">下一页</button></nav>

  <div v-if="selected" class="media-dialog-backdrop" @click.self="selected = null">
    <section class="card media-dialog" role="dialog" aria-modal="true" aria-labelledby="media-detail-title">
      <button class="dialog-close" type="button" aria-label="关闭" @click="selected = null">×</button>
      <img :src="selected.url" :alt="selected.filename" />
      <div class="media-detail"><h2 id="media-detail-title">{{ selected.filename }}</h2><p>{{ selected.contentType }} · {{ selected.width }}×{{ selected.height }} · {{ (selected.size / 1024).toFixed(1) }} KiB</p><code>{{ selected.url }}</code>
        <h3>引用位置</h3><ul v-if="selected.references.length"><li v-for="reference in selected.references" :key="`${reference.resourceType}-${reference.resourceId}-${reference.field}`">{{ reference.label }} · {{ reference.field }}</li></ul><p v-else>当前未被任何内容引用。</p>
        <div class="admin-actions"><button class="button" type="button" @click="copyURL(selected)">{{ copied === selected.id ? '已复制' : '复制 URL' }}</button><button v-if="selected.status === 'active'" class="button danger" type="button" :disabled="selected.usageCount > 0" @click="trash(selected)">移入回收站</button><template v-else><button class="button" type="button" @click="restore(selected)">恢复</button><button class="button danger" type="button" @click="removePermanent(selected)">永久删除</button></template></div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.upload-button { cursor: pointer; }
.media-toolbar { display: grid; grid-template-columns: minmax(12rem, 1fr) 10rem auto auto; gap: .7rem; margin-bottom: 1rem; }
.status-switch { display: flex; padding: .2rem; border-radius: .65rem; background: var(--button-bg); }
.status-switch button { border: 0; padding: 0 .8rem; border-radius: .5rem; color: var(--text-muted); background: transparent; cursor: pointer; }
.status-switch button.active { color: var(--primary-strong); background: var(--card-bg); }
.media-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 1rem; }
.media-card { overflow: hidden; cursor: pointer; }
.media-card > img { width: 100%; aspect-ratio: 4 / 3; object-fit: cover; background: var(--card-muted); }
.media-card > div { display: grid; gap: .25rem; padding: .7rem; }
.media-card strong { overflow: hidden; color: var(--text-strong); text-overflow: ellipsis; white-space: nowrap; }
.media-card small { color: var(--text-faint); }
.media-card span { justify-self: start; padding: .15rem .45rem; border-radius: 99px; color: var(--text-muted); background: var(--button-bg); font-size: .7rem; }
.media-card span.used { color: oklch(.5 .13 150); }
.media-empty { padding: 4rem 1rem; color: var(--text-muted); text-align: center; }
.media-pagination { display: flex; align-items: center; justify-content: center; gap: 1rem; margin-top: 1.5rem; color: var(--text-muted); }
.media-dialog-backdrop { position: fixed; z-index: 100; inset: 0; display: grid; padding: 2rem; place-items: center; background: rgb(0 0 0 / .55); }
.media-dialog { position: relative; display: grid; grid-template-columns: minmax(0, 1.3fr) minmax(18rem, 1fr); width: min(60rem, 100%); max-height: calc(100vh - 4rem); overflow: auto; }
.media-dialog > img { width: 100%; height: 100%; max-height: 38rem; object-fit: contain; background: var(--card-muted); }
.media-detail { padding: 1.2rem; }
.media-detail h2 { margin: 0; overflow-wrap: anywhere; color: var(--text-strong); }
.media-detail h3 { margin-bottom: .4rem; font-size: .9rem; }
.media-detail p, .media-detail li { color: var(--text-muted); font-size: .8rem; }
.media-detail code { display: block; padding: .6rem; overflow-wrap: anywhere; border-radius: .5rem; background: var(--button-bg); font-size: .72rem; }
.dialog-close { position: absolute; z-index: 2; top: .5rem; right: .5rem; width: 2rem; height: 2rem; border: 0; border-radius: 50%; background: var(--card-bg); cursor: pointer; }
.button.danger { color: oklch(.58 .18 25); }
@media (max-width: 1050px) { .media-grid { grid-template-columns: repeat(3, 1fr); } }
@media (max-width: 760px) { .media-toolbar { grid-template-columns: 1fr 1fr; } .media-grid { grid-template-columns: repeat(2, 1fr); } .media-dialog { grid-template-columns: 1fr; } }
@media (max-width: 480px) { .media-toolbar, .media-grid { grid-template-columns: 1fr; } .media-dialog-backdrop { padding: .5rem; } }
</style>
