<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useSite } from '@/composables/useSite'
import BackToTop from './BackToTop.vue'
import SiteFooter from './SiteFooter.vue'
import SiteNavbar from './SiteNavbar.vue'
import SiteSidebar from './SiteSidebar.vue'

const { profile, tags, categories, loaded, error: siteError, loadSite } = useSite()
const route = useRoute()
const isHome = computed(() => route.name === 'home')
const bannerEnabled = computed(() => Boolean(profile.value?.bannerUrl) || (!loaded.value && !siteError.value))
const bannerReady = ref(false)
const navbarHidden = ref(false)
let scrollFrame = 0

function updateScrollState() {
  scrollFrame = 0
  if (!bannerEnabled.value) {
    navbarHidden.value = false
    return
  }
  const useTallHomeBanner = isHome.value && window.innerWidth >= 1024
  const bannerHeight = window.innerHeight * (useTallHomeBanner ? 0.65 : 0.35)
  const threshold = Math.max(0, bannerHeight - 72 - 56 - 16)
  navbarHidden.value = window.scrollY >= threshold
}

function scheduleScrollUpdate() {
  if (!scrollFrame) scrollFrame = window.requestAnimationFrame(updateScrollState)
}

watch(() => profile.value?.bannerUrl, () => { bannerReady.value = false })
watch([isHome, bannerEnabled], async () => {
  await nextTick()
  updateScrollState()
})
onMounted(() => {
  loadSite()
  updateScrollState()
  window.addEventListener('scroll', scheduleScrollUpdate, { passive: true })
  window.addEventListener('resize', scheduleScrollUpdate)
})
onBeforeUnmount(() => {
  window.removeEventListener('scroll', scheduleScrollUpdate)
  window.removeEventListener('resize', scheduleScrollUpdate)
  if (scrollFrame) window.cancelAnimationFrame(scrollFrame)
})
</script>

<template>
  <div class="public-layout" :class="{ 'is-home': isHome, 'with-banner': bannerEnabled }">
    <div class="navbar-region">
      <div class="navbar-wrapper" :class="{ hidden: navbarHidden }">
        <SiteNavbar :title="profile?.title ?? '浮光'" />
      </div>
    </div>

    <div v-if="bannerEnabled" class="banner-viewport" :class="{ ready: bannerReady }" aria-hidden="true">
      <img v-if="profile?.bannerUrl" :src="profile.bannerUrl" alt="" @load="bannerReady = true" />
    </div>

    <div class="main-anchor page-shell">
      <div class="main-grid">
        <div class="layout-sidebar onload-sidebar">
          <SiteSidebar :profile="profile" :tags="tags" :categories="categories" />
        </div>
        <div class="content-stack">
          <slot />
        </div>
        <SiteFooter class="onload-footer" :author-name="profile?.authorName ?? '见山'" :icp-number="profile?.icpNumber" />
      </div>
      <BackToTop />
    </div>
  </div>
</template>

<style scoped>
.public-layout { --public-banner-height: 35vh; position: relative; min-height: 100vh; }
.navbar-region { position: relative; z-index: 50; height: 4.5rem; pointer-events: none; }
.with-banner .navbar-region { position: absolute; top: 0; right: 0; left: 0; height: calc(65vh - 4.5rem); }
.navbar-wrapper { position: sticky; top: 0; pointer-events: auto; transition: opacity 300ms ease, transform 300ms ease; }
.navbar-wrapper.hidden { opacity: 0; transform: translateY(-4rem); pointer-events: none; }

.banner-viewport {
  position: relative;
  z-index: 10;
  width: 100%;
  height: var(--public-banner-height);
  overflow: hidden;
  background: var(--button-bg);
  transition: height 700ms ease;
}
.banner-viewport img { width: 100%; height: 65vh; object-fit: cover; object-position: center; opacity: 0; transform: translateY(-15vh) scale(1.05); transition: opacity 700ms ease, transform 700ms ease; }
.banner-viewport.ready img { opacity: 1; transform: translateY(-15vh) scale(1); }

.main-anchor { position: relative; z-index: 30; }
.with-banner .main-anchor { margin-top: -3.5rem; }
.main-grid { width: 100%; padding-top: 0; }
.public-layout:not(.with-banner) .main-grid { padding-top: 1rem; }
.onload-sidebar { animation: fade-in-up 300ms 100ms both; }
.content-stack { animation: fade-in-up 300ms 150ms both; }
.onload-footer { animation: fade-in-up 300ms 250ms both; }
.main-grid > :deep(.site-footer) { grid-column: 2; }

@media (min-width: 1024px) {
  .public-layout.is-home { --public-banner-height: 65vh; }
  .public-layout.is-home .banner-viewport img { transform: translateY(0) scale(1.05); }
  .public-layout.is-home .banner-viewport.ready img { transform: translateY(0) scale(1); }
}

@media (max-width: 1023px) {
  .main-grid > :deep(.site-footer) {
    grid-column: 1;
    order: 3;
  }
}
@keyframes fade-in-up { from { opacity: 0; transform: translateY(2rem); } to { opacity: 1; transform: translateY(0); } }
</style>
