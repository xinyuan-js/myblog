<script setup lang="ts">
import { onMounted } from 'vue'
import { useDocumentMeta } from '@/composables/useDocumentMeta'
import { useSite } from '@/composables/useSite'
import MarkdownRenderer from '@/components/blog/MarkdownRenderer.vue'

const { profile, loadSite } = useSite()
useDocumentMeta('关于')
onMounted(loadSite)
</script>

<template>
  <article class="about-card card">
    <MarkdownRenderer v-if="profile?.aboutMarkdown" :source="profile.aboutMarkdown" />
    <template v-else>
      <h1>关于</h1>
      <p>正在加载站点资料…</p>
    </template>
    <div v-if="profile?.socialLinks.length" class="contact-list">
      <a v-for="link in profile?.socialLinks" :key="link.url" class="button" :href="link.url" target="_blank" rel="me noopener">{{ link.label }}</a>
    </div>
  </article>
</template>

<style scoped>
.about-card { padding: 1.5rem 2.25rem; }
.contact-list { display: flex; flex-wrap: wrap; gap: 0.6rem; margin-top: 2rem; padding-top: 1rem; border-top: 1px dashed var(--line-color); }
</style>
