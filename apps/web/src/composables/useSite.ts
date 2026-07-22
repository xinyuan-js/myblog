import { ref } from 'vue'
import { api } from '@/services/api'
import type { Category, SiteProfile, Tag } from '@/types/blog'
import { sanitizeSiteProfile } from '@/utils/externalUrl'

const profile = ref<SiteProfile | null>(null)
const tags = ref<Tag[]>([])
const categories = ref<Category[]>([])
const loading = ref(false)
const loaded = ref(false)
const error = ref<string | null>(null)

export function useSite() {
  function setSiteProfile(value: SiteProfile) {
    profile.value = sanitizeSiteProfile(value)
    loaded.value = true
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
      loaded.value = true
    } catch (cause) {
      error.value = cause instanceof Error ? cause.message : '站点信息加载失败'
    } finally {
      loading.value = false
    }
  }

  return { profile, tags, categories, loading, loaded, error, loadSite, setSiteProfile }
}
