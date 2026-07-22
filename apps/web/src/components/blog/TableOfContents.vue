<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { HeadingItem } from '@/types/blog'

const props = defineProps<{ headings: HeadingItem[] }>()
const activeId = ref('')
const minLevel = computed(() => Math.min(...props.headings.map((heading) => heading.level), 6))

function updateActive() {
  let current = props.headings[0]?.id ?? ''
  for (const heading of props.headings) {
    const element = document.getElementById(heading.id)
    if (element && element.getBoundingClientRect().top <= 120) current = heading.id
  }
  activeId.value = current
}

async function refresh() {
  await nextTick()
  updateActive()
}

onMounted(() => {
  window.addEventListener('scroll', updateActive, { passive: true })
  refresh()
})
onBeforeUnmount(() => window.removeEventListener('scroll', updateActive))
watch(() => props.headings, refresh, { deep: true })
</script>

<template>
  <nav v-if="headings.length" class="toc" aria-label="文章目录">
    <a v-for="(heading, index) in headings" :key="heading.id" :href="`#${heading.id}`" :class="[{ active: activeId === heading.id }, `level-${heading.level - minLevel}`]">
      <span class="toc-badge"><template v-if="heading.level === minLevel">{{ headings.slice(0, index + 1).filter((item) => item.level === minLevel).length }}</template><i v-else /></span>
      <span>{{ heading.text }}</span>
    </a>
  </nav>
</template>

<style scoped>
.toc { position: fixed; top: 3.5rem; left: calc(50% + var(--page-width) / 2 + 1rem); width: calc((100vw - var(--page-width)) / 2 - 1rem); max-height: calc(100vh - 20rem); overflow-y: auto; scrollbar-width: none; mask-image: linear-gradient(to bottom, transparent 0, black 2rem, black calc(100% - 2rem), transparent 100%); padding: 2rem 0; }
.toc::-webkit-scrollbar { display: none; }
.toc a { position: relative; display: flex; width: 100%; min-height: 2.25rem; align-items: flex-start; gap: 0.5rem; padding: 0.5rem; border: 2px solid transparent; border-radius: 0.75rem; color: var(--text-muted); font-size: 0.875rem; line-height: 1.25rem; }
.toc a:hover { background: oklch(0.926 0.015 var(--hue)); }
.toc a.active { border-color: oklch(0.9 0.015 var(--hue)); border-style: dashed; background: oklch(0.926 0.015 var(--hue)); }
.toc-badge { display: grid; flex: none; width: 1.25rem; height: 1.25rem; place-items: center; border-radius: 0.5rem; color: var(--button-content); background: oklch(0.89 0.05 var(--hue)); font-size: 0.75rem; font-weight: 700; }
.toc-badge i { width: 0.5rem; height: 0.5rem; border-radius: 0.1875rem; background: oklch(0.89 0.05 var(--hue)); }
.toc .level-1 { padding-left: 1.5rem; color: var(--text-muted); }
.toc .level-1 .toc-badge { background: transparent; }
:global(:root.dark) .toc a:hover,
:global(:root.dark) .toc a.active { background: oklch(0.22 0.02 var(--hue)); border-color: oklch(0.25 0.02 var(--hue)); }
:global(:root.dark) .toc-badge { background: var(--button-bg); }
@media (max-width: 1535px) { .toc { display: none; } }
</style>
