<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { ChevronUp } from '@lucide/vue'

const visible = ref(false)

function updateVisibility() {
  visible.value = window.scrollY > window.innerHeight * 0.35
}

function scrollToTop() {
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

onMounted(() => {
  updateVisibility()
  window.addEventListener('scroll', updateVisibility, { passive: true })
  window.addEventListener('resize', updateVisibility)
})

onBeforeUnmount(() => {
  window.removeEventListener('scroll', updateVisibility)
  window.removeEventListener('resize', updateVisibility)
})
</script>

<template>
  <div class="back-to-top-wrapper">
    <button
      class="back-to-top card"
      :class="{ visible }"
      type="button"
      aria-label="返回顶部"
      :tabindex="visible ? 0 : -1"
      @click="scrollToTop"
    >
      <ChevronUp :size="28" :stroke-width="2.2" aria-hidden="true" />
    </button>
  </div>
</template>

<style scoped>
.back-to-top-wrapper { position: absolute; top: 0; right: 0; display: block; width: 3.75rem; height: 3.75rem; pointer-events: none; }
.back-to-top {
  position: fixed;
  z-index: 40;
  right: max(1rem, calc((100vw - var(--page-width)) / 2 - 5rem));
  bottom: 10rem;
  display: grid;
  width: 3.75rem;
  height: 3.75rem;
  padding: 0;
  place-items: center;
  border: 0;
  color: var(--primary);
  background: var(--card-bg);
  cursor: pointer;
  opacity: 0;
  pointer-events: none;
  transform: translateX(5rem) scale(0.9);
  transition: color 300ms ease, background-color 300ms ease, opacity 300ms ease, transform 300ms ease;
}

.back-to-top.visible {
  opacity: 1;
  pointer-events: auto;
  transform: none;
}

.back-to-top:hover { background: var(--button-bg); }
.back-to-top:active { transform: scale(0.92); }

@media (max-width: 1023px) {
  .back-to-top-wrapper { display: none; }
}
</style>
