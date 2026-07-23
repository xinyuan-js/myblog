<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import MarkdownRenderer from '@/components/blog/MarkdownRenderer.vue'
import { api } from '@/services/api'
import type { Category, PostMutation, Tag } from '@/types/blog'
import { useDocumentMeta } from '@/composables/useDocumentMeta'
import { useAdminToast } from '@/composables/useAdminToast'

const route = useRoute()
const router = useRouter()
const id = computed(() => Number(route.params.id) || null)
const isEditing = computed(() => id.value !== null)
const tags = ref<Tag[]>([])
const categories = ref<Category[]>([])
const loading = ref(true)
const saving = ref(false)
const uploading = ref(false)
const uploadingBody = ref(false)
const markdownInput = ref<HTMLTextAreaElement | null>(null)
const slugLocked = ref(false)
const toast = useAdminToast()
const form = reactive<PostMutation>({
  title: '', slug: '', excerpt: '', contentMarkdown: '# 新文章\n\n从这里开始写作。', coverUrl: null,
  status: 'draft', publishedAt: null, categoryId: null, tagIds: [],
})

useDocumentMeta(() => isEditing.value ? '编辑文章' : '新建文章')

function toLocalDateTime(value: string | null) {
  if (!value) return ''
  const date = new Date(value)
  const offset = date.getTimezoneOffset() * 60_000
  return new Date(date.getTime() - offset).toISOString().slice(0, 16)
}

function updatePublishedAt(event: Event) {
  const value = (event.target as HTMLInputElement).value
  form.publishedAt = value ? new Date(value).toISOString() : null
}

function toggleTag(tagId: number) {
  form.tagIds = form.tagIds.includes(tagId) ? form.tagIds.filter((idValue) => idValue !== tagId) : [...form.tagIds, tagId]
}

function normalizeSlug() {
  form.slug = form.slug.trim().toLowerCase().replace(/[^a-z0-9-]+/g, '-').replace(/-{2,}/g, '-').replace(/(^-|-$)/g, '')
}

async function uploadCover(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return
  uploading.value = true
  try {
    form.coverUrl = (await api.upload(file)).url
    toast.success('封面上传成功，保存文章后生效')
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : '上传失败')
  } finally {
    uploading.value = false
  }
}

async function uploadBodyImage(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  uploadingBody.value = true
  try {
    const result = await api.upload(file)
    const textarea = markdownInput.value
    const start = textarea?.selectionStart ?? form.contentMarkdown.length
    const end = textarea?.selectionEnd ?? start
    const alt = file.name.replace(/\.[^.]+$/, '')
    const markdownValue = `\n![${alt}](${result.url})\n`
    form.contentMarkdown = `${form.contentMarkdown.slice(0, start)}${markdownValue}${form.contentMarkdown.slice(end)}`
    await nextTick()
    textarea?.focus()
    textarea?.setSelectionRange(start + markdownValue.length, start + markdownValue.length)
    toast.success('图片已插入正文')
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : '正文图片上传失败')
  } finally {
    uploadingBody.value = false
    input.value = ''
  }
}

