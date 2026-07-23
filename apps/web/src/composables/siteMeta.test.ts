import { describe, expect, it } from 'vitest'
import { formatDocumentTitle } from './useDocumentMeta'
import { backgroundRefreshDue, backgroundRefreshIntervalMs } from './useSite'

describe('site metadata and background refresh policy', () => {
  it('uses the configured site title in document titles', () => {
    expect(formatDocumentTitle('', 'xinyuan')).toBe('xinyuan')
    expect(formatDocumentTitle('文章', 'xinyuan')).toBe('文章 · xinyuan')
    expect(formatDocumentTitle('文章', '   ')).toBe('文章 · MyBlog')
  })

  it('throttles focus refreshes but tolerates clock changes', () => {
    const lastRefresh = 10_000
    expect(backgroundRefreshDue(0, lastRefresh)).toBe(true)
    expect(backgroundRefreshDue(lastRefresh, lastRefresh + backgroundRefreshIntervalMs - 1)).toBe(false)
    expect(backgroundRefreshDue(lastRefresh, lastRefresh + backgroundRefreshIntervalMs)).toBe(true)
    expect(backgroundRefreshDue(lastRefresh, lastRefresh - 1)).toBe(true)
  })
})
