<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api } from '@/services/api'
import type { AuditEvent, AuditEventQuery } from '@/types/blog'
import { useAdminToast } from '@/composables/useAdminToast'
import { useDocumentMeta } from '@/composables/useDocumentMeta'

const events = ref<AuditEvent[]>([])
const query = ref('')
const outcome = ref<NonNullable<AuditEventQuery['outcome']>>('all')
const page = ref(1)
const total = ref(0)
const totalPages = ref(1)
const loading = ref(true)
const toast = useAdminToast()
useDocumentMeta('操作审计')

async function load(resetPage = false) {
  if (resetPage) page.value = 1
  loading.value = true
  try {
    const result = await api.listAuditEvents({
      page: page.value,
      pageSize: 30,
      outcome: outcome.value,
      q: query.value.trim(),
    })
    if (result.pagination.total > 0 && page.value > result.pagination.totalPages) {
      page.value = result.pagination.totalPages
      await load()
      return
    }
    events.value = result.items
    total.value = result.pagination.total
    totalPages.value = result.pagination.totalPages
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : '审计日志加载失败')
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

function operationLabel(event: AuditEvent) {
  const path = event.requestPath.replace(/^\/api\/admin\//, '')
  const resource = path
    .replace(/^site\/appearance$/, '站点设置')
    .replace(/^administrators(?:\/\d+)?$/, '管理员权限')
    .replace(/^users\/\d+\/comment-policy$/, '评论用户策略')
    .replace(/^posts(?:\/\d+)?(?:\/(?:restore|permanent))?$/, '文章')
    .replace(/^tags(?:\/\d+)?$/, '标签')
    .replace(/^categories(?:\/\d+)?$/, '分类')
    .replace(/^uploads(?:\/\d+)?(?:\/(?:restore|permanent))?$/, '媒体')
  const action = event.method === 'POST' ? '创建/执行' :
    event.method === 'PUT' || event.method === 'PATCH' ? '修改' : '删除'
  return `${action} · ${resource}`
}

onMounted(() => void load())
</script>

<template>
  <header class="admin-page-header">
    <div>
      <h1>操作审计</h1>
      <p>仅记录已通过管理员会话的写操作，不保存请求正文或密钥；日志默认保留一年。</p>
    </div>
  </header>

  <section class="card admin-panel audit-filter">
    <form @submit.prevent="load(true)">
      <input
        v-model="query"
        class="admin-input"
        type="search"
        autocomplete="off"
        maxlength="100"
        placeholder="搜索管理员、资源路径或请求 ID"
      />
      <select v-model="outcome" class="admin-select" aria-label="操作结果" @change="load(true)">
        <option value="all">全部结果</option>
        <option value="success">成功</option>
        <option value="failure">失败</option>
      </select>
      <button class="button primary" type="submit" :disabled="loading">{{ loading ? '查询中…' : '查询' }}</button>
    </form>
  </section>

  <section class="card admin-panel audit-panel">
    <div class="panel-heading">
      <div><h2>管理员写操作</h2><p>失败记录包含参数校验、权限校验和服务错误。</p></div>
      <span>共 {{ total }} 条</span>
    </div>
    <div v-if="loading" class="loading-state">正在读取审计日志…</div>
    <div v-else-if="events.length === 0" class="loading-state">没有符合条件的审计记录</div>
    <div v-else class="admin-table-wrap">
      <table class="admin-table">
        <thead>
          <tr><th>时间</th><th>操作者</th><th>操作</th><th>结果</th><th>来源</th><th>请求 ID</th></tr>
        </thead>
        <tbody>
          <tr v-for="event in events" :key="event.id">
            <td>{{ new Date(event.occurredAt).toLocaleString('zh-CN') }}</td>
            <td><strong>@{{ event.actorLogin }}</strong><small>{{ event.actorGithubId }}</small></td>
            <td><strong>{{ operationLabel(event) }}</strong><small>{{ event.requestPath }}</small></td>
            <td>
              <span class="status-badge" :class="{ success: event.responseStatus >= 200 && event.responseStatus < 400, failure: event.responseStatus >= 400 }">
                HTTP {{ event.responseStatus }}
              </span>
            </td>
            <td><code>{{ event.clientIp }}</code></td>
            <td><code :title="event.requestId">{{ event.requestId.slice(0, 12) }}</code></td>
          </tr>
        </tbody>
      </table>
    </div>
    <nav v-if="totalPages > 1" class="audit-pagination" aria-label="审计日志分页">
      <button class="button" type="button" :disabled="page <= 1 || loading" @click="movePage(-1)">上一页</button>
      <span>{{ page }} / {{ totalPages }}</span>
      <button class="button" type="button" :disabled="page >= totalPages || loading" @click="movePage(1)">下一页</button>
    </nav>
  </section>
</template>

<style scoped>
.audit-filter form { display: grid; grid-template-columns: minmax(14rem, 1fr) 9rem auto; gap: 0.7rem; }
.audit-panel { margin-top: 1rem; }
.panel-heading { display: flex; justify-content: space-between; gap: 1rem; margin-bottom: 1rem; }
.panel-heading h2 { margin: 0; color: var(--text-strong); font-size: 1.05rem; }
.panel-heading p { margin: 0.35rem 0 0; color: var(--text-muted); font-size: 0.82rem; }
.panel-heading > span { color: var(--text-muted); font-size: 0.78rem; }
.admin-table td small { display: block; max-width: 22rem; margin-top: 0.2rem; overflow: hidden; color: var(--text-muted); font-size: 0.7rem; text-overflow: ellipsis; white-space: nowrap; }
.admin-table code { color: var(--text-muted); font-size: 0.75rem; }
.status-badge.success { color: oklch(.5 .14 150); background: oklch(.93 .05 150); }
.status-badge.failure { color: oklch(.55 .17 25); background: oklch(.94 .05 25); }
:global(:root.dark) .status-badge.success { color: oklch(.78 .13 150); background: oklch(.28 .05 150); }
:global(:root.dark) .status-badge.failure { color: oklch(.75 .15 25); background: oklch(.28 .05 25); }
.audit-pagination { display: flex; align-items: center; justify-content: flex-end; gap: 0.75rem; margin-top: 1rem; }
.audit-pagination span { color: var(--text-muted); font-size: 0.8rem; }
@media (max-width: 720px) {
  .audit-filter form { grid-template-columns: 1fr; }
}
</style>
