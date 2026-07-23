<script setup lang="ts">
defineProps<{
  authorName: string
  icpNumber?: string | null
  publicSecurityRecordNumber?: string | null
}>()

function publicSecurityRecordURL(value: string) {
  const recordCode = value.match(/\d{10,}/)?.[0]
  const baseURL = 'https://beian.mps.gov.cn/#/query/webSearch'
  return recordCode ? `${baseURL}?code=${encodeURIComponent(recordCode)}` : baseURL
}
</script>

<template>
  <footer class="site-footer">
    <div class="footer-line" />
    <p>
      © {{ new Date().getFullYear() }} {{ authorName }}
      / <RouterLink to="/archive">归档</RouterLink>
      / <RouterLink to="/about">关于</RouterLink>
      <template v-if="icpNumber">
        / <a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener noreferrer">{{ icpNumber }}</a>
      </template>
      <template v-if="publicSecurityRecordNumber">
        / <a :href="publicSecurityRecordURL(publicSecurityRecordNumber)" target="_blank" rel="noopener noreferrer">{{ publicSecurityRecordNumber }}</a>
      </template>
    </p>
  </footer>
</template>

<style scoped>
.site-footer { padding: 0 1.5rem 3rem; color: var(--text-muted); text-align: center; font-size: 0.875rem; }
.footer-line { margin: 2.5rem 8rem; border-top: 1px dashed var(--line-color); }
.site-footer p { margin: 0.1rem 0; }
.site-footer a { color: var(--primary); font-weight: 500; }
.site-footer a:hover { background: var(--plain-hover); }
@media (max-width: 640px) { .footer-line { margin-right: 1.5rem; margin-left: 1.5rem; } }
</style>
