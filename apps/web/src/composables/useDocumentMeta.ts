import { watchEffect, type MaybeRefOrGetter, toValue } from 'vue'

export function useDocumentMeta(title: MaybeRefOrGetter<string>, description?: MaybeRefOrGetter<string | undefined>) {
  watchEffect(() => {
    const titleValue = toValue(title)
    document.title = titleValue ? `${titleValue} · MyBlog` : 'MyBlog'
    const descriptionValue = description ? toValue(description) : undefined
    if (descriptionValue) {
      document.querySelector('meta[name="description"]')?.setAttribute('content', descriptionValue)
    }
  })
}