async function save() {
  normalizeSlug()
  if (!form.title.trim() || !form.slug || !form.contentMarkdown.trim()) {
    toast.error('标题、Slug 和正文不能为空')
    return
  }
  if (form.status === 'scheduled' && !form.publishedAt) {
    toast.error('定时发布必须设置发布时间')
    return
  }
  saving.value = true
  try {
    const result = id.value ? await api.updatePost(id.value, form) : await api.createPost(form)
    toast.success('文章已保存')
    if (!id.value) await router.replace(`/admin/posts/${result.id}/edit`)
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : '保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  try {
    const [tagResult, categoryResult] = await Promise.all([api.listAdminTags(), api.listAdminCategories()])
    tags.value = tagResult
    categories.value = categoryResult
    if (id.value) {
      const post = await api.getAdminPost(id.value)
      Object.assign(form, {
        title: post.title, slug: post.slug, excerpt: post.excerpt, contentMarkdown: post.contentMarkdown,
        coverUrl: post.coverUrl, status: post.status, publishedAt: post.publishedAt,
        categoryId: post.category?.id ?? null, tagIds: post.tags.map((tag) => tag.id),
      })
      slugLocked.value = post.status === 'published' && post.publishedAt !== null && new Date(post.publishedAt) <= new Date()
    }
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : '文章加载失败')
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <header class="admin-page-header">
    <div><h1>{{ isEditing ? '编辑文章' : '写新文章' }}</h1><p>Markdown 正文会在保存后由公开页面安全渲染。</p></div>
    <div class="admin-actions"><RouterLink class="button" to="/admin/posts">返回列表</RouterLink><button class="button primary" type="button" :disabled="saving || loading" @click="save">{{ saving ? '保存中…' : '保存文章' }}</button></div>
  </header>
  <div v-if="loading" class="card loading-state">正在准备编辑器…</div>
  <form v-else class="editor-grid" @submit.prevent="save">
    <div class="editor-main">
      <section class="card admin-panel editor-fields">
        <div class="admin-field"><label for="post-title">标题</label><input id="post-title" v-model="form.title" class="admin-input" maxlength="200" required /></div>
        <div class="admin-field"><label for="post-slug">Slug</label><input id="post-slug" v-model="form.slug" class="admin-input" pattern="[a-z0-9-]+" placeholder="my-first-post" required :disabled="slugLocked" @blur="normalizeSlug" /><small v-if="slugLocked" class="field-help">文章发布后锁定 Slug，避免已有链接和评论失效。</small></div>
        <div class="admin-field"><label for="post-excerpt">摘要</label><textarea id="post-excerpt" v-model="form.excerpt" class="admin-textarea excerpt-input" maxlength="500" /></div>
        <div class="admin-field"><div class="field-label-row"><label for="post-content">Markdown 正文</label><label class="body-upload"><input class="sr-only" type="file" accept="image/jpeg,image/png,image/webp,image/gif" @change="uploadBodyImage" />{{ uploadingBody ? '上传中…' : '插入图片' }}</label></div><textarea id="post-content" ref="markdownInput" v-model="form.contentMarkdown" class="admin-textarea markdown-input" spellcheck="false" required /></div>
      </section>
      <section class="card admin-panel preview-panel"><h2>预览</h2><MarkdownRenderer :source="form.contentMarkdown" /></section>
    </div>
    <aside class="editor-sidebar">
      <section class="card admin-panel side-fields">
        <div class="admin-field"><label for="post-status">状态</label><select id="post-status" v-model="form.status" class="admin-select"><option value="draft">草稿</option><option value="published">立即发布</option><option value="scheduled">定时发布</option></select></div>
        <div class="admin-field"><label for="published-at">发布时间</label><input id="published-at" class="admin-input" type="datetime-local" :value="toLocalDateTime(form.publishedAt)" @input="updatePublishedAt" /></div>
        <div class="admin-field"><label for="post-category">分类</label><select id="post-category" v-model="form.categoryId" class="admin-select"><option :value="null">未分类</option><option v-for="category in categories" :key="category.id" :value="category.id">{{ category.name }}</option></select></div>
        <fieldset class="tag-field"><legend>标签</legend><button v-for="tag in tags" :key="tag.id" class="pill" :class="{ selected: form.tagIds.includes(tag.id) }" type="button" @click="toggleTag(tag.id)"># {{ tag.name }}</button></fieldset>
      </section>
      <section class="card admin-panel side-fields">
        <div class="admin-field"><label for="cover-url">封面地址</label><input id="cover-url" v-model="form.coverUrl" class="admin-input" type="url" placeholder="https://…" /></div>
        <label class="button upload-button"><input class="sr-only" type="file" accept="image/jpeg,image/png,image/webp,image/gif" @change="uploadCover" />{{ uploading ? '上传中…' : '上传封面' }}</label>
        <img v-if="form.coverUrl" class="cover-preview" :src="form.coverUrl" alt="封面预览" />
      </section>
    </aside>
  </form>
</template>

<style scoped>
.editor-grid { display: grid; grid-template-columns: minmax(0, 1fr) 19rem; align-items: start; gap: 1rem; }
.editor-main,
.editor-sidebar,
.editor-fields,
.side-fields { display: grid; gap: 1rem; }
.editor-sidebar { position: sticky; top: 5rem; }
.excerpt-input { min-height: 5rem; }
.markdown-input { min-height: 30rem; font-family: "SFMono-Regular", Consolas, monospace; font-size: 0.86rem; line-height: 1.7; }
.field-help { color: var(--text-faint); }
.field-label-row { display: flex; align-items: center; justify-content: space-between; }
.body-upload { color: var(--primary-strong) !important; cursor: pointer; }
.preview-panel h2 { margin: 0 0 1.2rem; color: var(--text-strong); font-size: 1rem; }
.tag-field { display: flex; flex-wrap: wrap; gap: 0.4rem; margin: 0; padding: 0; border: 0; }
.tag-field legend { width: 100%; margin-bottom: 0.4rem; color: var(--text-muted); font-size: 0.8rem; font-weight: 750; }
.tag-field button { border: 0; cursor: pointer; }
.tag-field button.selected { color: white; background: var(--primary); }
.upload-button { cursor: pointer; }
.cover-preview { width: 100%; max-height: 11rem; border-radius: 0.65rem; object-fit: cover; }
@media (max-width: 1040px) { .editor-grid { grid-template-columns: 1fr; } .editor-sidebar { position: static; grid-template-columns: 1fr 1fr; } }
@media (max-width: 650px) { .editor-sidebar { grid-template-columns: 1fr; } }
</style>
