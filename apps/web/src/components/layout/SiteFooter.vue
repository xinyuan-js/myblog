<script setup lang="ts">
defineProps<{ authorName: string; icpNumber?: string | null }>()
</script>

<template>
  <footer class="site-footer">
    <div class="footer-line" />
    <p>© {{ new Date().getFullYear() }} {{ authorName }}. All Rights Reserved. / <RouterLink to="/archive">归档</RouterLink> / <RouterLink to="/about">关于</RouterLink><template v-if="icpNumber"> / <a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener">{{ icpNumber }}</a></template></p>
    <p>Powered by <a href="https://vuejs.org/" target="_blank" rel="noopener">Vue</a> &amp; <a href="https://github.com/saicaca/fuwari" target="_blank" rel="noopener">Fuwari</a></p>
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
