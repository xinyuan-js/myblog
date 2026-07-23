<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api } from '@/services/api'
import type { Administrator } from '@/types/blog'
import { useAdminToast } from '@/composables/useAdminToast'
import { useDocumentMeta } from '@/composables/useDocumentMeta'

const administrators = ref<Administrator[]>([])
const githubId = ref('')
const loading = ref(true)
const saving = ref(false)
const removing = ref<number | null>(null)
const toast = useAdminToast()
useDocumentMeta('管理员权限')

async function load() {
  administrators.value = await api.listAdministrators()
}

onMounted(async () => {
  try {
    await load()
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : '管理员列表加载失败')
  } finally {
    loading.value = false
  }
})

async function add() {
  const value = Number(githubId.value.trim())
  if (!Number.isSafeInteger(value) || value <= 0) {
    toast.error('请输入有效的 GitHub 数字 ID')
    return
  }
  saving.value = true
  try {
    await api.addAdministrator(value)
    githubId.value = ''
    await load()
    toast.success('管理员权限已添加')
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : '添加管理员失败')
  } finally {
    saving.value = false
  }
}

async function remove(item: Administrator) {
  if (item.isOwner || removing.value !== null) return
  if (!window.confirm(`确认移除 GitHub 用户 ${item.githubId} 的管理权限？`)) return
  removing.value = item.githubId
  try {
    await api.removeAdministrator(item.githubId)
    administrators.value = administrators.value.filter((value) => value.githubId !== item.githubId)
    toast.success('管理员权限已移除')
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : '移除管理员失败')
  } finally {
    removing.value = null
  }
}
</script>

<template>
  <header class="admin-page-header">
    <div>
      <h1>管理员权限</h1>
      <p>只有服务器配置的站点所有者可以添加或移除管理员。</p>
    </div>
  </header>

  <section class="card admin-panel permission-panel">
    <div>
      <h2>添加管理员</h2>
      <p>填写对方 GitHub 个人资料中的数字用户 ID。授权会立即生效。</p>
    </div>
    <form class="add-form" @submit.prevent="add">
      <div class="admin-field">
        <label for="administrator-github-id">GitHub 数字 ID</label>
        <input
          id="administrator-github-id"
          v-model="githubId"
          class="admin-input"
          inputmode="numeric"
          autocomplete="off"
          placeholder="例如 12345678"
        />
      </div>
      <button class="button primary" type="submit" :disabled="saving">
        {{ saving ? '添加中…' : '添加管理员' }}
      </button>
    </form>
  </section>

  <section class="card admin-panel list-panel">
    <div class="panel-heading">
      <div><h2>当前管理员</h2><p>站点所有者由 ADMIN_GITHUB_ID 配置，不能在这里移除。</p></div>
      <span>{{ administrators.length }} 人</span>
    </div>
    <div v-if="loading" class="loading-state">正在读取权限…</div>
    <div v-else class="admin-table-wrap">
      <table class="admin-table">
        <thead><tr><th>GitHub 用户 ID</th><th>权限类型</th><th>添加时间</th><th>操作</th></tr></thead>
        <tbody>
          <tr v-for="item in administrators" :key="item.githubId">
            <td><strong>{{ item.githubId }}</strong></td>
            <td><span class="status-badge" :class="{ owner: item.isOwner }">{{ item.isOwner ? '站点所有者' : '管理员' }}</span></td>
            <td>{{ item.grantedAt ? new Date(item.grantedAt).toLocaleString('zh-CN') : '服务器配置' }}</td>
            <td>
              <span v-if="item.isOwner" class="protected">受保护</span>
              <button v-else class="button danger" type="button" :disabled="removing !== null" @click="remove(item)">
                {{ removing === item.githubId ? '移除中…' : '移除' }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<style scoped>
.permission-panel { display: grid; grid-template-columns: minmax(15rem, 1fr) minmax(18rem, 1.4fr); gap: 2rem; align-items: end; }
.permission-panel h2,
.panel-heading h2 { margin: 0; color: var(--text-strong); font-size: 1.05rem; }
.permission-panel p,
.panel-heading p { margin: 0.35rem 0 0; color: var(--text-muted); font-size: 0.82rem; }
.add-form { display: grid; grid-template-columns: 1fr auto; gap: 0.7rem; align-items: end; }
.list-panel { margin-top: 1rem; }
.panel-heading { display: flex; justify-content: space-between; gap: 1rem; margin-bottom: 1rem; }
.panel-heading > span,
.protected { color: var(--text-muted); font-size: 0.78rem; }
.status-badge.owner { color: var(--primary-strong); }
.button.danger { color: oklch(.58 .18 25); }
@media (max-width: 720px) {
  .permission-panel { grid-template-columns: 1fr; gap: 1rem; }
  .add-form { grid-template-columns: 1fr; }
}
</style>
