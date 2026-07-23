import { ref } from 'vue'
import { api } from '@/services/api'
import type { Category, SiteProfile, Tag } from '@/types/blog'
import { sanitizeSiteProfile } from '@/utils/externalUrl'

export const siteProfileUpdatedStorageKey = 'myblog:site-profile-updated'
export const siteTaxonomiesUpdatedStorageKey = 'myblog:site-taxonomies-updated'

const profile = ref<SiteProfile | null>(null)
const tags = ref<Tag[]>([])
const categories = ref<Category[]>([])
const loading = ref(false)
const loaded = ref(false)
const error = ref<string | null>(null)
let refreshingProfile: Promise<void> | null = null
let refreshingTaxonomies: Promise<void> | null = null
let profileRefreshedAt = 0
let taxonomiesRefreshedAt = 0

export const backgroundRefreshIntervalMs = 30_000

export function backgroundRefreshDue(lastRefresh: number, now = Date.now()) {
  return lastRefresh <= 0 || now < lastRefresh || now - lastRefresh >= backgroundRefreshIntervalMs
}

export function useSite() {
  function setSiteProfile(value: SiteProfile) {
    profile.value = sanitizeSiteProfile(value)
    loaded.value = true
    profileRefreshedAt = Date.now()
  }

  async function refreshSiteProfile(options: { force?: boolean } = {}) {
    if (refreshingProfile) return refreshingProfile
    if (options.force === false && !backgroundRefreshDue(profileRefreshedAt)) return
    refreshingProfile = (async () => {
      try {
        setSiteProfile(await api.getSiteProfile())
        error.value = null
      } catch (cause) {
        error.value = cause instanceof Error ? cause.message : '站点信息加载失败'
      } finally {
        refreshingProfile = null
      }
    })()
    return refreshingProfile
  }

  async function refreshTaxonomies(options: { force?: boolean } = {}) {
    if (refreshingTaxonomies) return refreshingTaxonomies
    if (options.force === false && !backgroundRefreshDue(taxonomiesRefreshedAt)) return
    refreshingTaxonomies = (async () => {
      try {
        const [tagsValue, categoriesValue] = await Promise.all([api.listTags(), api.listCategories()])
        tags.value = tagsValue
        categories.value = categoriesValue
        taxonomiesRefreshedAt = Date.now()
        error.value = null
      } catch (cause) {
        error.value = cause instanceof Error ? cause.message : '分类与标签加载失败'
      } finally {
        refreshingTaxonomies = null
      }
    })()
    return refreshingTaxonomies
  }

  async function loadSite() {
    if (loaded.value || loading.value) return
    loading.value = true
    error.value = null
    try {
      const [profileValue, tagsValue, categoriesValue] = await Promise.all([
        api.getSiteProfile(),
        api.listTags(),
        api.listCategories(),
      ])
      profile.value = sanitizeSiteProfile(profileValue)
      tags.value = tagsValue
      categories.value = categoriesValue
      const refreshedAt = Date.now()
      profileRefreshedAt = refreshedAt
      taxonomiesRefreshedAt = refreshedAt
      loaded.value = true
    } catch (cause) {
      error.value = cause instanceof Error ? cause.message : '站点信息加载失败'
    } finally {
      loading.value = false
    }
  }

  return { profile, tags, categories, loading, loaded, error, loadSite, refreshSiteProfile, refreshTaxonomies, setSiteProfile }
}
