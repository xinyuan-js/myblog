<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import { useDocumentMeta } from '@/composables/useDocumentMeta'
import { useSite } from '@/composables/useSite'

const { categories, loadSite } = useSite()
useDocumentMeta('分类')
onMounted(loadSite)
</script>

<template>
  <section class="category-panel card">
    <h1>分类</h1>
    <RouterLink v-for="category in categories" :key="category.id" class="category-row" :to="`/categories/${category.slug}`">
      <span>{{ category.name }}</span><small>{{ category.postCount }}</small>
    </RouterLink>
  </section>
</template>

<style scoped>
.category-panel { padding: 0 1rem 1rem; }
.category-panel h1 { position: relative; margin: 1rem 0 0.75rem 1rem; color: var(--text-strong); font-size: 1.125rem; }
.category-panel h1::before { content: ''; position: absolute; top: 0.3rem; left: -1rem; width: 0.25rem; height: 1rem; border-radius: 999px; background: var(--primary); }
.category-row { display: flex; height: 2.5rem; align-items: center; justify-content: space-between; padding: 0 0.5rem; border-radius: 0.5rem; color: var(--text-main); transition: padding-left 160ms ease, color 160ms ease, background-color 160ms ease; }
.category-row:hover { padding-left: 0.75rem; color: var(--primary); background: var(--plain-hover); }
.category-row small { display: grid; min-width: 2rem; height: 1.75rem; margin-left: 1rem; padding: 0 0.5rem; place-items: center; border-radius: 0.5rem; color: var(--button-content); background: var(--button-bg); font-size: 0.875rem; font-weight: 700; }
:global(:root.dark) .category-row small { color: var(--deep-text); background: var(--primary); }
</style>
