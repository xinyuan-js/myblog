<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api } from '@/services/api'
import type { CommentUser } from '@/types/blog'
import { useAdminToast } from '@/composables/useAdminToast'
import { useDocumentMeta } from '@/composables/useDocumentMeta'

type Draft = {
  blocked: boolean
  reason: string
  dailyLimit: string
}

const users = ref<CommentUser[]>([])
const drafts = ref<Record<number, Draft>>({})
const query = ref('')
const loading = ref(true)
const saving = ref<number | null>(null)
const toast = useAdminToast()
useDocumentMeta('用户管理')

function setUsers(items: CommentUser[]) {
  users.value = items
  drafts.value = Object.fromEntries(items.map((user) => [user.githubId, {
    blocked: user.commentsBlocked,
    reason: user.commentBlockReason,
    dailyLimit: user.dailyLimit === null ? '' : String(user.dailyLimit),
  }]))
}

async function load() {
  loading.value = true
  try {
    setUsers(await api.listCommentUsers(query.value))
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : '用户列表加载失败')
  } finally {
    loading.value = false
  }
}

async function save(user: CommentUser) {
  const draft = drafts.value[user.githubId]
  if (!draft || saving.value !== null || user.isAdmin) return
  const normalized = draft.dailyLimit.trim()
  const dailyLimit = normalized === '' ? null : Number(normalized)
  if (dailyLimit !== null && (!Number.isInteger(dailyLimit) || dailyLimit < 1 || dailyLimit > 1000)) {
    toast.error('每日额度必须是 1 到 1000 之间的整数')
    return
  }
  if (draft.blocked && !draft.reason.trim()) {
    toast.error('封禁账号时请填写原因')
    return
  }
  saving.value = user.githubId
  try {
    const updated = await api.updateCommentPolicy(user.githubId, {
      commentsBlocked: draft.blocked,
      commentBlockReason: draft.reason.trim(),
      dailyLimit,
    })
    const index = users.value.findIndex((item) => item.githubId === user.githubId)
    if (index >= 0) users.value[index] = updated
    setUsers([...users.value])
    toast.success(updated.commentsBlocked ? '账号已禁止评论' : '评论策略已更新')
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : '评论策略保存失败')
  } finally {
    saving.value = null
  }
}

onMounted(() => void load())
</script>

<template>
  <header class="admin-page-header">
    <div>
      <h1>用户管理</h1>
      <p>管理通过 GitHub 登录过的评论用户。管理员账号不受封禁和每日额度限制。</p>
    </div>
  </header>

  <section class="card admin-panel search-panel">
    <form @submit.prevent="load">
      <input
        v-model="query"
        class="admin-input"
        type="search"
        autocomplete="off"
        placeholder="搜索 GitHub ID、用户名或昵称"
      />
      <button class="button primary" type="submit" :disabled="loading">{{ loading ? '查询中…' : '查询' }}</button>
    </form>
  </section>

  <section class="card admin-panel users-panel">
    <div class="panel-heading">
      <div><h2>评论用户</h2><p>留空每日额度时使用服务器默认值。</p></div>
      <span>{{ users.length }} 人</span>
    </div>
    <div v-if="loading" class="loading-state">正在读取用户…</div>
    <div v-else-if="users.length === 0" class="loading-state">没有找到用户</div>
    <div v-else class="admin-table-wrap">
      <table class="admin-table">
        <thead>
          <tr><th>用户</th><th>今日评论</th><th>每日额度</th><th>评论状态</th><th>原因</th><th>操作</th></tr>
        </thead>
        <tbody>
          <tr v-for="user in users" :key="user.githubId">
            <td>
              <div class="user-cell">
                <img v-if="user.avatarUrl" :src="user.avatarUrl" alt="" />
                <span v-else>{{ user.name.slice(0, 1) }}</span>
                <div><strong>{{ user.name }}</strong><small>@{{ user.login }} · {{ user.githubId }}</small></div>
              </div>
            </td>
            <td><strong>{{ user.todayCount }}</strong> / {{ user.effectiveDailyLimit }}</td>
            <td>
              <input
                v-model="drafts[user.githubId]!.dailyLimit"
                class="admin-input compact"
                inputmode="numeric"
                :disabled="user.isAdmin"
                :placeholder="`默认 ${user.effectiveDailyLimit}`"
              />
            </td>
            <td>
              <label v-if="!user.isAdmin" class="block-toggle">
                <input v-model="drafts[user.githubId]!.blocked" type="checkbox" />
                <span>{{ drafts[user.githubId]!.blocked ? '已封禁' : '正常' }}</span>
              </label>
              <span v-else class="status-badge">{{ user.isOwner ? '站点所有者' : '管理员' }}</span>
            </td>
            <td>
              <input
                v-model="drafts[user.githubId]!.reason"
                class="admin-input reason"
                :disabled="user.isAdmin"
                :placeholder="drafts[user.githubId]!.blocked ? '填写封禁原因' : '可选备注'"
              />
            </td>
            <td>
              <button
                class="button primary"
                type="button"
                :disabled="user.isAdmin || saving !== null"
                @click="save(user)"
              >
                {{ saving === user.githubId ? '保存中…' : '保存' }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<style scoped>
.search-panel form { display: grid; grid-template-columns: minmax(12rem, 28rem) auto; gap: 0.7rem; }
.users-panel { margin-top: 1rem; }
.panel-heading { display: flex; justify-content: space-between; gap: 1rem; margin-bottom: 1rem; }
.panel-heading h2 { margin: 0; color: var(--text-strong); font-size: 1.05rem; }
.panel-heading p { margin: 0.3rem 0 0; color: var(--text-muted); font-size: 0.82rem; }
.panel-heading > span { color: var(--text-muted); font-size: 0.78rem; }
.user-cell { display: flex; align-items: center; min-width: 13rem; gap: 0.65rem; }
.user-cell > img,
.user-cell > span { width: 2.25rem; height: 2.25rem; border-radius: 50%; object-fit: cover; }
.user-cell > span { display: grid; place-items: center; color: white; background: var(--primary); font-weight: 800; }
.user-cell strong,
.user-cell small { display: block; }
.user-cell small { margin-top: 0.15rem; color: var(--text-muted); font-size: 0.72rem; }
.admin-input.compact { width: 6rem; }
.admin-input.reason { min-width: 12rem; }
.block-toggle { display: inline-flex; align-items: center; gap: 0.45rem; white-space: nowrap; cursor: pointer; }
.block-toggle:has(input:checked) { color: oklch(.58 .18 25); }
@media (max-width: 640px) {
  .search-panel form { grid-template-columns: 1fr; }
}
</style>
