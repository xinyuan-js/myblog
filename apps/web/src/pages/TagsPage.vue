<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import { useDocumentMeta } from '@/composables/useDocumentMeta'
import { useSite } from '@/composables/useSite'

const { tags, loadSite } = useSite()
useDocumentMeta('标签')
onMounted(loadSite)
</script>

<template>
  <section class="taxonomy-panel card">
    <h1>标签</h1>
    <div class="tag-cloud">
      <RouterLink v-for="tag in tags" :key="tag.id" class="pill" :to="`/tags/${tag.slug}`">{{ tag.name }}</RouterLink>
    </div>
  </section>
</template>

<style scoped>
.taxonomy-panel { padding: 0 1rem 1rem; }
.taxonomy-panel h1 { position: relative; margin: 1rem 0 0.75rem 1rem; color: var(--text-strong); font-size: 1.125rem; }
.taxonomy-panel h1::before { content: ''; position: absolute; top: 0.3rem; left: -1rem; width: 0.25rem; height: 1rem; border-radius: 999px; background: var(--primary); }
.tag-cloud { display: flex; flex-wrap: wrap; gap: 0.5rem; }
</style>
