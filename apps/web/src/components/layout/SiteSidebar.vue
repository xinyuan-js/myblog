<script setup lang="ts">
import { ref } from 'vue'
import { RouterLink } from 'vue-router'
import { Contact, GitFork, Mail, MoreHorizontal } from '@lucide/vue'
import type { Category, SiteProfile, Tag } from '@/types/blog'

defineProps<{
  profile: SiteProfile | null
  tags: Tag[]
  categories: Category[]
}>()

const categoriesExpanded = ref(false)
const tagsExpanded = ref(false)
</script>

<template>
  <aside class="sidebar" aria-label="站点信息">
    <section class="profile-card card">
      <RouterLink class="avatar" to="/about" aria-label="查看关于页面">
        <i class="avatar-overlay"><Contact :size="48" aria-hidden="true" /></i>
        <img v-if="profile?.avatarUrl" :src="profile.avatarUrl" :alt="profile.authorName" />
        <span v-else aria-hidden="true">见</span>
      </RouterLink>
      <h2>{{ profile?.authorName ?? '博主' }}</h2>
      <span class="accent-line" />
      <p>{{ profile?.authorBio ?? '正在加载站点介绍…' }}</p>
      <div class="social-links">
        <a v-for="link in profile?.socialLinks" :key="link.url" class="social-button" :href="link.url" :aria-label="link.label" target="_blank" rel="me noopener">
          <GitFork v-if="link.icon === 'github'" :size="24" aria-hidden="true" />
          <Mail v-else :size="24" aria-hidden="true" />
        </a>
      </div>
    </section>

    <div class="sticky-widgets">
      <section class="widget card">
        <div class="widget-title">
          <h2>分类</h2>
        </div>
        <div class="widget-content" :class="{ collapsed: categories.length >= 5 && !categoriesExpanded }">
          <RouterLink v-for="category in categories" :key="category.id" class="taxonomy-row" :to="`/categories/${category.slug}`">
            <span>{{ category.name }}</span>
            <small class="count-badge">{{ category.postCount }}</small>
          </RouterLink>
        </div>
        <button v-if="categories.length >= 5 && !categoriesExpanded" class="expand-button" type="button" @click="categoriesExpanded = true"><MoreHorizontal :size="28" /><span>更多</span></button>
      </section>

      <section class="widget card">
        <div class="widget-title">
          <h2>标签</h2>
        </div>
        <div class="widget-content" :class="{ collapsed: tags.length >= 20 && !tagsExpanded }">
          <div class="tag-cloud">
            <RouterLink v-for="tag in tags" :key="tag.id" class="pill" :to="`/tags/${tag.slug}`">{{ tag.name }}</RouterLink>
          </div>
        </div>
        <button v-if="tags.length >= 20 && !tagsExpanded" class="expand-button" type="button" @click="tagsExpanded = true"><MoreHorizontal :size="28" /><span>更多</span></button>
      </section>
    </div>
  </aside>
</template>

<style scoped>
.sidebar {
  display: grid;
  gap: 1rem;
}

.profile-card { padding: 0.75rem; text-align: center; }

.avatar {
  position: relative;
  display: grid;
  width: 100%;
  aspect-ratio: 1;
  overflow: hidden;
  border-radius: 0.75rem;
  color: white;
  background:
    radial-gradient(circle at 72% 24%, oklch(0.88 0.12 calc(var(--hue) + 35)), transparent 28%),
    linear-gradient(145deg, oklch(0.72 0.13 var(--hue)), oklch(0.55 0.13 calc(var(--hue) + 45)));
  max-width: 12rem;
  margin: 0.25rem auto 0.75rem;
  transition: transform 160ms ease, filter 160ms ease;
}

.avatar:hover { transform: scale(0.985); filter: saturate(1.1); }
.avatar-overlay { position: absolute; z-index: 2; inset: 0; display: grid; place-items: center; color: white; background: transparent; opacity: 0; transition: opacity 160ms ease, background-color 160ms ease; }
.avatar:hover .avatar-overlay { background: rgb(0 0 0 / 0.3); opacity: 1; }
.avatar img { width: 100%; height: 100%; object-fit: cover; }
.avatar span { margin: auto; font-family: ui-serif, Georgia, serif; font-size: 4rem; font-weight: 800; }

.profile-card h2 { margin: 0 0 0.25rem; color: var(--text-strong); font-size: 1.25rem; }
.accent-line { display: block; width: 1.4rem; height: 0.25rem; margin: 0.45rem auto 0.65rem; border-radius: 99px; background: var(--primary); }
.profile-card p { margin: 0; padding: 0 0.5rem; color: var(--text-muted); font-size: 0.9rem; }
.social-links { display: flex; flex-wrap: wrap; justify-content: center; gap: 0.5rem; margin: 0.625rem 0 0.25rem; }
.social-button { display: grid; width: 2.5rem; height: 2.5rem; place-items: center; border-radius: 0.5rem; color: var(--button-content); background: var(--button-bg); transition: color 160ms ease, background-color 160ms ease, transform 160ms ease; }
.social-button:hover { background: var(--button-hover); }
.social-button:active { background: var(--button-active); transform: scale(0.9); }

.sticky-widgets { position: sticky; top: 1rem; display: grid; gap: 1rem; }
.widget { padding: 0 1rem 1rem; }
.widget-title { display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.65rem; }
.widget-title h2 { position: relative; margin: 1rem 0 0 1rem; color: var(--text-strong); font-size: 1.125rem; }
.widget-title h2::before { content: ''; position: absolute; top: 0.28rem; left: -1rem; width: 0.25rem; height: 1rem; border-radius: 999px; background: var(--primary); }
.taxonomy-row { display: flex; height: 2.5rem; align-items: center; justify-content: space-between; padding: 0 0.5rem; border-radius: 0.5rem; font-size: 1rem; transition: padding-left 160ms ease, color 160ms ease, background-color 160ms ease; }
.taxonomy-row:hover { padding-left: 0.75rem; color: var(--primary-strong); background: var(--plain-hover); }
.taxonomy-row small { color: var(--text-faint); }
.taxonomy-row .count-badge { display: grid; min-width: 2rem; height: 1.75rem; padding: 0 0.5rem; place-items: center; border-radius: 0.5rem; color: var(--button-content); background: var(--button-bg); font-size: 0.875rem; font-weight: 700; }
:global(:root.dark) .taxonomy-row .count-badge { color: var(--deep-text); background: var(--primary); }
.tag-cloud { display: flex; flex-wrap: wrap; gap: 0.45rem; }
.widget-content { overflow: hidden; }
.widget-content.collapsed { height: 7.5rem; }
.expand-button { display: flex; width: 100%; height: 2.25rem; margin-bottom: -0.5rem; align-items: center; justify-content: center; gap: 0.5rem; border: 0; border-radius: 0.5rem; color: var(--primary); background: transparent; cursor: pointer; }
.expand-button:hover { background: var(--plain-hover); }

@media (max-width: 1023px) {
  .sidebar { grid-template-columns: minmax(14rem, 0.75fr) minmax(0, 1.25fr); }
  .sticky-widgets { position: static; }
  .avatar { max-height: 12rem; }
}

@media (max-width: 700px) {
  .sidebar { grid-template-columns: 1fr; }
}

@media (min-width: 1024px) { .avatar { max-width: none; margin-top: 0; } }
</style>
