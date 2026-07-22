<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { api } from '@/services/api'
import { useSite } from '@/composables/useSite'
import type { SiteAppearanceMutation } from '@/types/blog'

const form = reactive<SiteAppearanceMutation>({ avatarUrl: null, bannerUrl: null })
const loading = ref(true)
const saving = ref(false)
const uploading = ref<'avatar' | 'background' | null>(null)
const error = ref<string | null>(null)
const saved = ref(false)
const { setSiteProfile } = useSite()

onMounted(async () => {
  try {
    const profile = await api.getSiteProfile()
    form.avatarUrl = profile.avatarUrl
    form.bannerUrl = profile.bannerUrl
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '站点资料加载失败'
  } finally {
    loading.value = false
  }
})

async function uploadImage(event: Event, target: 'avatar' | 'background') {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file || uploading.value) return
  error.value = null
  saved.value = false
  uploading.value = target
  try {
    const result = await api.upload(file)
    if (target === 'avatar') form.avatarUrl = result.url
    else form.bannerUrl = result.url
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '图片上传失败'
  } finally {
    uploading.value = null
  }
}

async function save() {
  if (saving.value || uploading.value) return
  saving.value = true
  saved.value = false
  error.value = null
  try {
    const profile = await api.updateSiteAppearance({ ...form })
    setSiteProfile(profile)
    saved.value = true
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '站点外观保存失败'
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <header class="admin-page-header">
    <div><h1>站点设置</h1><p>管理博客头像和全站 Banner 图片。</p></div>
    <a class="button" href="/" target="_blank">预览首页 ↗</a>
  </header>

  <p v-if="error" class="admin-error" role="alert">{{ error }}</p>
  <div v-if="loading" class="admin-panel card">正在加载站点资料…</div>
  <form v-else class="appearance-form" @submit.prevent="save">
    <section class="admin-panel card setting-card">
      <div>
        <h2>主页头像</h2>
        <p>显示在首页左侧个人资料卡片中，推荐使用正方形图片。</p>
      </div>
      <div class="avatar-preview preview-surface">
        <img v-if="form.avatarUrl" :src="form.avatarUrl" alt="头像预览" />
        <span v-else>暂无头像</span>
      </div>
      <div class="admin-actions">
        <label class="button upload-action"><input class="sr-only" type="file" accept="image/jpeg,image/png,image/webp,image/gif" @change="uploadImage($event, 'avatar')" />{{ uploading === 'avatar' ? '上传中…' : '上传头像' }}</label>
        <button v-if="form.avatarUrl" class="button secondary" type="button" @click="form.avatarUrl = null; saved = false">移除头像</button>
      </div>
    </section>

    <section class="admin-panel card setting-card background-setting">
      <div>
        <h2>全站 Banner</h2>
        <p>所有公开页面都会展示；桌面首页使用 65vh 高度，其他页面和移动端使用 35vh。</p>
      </div>
      <div class="background-preview preview-surface">
        <img v-if="form.bannerUrl" :src="form.bannerUrl" alt="全站 Banner 预览" />
        <span v-else>未设置 Banner，公开页面将使用无图布局</span>
      </div>
      <div class="admin-actions">
        <label class="button upload-action"><input class="sr-only" type="file" accept="image/jpeg,image/png,image/webp,image/gif" @change="uploadImage($event, 'background')" />{{ uploading === 'background' ? '上传中…' : '上传 Banner' }}</label>
        <button v-if="form.bannerUrl" class="button secondary" type="button" @click="form.bannerUrl = null; saved = false">移除 Banner</button>
      </div>
    </section>

    <footer class="save-bar card">
      <p v-if="saved" class="save-success" role="status">站点外观已保存，公开页面会立即使用新图片。</p>
      <span v-else>上传图片后需要保存才能正式生效。</span>
      <button class="button primary" type="submit" :disabled="saving || Boolean(uploading)">{{ saving ? '保存中…' : '保存修改' }}</button>
    </footer>
  </form>
</template>

<style scoped>
.appearance-form { display: grid; gap: 1rem; }
.setting-card { display: grid; grid-template-columns: minmax(12rem, 0.7fr) minmax(16rem, 1.3fr); gap: 1.2rem 2rem; align-items: center; }
.setting-card h2 { margin: 0; color: var(--text-strong); font-size: 1.1rem; }
.setting-card p { margin: 0.35rem 0 0; color: var(--text-muted); font-size: 0.85rem; line-height: 1.6; }
.preview-surface { display: grid; overflow: hidden; place-items: center; border: 1px dashed var(--line-color); border-radius: 0.8rem; color: var(--text-muted); background: var(--card-muted); }
.preview-surface img { width: 100%; height: 100%; object-fit: cover; }
.avatar-preview { width: 10rem; height: 10rem; justify-self: start; }
.background-preview { width: 100%; aspect-ratio: 16 / 6; }
.setting-card .admin-actions { grid-column: 2; }
.upload-action { cursor: pointer; }
.button.secondary { color: var(--text-muted); background: transparent; }
.save-bar { position: sticky; z-index: 10; bottom: 1rem; display: flex; min-height: 4.2rem; align-items: center; justify-content: space-between; gap: 1rem; padding: 0.8rem 1rem; box-shadow: var(--shadow-float); }
.save-bar span { color: var(--text-muted); font-size: 0.82rem; }
.save-success { margin: 0; color: oklch(0.5 0.13 150); font-size: 0.85rem; font-weight: 700; }
@media (max-width: 700px) {
  .setting-card { grid-template-columns: 1fr; }
  .setting-card .admin-actions { grid-column: 1; }
  .save-bar { position: static; align-items: stretch; flex-direction: column; }
}
</style>
