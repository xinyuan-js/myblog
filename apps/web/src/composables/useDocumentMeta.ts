import { watchEffect, type MaybeRefOrGetter, toValue } from 'vue'
import { useSite } from '@/composables/useSite'

export function formatDocumentTitle(pageTitle: string, siteTitle: string) {
  const normalizedSiteTitle = siteTitle.trim() || 'MyBlog'
  const normalizedPageTitle = pageTitle.trim()
  return normalizedPageTitle ? `${normalizedPageTitle} · ${normalizedSiteTitle}` : normalizedSiteTitle
}

export function useDocumentMeta(title: MaybeRefOrGetter<string>, description?: MaybeRefOrGetter<string | undefined>) {
  const { profile } = useSite()
  watchEffect(() => {
    const titleValue = toValue(title)
    document.title = formatDocumentTitle(titleValue, profile.value?.title ?? 'MyBlog')
    const explicitDescription = description ? toValue(description) : undefined
    const descriptionValue = explicitDescription?.trim() || profile.value?.description?.trim()
    if (descriptionValue !== undefined) {
      document.querySelector('meta[name="description"]')?.setAttribute('content', descriptionValue)
    }
  })
}
