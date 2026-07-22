<script setup lang="ts">
import { onMounted } from 'vue'
import { useDocumentMeta } from '@/composables/useDocumentMeta'
import { useSite } from '@/composables/useSite'

const { profile, loadSite } = useSite()
useDocumentMeta('关于')
onMounted(loadSite)
</script>

<template>
  <article class="about-card card">
    <h1>你好，我是{{ profile?.authorName ?? '见山' }}。</h1>
    <p>{{ profile?.authorBio }}</p>
    <h2>关于这个博客</h2>
    <p>{{ profile?.description }}</p>
    <p>这里主要记录 Go、数据库和软件工程实践，也会写下一些阅读笔记与生活片段。文章会尽量把背景、取舍和失败过程说清楚，而不仅仅给出最后的答案。</p>
    <h2>联系我</h2>
    <div class="contact-list">
      <a v-for="link in profile?.socialLinks" :key="link.url" class="button" :href="link.url" target="_blank" rel="me noopener">{{ link.label }}</a>
    </div>
  </article>
</template>

<style scoped>
.about-card { padding: 1.5rem 2.25rem; }
.about-card h1 { max-width: 46rem; margin: 0.5rem 0 1rem; color: var(--text-strong); font-size: 1.875rem; line-height: 1.25; }
.about-card h2 { margin: 2rem 0 0.6rem; color: var(--text-strong); }
.about-card p { max-width: 48rem; color: var(--text-main); line-height: 1.85; }
.contact-list { display: flex; flex-wrap: wrap; gap: 0.6rem; }
</style>
