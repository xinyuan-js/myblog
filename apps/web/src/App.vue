<script setup lang="ts">
import { RouterView, useRoute } from 'vue-router'
import { computed } from 'vue'
import AdminLayout from '@/components/admin/AdminLayout.vue'
import PublicLayout from '@/components/layout/PublicLayout.vue'

const route = useRoute()
const layout = computed(() => {
  if (route.meta.layout === 'admin') return AdminLayout
  if (route.meta.layout === 'bare') return 'div'
  return PublicLayout
})
</script>

<template>
  <RouterView v-slot="{ Component }">
    <component :is="layout">
      <Transition name="route" mode="out-in">
        <div :key="route.path" class="route-view">
          <component :is="Component" />
        </div>
      </Transition>
    </component>
  </RouterView>
</template>

<style scoped>
.route-view { display: grid; min-width: 0; gap: 1rem; }
</style>
