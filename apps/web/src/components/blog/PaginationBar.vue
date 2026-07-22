<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { ChevronLeft, ChevronRight, Ellipsis } from '@lucide/vue'
import type { PaginationMeta } from '@/types/blog'
import { createPaginationItems, withPageQuery } from '@/utils/pagination'

const props = defineProps<{ pagination: PaginationMeta }>()
const route = useRoute()

const previousQuery = computed(() => withPageQuery(route.query, props.pagination.page - 1))
const nextQuery = computed(() => withPageQuery(route.query, props.pagination.page + 1))
const pages = computed(() => createPaginationItems(props.pagination.totalPages, props.pagination.page))
const pageQuery = (page: number) => withPageQuery(route.query, page)
</script>

<template>
  <nav v-if="pagination.totalPages > 1" class="pagination" aria-label="文章分页">
    <span v-if="pagination.page <= 1" class="page-button side disabled" aria-disabled="true"><ChevronLeft :size="26" /></span>
    <RouterLink v-else class="page-button side" :to="{ query: previousQuery }" aria-label="上一页"><ChevronLeft :size="26" /></RouterLink>
    <div class="page-numbers">
      <template v-for="(pageValue, index) in pages" :key="`${pageValue}-${index}`">
        <span v-if="pageValue === 'ellipsis'" class="page-ellipsis" aria-hidden="true"><Ellipsis :size="18" /></span>
        <span v-else-if="pageValue === pagination.page" class="page-button current" aria-current="page">{{ pageValue }}</span>
        <RouterLink v-else class="page-button" :to="{ query: pageQuery(pageValue) }">{{ pageValue }}</RouterLink>
      </template>
    </div>
    <span v-if="pagination.page >= pagination.totalPages" class="page-button side disabled" aria-disabled="true"><ChevronRight :size="26" /></span>
    <RouterLink v-else class="page-button side" :to="{ query: nextQuery }" aria-label="下一页"><ChevronRight :size="26" /></RouterLink>
  </nav>
</template>

<style scoped>
.pagination { display: flex; align-items: center; justify-content: center; gap: 0.75rem; padding: 0.5rem; }
.page-numbers { display: flex; min-height: 2.75rem; align-items: center; overflow: hidden; border-radius: 0.5rem; background: var(--card-bg); }
.page-button { display: grid; width: 2.75rem; height: 2.75rem; place-items: center; color: var(--text-main); font-weight: 750; transition: transform 150ms ease, color 150ms ease, background-color 150ms ease; }
.page-button:hover { color: var(--primary-strong); background: var(--card-hover); }
.page-button.current { color: white; background: var(--primary); }
.page-button.side { border-radius: 0.5rem; color: var(--primary); background: var(--card-bg); }
.page-ellipsis { display: grid; width: 2.25rem; height: 2.75rem; place-items: center; color: var(--text-faint); }
.disabled { opacity: 0.45; cursor: not-allowed; }
:global(:root.dark) .page-button.current { color: rgb(0 0 0 / 0.7); }
</style>
