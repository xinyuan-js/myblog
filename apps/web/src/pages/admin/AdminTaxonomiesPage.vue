<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { api } from '@/services/api'
import type { Category, Tag } from '@/types/blog'
import { useDocumentMeta } from '@/composables/useDocumentMeta'
import { siteTaxonomiesUpdatedStorageKey, useSite } from '@/composables/useSite'
import { useAdminToast } from '@/composables/useAdminToast'

const tags = ref<Tag[]>([])
const categories = ref<Category[]>([])
const tagForm = reactive({ id: 0, name: '', slug: '' })
const categoryForm = reactive({ id: 0, name: '', slug: '', description: '' })
const toast = useAdminToast()
const { refreshTaxonomies } = useSite()
useDocumentMeta('标签与分类')

async function load() {
  try {
    ;[tags.value, categories.value] = await Promise.all([api.listAdminTags(), api.listAdminCategories()])
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : '分类与标签加载失败')
  }
}

async function notifyPublicTaxonomiesChanged() {
  await refreshTaxonomies()
  localStorage.setItem(siteTaxonomiesUpdatedStorageKey, String(Date.now()))
}

function resetTag() { Object.assign(tagForm, { id: 0, name: '', slug: '' }) }
function resetCategory() { Object.assign(categoryForm, { id: 0, name: '', slug: '', description: '' }) }
function editTag(tag: Tag) { Object.assign(tagForm, { id: tag.id, name: tag.name, slug: tag.slug }) }
function editCategory(category: Category) { Object.assign(categoryForm, { id: category.id, name: category.name, slug: category.slug, description: category.description ?? '' }) }

async function saveTag() {
  try {
    const input = { name: tagForm.name.trim(), slug: tagForm.slug.trim() }
    const editing = Boolean(tagForm.id)
    if (editing) await api.updateTag(tagForm.id, input)
    else await api.createTag(input)
    resetTag()
    await Promise.all([load(), notifyPublicTaxonomiesChanged()])
    toast.success(editing ? '标签已更新' : '标签已添加')
  } catch (cause) { toast.error(cause instanceof Error ? cause.message : '标签保存失败') }
}

async function saveCategory() {
  try {
    const input = { name: categoryForm.name.trim(), slug: categoryForm.slug.trim(), description: categoryForm.description.trim() || null }
    const editing = Boolean(categoryForm.id)
    if (editing) await api.updateCategory(categoryForm.id, input)
    else await api.createCategory(input)
    resetCategory()
    await Promise.all([load(), notifyPublicTaxonomiesChanged()])
    toast.success(editing ? '分类已更新' : '分类已添加')
  } catch (cause) { toast.error(cause instanceof Error ? cause.message : '分类保存失败') }
}

async function removeTag(tag: Tag) {
  if (!window.confirm(`确定删除标签“${tag.name}”吗？`)) return
  try {
    await api.deleteTag(tag.id)
    await Promise.all([load(), notifyPublicTaxonomiesChanged()])
    toast.success('标签已删除')
  } catch (cause) { toast.error(cause instanceof Error ? cause.message : '标签删除失败') }
}

async function removeCategory(category: Category) {
  if (!window.confirm(`确定删除分类“${category.name}”吗？已有文章时后端应拒绝删除。`)) return
  try {
    await api.deleteCategory(category.id)
    await Promise.all([load(), notifyPublicTaxonomiesChanged()])
    toast.success('分类已删除')
  } catch (cause) { toast.error(cause instanceof Error ? cause.message : '分类删除失败') }
}

onMounted(load)
</script>

<template>
  <header class="admin-page-header"><div><h1>标签与分类</h1><p>文章数量包含回收站；仍被回收站文章使用的项目不能删除。</p></div></header>
  <div class="taxonomy-admin-grid">
    <section class="card admin-panel taxonomy-panel">
      <h2>标签</h2>
      <form class="taxonomy-form" @submit.prevent="saveTag">
        <div class="admin-field"><label for="tag-name">名称</label><input id="tag-name" v-model="tagForm.name" class="admin-input" required /></div>
        <div class="admin-field"><label for="tag-slug">Slug</label><input id="tag-slug" v-model="tagForm.slug" class="admin-input" pattern="[a-z0-9-]+" required /></div>
        <div class="admin-actions"><button class="button primary" type="submit">{{ tagForm.id ? '保存修改' : '添加标签' }}</button><button v-if="tagForm.id" class="button" type="button" @click="resetTag">取消</button></div>
      </form>
      <div class="taxonomy-rows">
        <div v-for="tag in tags" :key="tag.id"><span><strong># {{ tag.name }}</strong><small>{{ tag.slug }} · {{ tag.postCount }} 篇（含回收站）</small></span><span><button type="button" @click="editTag(tag)">编辑</button><button class="delete-link" type="button" @click="removeTag(tag)">删除</button></span></div>
      </div>
    </section>

    <section class="card admin-panel taxonomy-panel">
      <h2>分类</h2>
      <form class="taxonomy-form" @submit.prevent="saveCategory">
        <div class="admin-field"><label for="category-name">名称</label><input id="category-name" v-model="categoryForm.name" class="admin-input" required /></div>
        <div class="admin-field"><label for="category-slug">Slug</label><input id="category-slug" v-model="categoryForm.slug" class="admin-input" pattern="[a-z0-9-]+" required /></div>
        <div class="admin-field full"><label for="category-description">说明</label><input id="category-description" v-model="categoryForm.description" class="admin-input" maxlength="300" /></div>
        <div class="admin-actions full"><button class="button primary" type="submit">{{ categoryForm.id ? '保存修改' : '添加分类' }}</button><button v-if="categoryForm.id" class="button" type="button" @click="resetCategory">取消</button></div>
      </form>
      <div class="taxonomy-rows">
        <div v-for="category in categories" :key="category.id"><span><strong>{{ category.name }}</strong><small>{{ category.slug }} · {{ category.postCount }} 篇（含回收站）</small></span><span><button type="button" @click="editCategory(category)">编辑</button><button class="delete-link" type="button" @click="removeCategory(category)">删除</button></span></div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.taxonomy-admin-grid { display: grid; grid-template-columns: 1fr 1fr; align-items: start; gap: 1rem; }
.taxonomy-panel h2 { margin: 0 0 1rem; color: var(--text-strong); font-size: 1.1rem; }
.taxonomy-form { display: grid; grid-template-columns: 1fr 1fr; gap: 0.8rem; padding-bottom: 1rem; border-bottom: 1px dashed var(--line-color); }
.taxonomy-form .full { grid-column: 1 / -1; }
.taxonomy-rows { display: grid; margin-top: 0.6rem; }
.taxonomy-rows > div { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding: 0.7rem 0.4rem; border-bottom: 1px solid var(--line-color); }
.taxonomy-rows strong,
.taxonomy-rows small { display: block; }
.taxonomy-rows strong { color: var(--text-strong); }
.taxonomy-rows small { color: var(--text-faint); font-size: 0.72rem; }
.taxonomy-rows button { border: 0; color: var(--primary-strong); background: transparent; cursor: pointer; font-size: 0.78rem; }
.taxonomy-rows .delete-link { margin-left: 0.6rem; color: oklch(0.58 0.18 25); }
@media (max-width: 980px) { .taxonomy-admin-grid { grid-template-columns: 1fr; } }
@media (max-width: 520px) { .taxonomy-form { grid-template-columns: 1fr; } .taxonomy-form .full { grid-column: auto; } }
</style>
