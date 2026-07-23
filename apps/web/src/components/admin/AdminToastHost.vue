<script setup lang="ts">
import { useAdminToast } from '@/composables/useAdminToast'

const { toasts, dismissToast } = useAdminToast()
</script>

<template>
  <div class="admin-toast-host" aria-live="polite" aria-atomic="false">
    <TransitionGroup name="admin-toast">
      <div
        v-for="toast in toasts"
        :key="toast.id"
        class="admin-toast"
        :class="toast.type"
        :role="toast.type === 'error' ? 'alert' : 'status'"
      >
        <span class="toast-mark" aria-hidden="true">{{ toast.type === 'success' ? '✓' : toast.type === 'error' ? '!' : 'i' }}</span>
        <span class="toast-message">{{ toast.message }}</span>
        <button type="button" aria-label="关闭提示" @click="dismissToast(toast.id)">×</button>
      </div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.admin-toast-host {
  position: fixed;
  z-index: 220;
  top: 1rem;
  left: 50%;
  display: grid;
  width: min(28rem, calc(100vw - 2rem));
  gap: 0.65rem;
  transform: translateX(-50%);
  pointer-events: none;
}
.admin-toast {
  display: grid;
  grid-template-columns: 1.7rem minmax(0, 1fr) 1.8rem;
  align-items: center;
  gap: 0.65rem;
  min-height: 3.4rem;
  padding: 0.65rem 0.7rem;
  border: 1px solid var(--line-color);
  border-radius: 0.85rem;
  color: var(--text-main);
  background: var(--float-panel-bg);
  box-shadow: var(--shadow-float);
  backdrop-filter: blur(18px);
  pointer-events: auto;
}
.toast-mark {
  display: grid;
  width: 1.7rem;
  height: 1.7rem;
  place-items: center;
  border-radius: 50%;
  color: white;
  background: var(--primary);
  font-size: 0.8rem;
  font-weight: 900;
}
.admin-toast.success .toast-mark { background: oklch(0.58 0.14 150); }
.admin-toast.error .toast-mark { background: oklch(0.58 0.18 25); }
.toast-message { overflow-wrap: anywhere; font-size: 0.88rem; font-weight: 700; line-height: 1.45; }
.admin-toast button {
  width: 1.8rem;
  height: 1.8rem;
  border: 0;
  border-radius: 0.45rem;
  color: var(--text-muted);
  background: transparent;
  cursor: pointer;
  font-size: 1.2rem;
}
.admin-toast button:hover { color: var(--text-strong); background: var(--button-bg); }
.admin-toast-enter-active,
.admin-toast-leave-active { transition: opacity 180ms ease, transform 180ms ease; }
.admin-toast-enter-from,
.admin-toast-leave-to { opacity: 0; transform: translateY(-1rem) scale(0.97); }
.admin-toast-leave-active { position: absolute; width: 100%; }
</style>
