<script setup lang="ts">
import { useDocumentMeta } from '@/composables/useDocumentMeta'
useDocumentMeta('页面不存在')
</script>

<template>
  <section class="not-found card">
    <span>404</span>
    <h1>这里没有你要找的页面</h1>
    <p>它可能被移动了，也可能只是一个写错的地址。</p>
    <RouterLink class="button primary" to="/">回到首页</RouterLink>
  </section>
</template>

<style scoped>
.not-found { display: grid; min-height: 28rem; place-items: center; align-content: center; padding: 2rem; text-align: center; }
.not-found > span { color: var(--primary); font-size: clamp(4rem, 15vw, 8rem); font-weight: 900; line-height: 1; opacity: 0.3; }
.not-found h1 { margin: 1rem 0 0.35rem; color: var(--text-strong); }
.not-found p { margin: 0 0 1.2rem; color: var(--text-muted); }
</style>
