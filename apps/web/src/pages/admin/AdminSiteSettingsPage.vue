<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { api } from '@/services/api'
import { siteProfileUpdatedStorageKey, useSite } from '@/composables/useSite'
import { useAdminToast } from '@/composables/useAdminToast'
import type { SiteAppearanceMutation } from '@/types/blog'

const form = reactive<SiteAppearanceMutation>({
  title: '',
  subtitle: '',
  description: '',
  avatarUrl: null,
  bannerUrl: null,
  authorName: '',
  authorBio: '',
  aboutMarkdown: '',
  socialLinks: [],
  icpNumber: null,
  publicSecurityRecordNumber: null,
})
const loading = ref(true)
const saving = ref(false)
const uploading = ref<'avatar' | 'background' | null>(null)
const { setSiteProfile } = useSite()
const toast = useAdminToast()

onMounted(async () => {
  try {
    const profile = await api.getSiteProfile()
    Object.assign(form, profile, { socialLinks: profile.socialLinks.map((link) => ({ ...link })) })
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : '站点资料加载失败')
  } finally {
    loading.value = false
  }
})

async function uploadImage(event: Event, target: 'avatar' | 'background') {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file || uploading.value) return
  uploading.value = target
  try {
    const result = await api.upload(file)
    if (target === 'avatar') form.avatarUrl = result.url
    else form.bannerUrl = result.url
    toast.success(target === 'avatar' ? '头像上传成功，保存后生效' : 'Banner 上传成功，保存后生效')
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : '图片上传失败')
  } finally {
    uploading.value = null
  }
}

function addSocialLink() {
  if (form.socialLinks.length < 10) {
    form.socialLinks.push({ label: '', url: '', icon: 'link' })
  }
}

function removeSocialLink(index: number) {
  form.socialLinks.splice(index, 1)
}

async function save() {
  if (saving.value || uploading.value) return
  saving.value = true
  try {
    const profile = await api.updateSiteAppearance({
      ...form,
      socialLinks: form.socialLinks.map((link) => ({ ...link })),
    })
    setSiteProfile(profile)
    localStorage.setItem(siteProfileUpdatedStorageKey, String(Date.now()))
    toast.success('站点设置已保存')
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : '站点外观保存失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <header class="admin-page-header">
    <div><h1>站点设置</h1><p>管理博客资料、About 内容、社交链接和图片。</p></div>
    <a class="button" href="/" target="_blank">预览首页 ↗</a>
  </header>

  <div v-if="loading" class="admin-panel card">正在加载站点资料…</div>
  <form v-else class="appearance-form" @submit.prevent="save">
    <section class="admin-panel card profile-settings">
      <div class="section-heading">
        <h2>基本资料</h2>
        <p>这些内容会显示在导航、首页资料卡、页面摘要和页脚中。</p>
      </div>
      <label class="admin-field"><span>站点标题</span><input v-model="form.title" class="admin-input" maxlength="120" required /></label>
      <label class="admin-field"><span>副标题</span><input v-model="form.subtitle" class="admin-input" maxlength="200" /></label>
      <label class="admin-field full-width"><span>站点描述</span><textarea v-model="form.description" class="admin-textarea compact" maxlength="500" /></label>
      <label class="admin-field"><span>作者名称</span><input v-model="form.authorName" class="admin-input" maxlength="120" required /></label>
      <label class="admin-field"><span>ICP备案号</span><input v-model="form.icpNumber" class="admin-input" maxlength="100" placeholder="未备案可留空" /></label>
      <label class="admin-field"><span>公安联网备案号</span><input v-model="form.publicSecurityRecordNumber" class="admin-input" maxlength="100" placeholder="例如：京公网安备 11000000000000号" /></label>
      <label class="admin-field full-width"><span>作者简介</span><textarea v-model="form.authorBio" class="admin-textarea compact" maxlength="500" /></label>
    </section>

    <section class="admin-panel card profile-settings">
      <div class="section-heading full-width">
        <h2>About 页面</h2>
        <p>使用 Markdown 编写，保存后将完整替换前台 About 正文。</p>
      </div>
      <label class="admin-field full-width"><span>About Markdown</span><textarea v-model="form.aboutMarkdown" class="admin-textarea about-editor" required /></label>
    </section>

    <section class="admin-panel card social-settings">
      <div class="section-heading">
        <h2>社交链接</h2>
        <p>支持 HTTP、HTTPS 和 mailto 地址，最多 10 个。</p>
      </div>
      <div v-for="(link, index) in form.socialLinks" :key="index" class="social-row">
        <input v-model="link.label" class="admin-input" maxlength="50" placeholder="名称，例如 GitHub" required />
        <input v-model="link.url" class="admin-input" maxlength="2048" placeholder="https://… 或 mailto:…" required />
        <select v-model="link.icon" class="admin-select">
          <option value="github">GitHub</option>
          <option value="mail">邮件</option>
          <option value="rss">RSS</option>
          <option value="link">链接</option>
        </select>
        <button class="button secondary" type="button" @click="removeSocialLink(index)">移除</button>
      </div>
      <button v-if="form.socialLinks.length < 10" class="button" type="button" @click="addSocialLink">添加链接</button>
    </section>

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
        <button v-if="form.avatarUrl" class="button secondary" type="button" @click="form.avatarUrl = null">移除头像</button>
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
        <button v-if="form.bannerUrl" class="button secondary" type="button" @click="form.bannerUrl = null">移除 Banner</button>
      </div>
    </section>

    <footer class="save-bar card">
      <span>修改后需要保存才能正式生效。</span>
      <button class="button primary" type="submit" :disabled="saving || Boolean(uploading)">{{ saving ? '保存中…' : '保存修改' }}</button>
    </footer>
  </form>
</template>

<style scoped>
.appearance-form { display: grid; gap: 1rem; }
.profile-settings { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1rem; }
.admin-field > span { color: var(--text-muted); font-size: 0.8rem; font-weight: 750; }
.section-heading { grid-column: 1 / -1; }
.section-heading h2 { margin: 0; color: var(--text-strong); font-size: 1.1rem; }
.section-heading p { margin: 0.35rem 0 0; color: var(--text-muted); font-size: 0.85rem; }
.full-width { grid-column: 1 / -1; }
.admin-textarea.compact { min-height: 5rem; }
.about-editor { min-height: 18rem; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.social-settings { display: grid; gap: 0.75rem; }
.social-row { display: grid; grid-template-columns: minmax(8rem, 0.5fr) minmax(14rem, 1.5fr) 7rem auto; gap: 0.6rem; align-items: center; }
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
@media (max-width: 700px) {
  .profile-settings { grid-template-columns: 1fr; }
  .full-width { grid-column: 1; }
  .social-row { grid-template-columns: 1fr; }
  .setting-card { grid-template-columns: 1fr; }
  .setting-card .admin-actions { grid-column: 1; }
  .save-bar { position: static; align-items: stretch; flex-direction: column; }
}
</style>
