import { describe, expect, it } from 'vitest'
import { createPaginationItems, isCanonicalPageQuery, parsePageQuery, withPageQuery } from './pagination'

describe('pagination helpers', () => {
  it('accepts only positive safe integer pages', () => {
    expect(parsePageQuery(undefined)).toBe(1)
    expect(parsePageQuery('2')).toBe(2)
    expect(parsePageQuery(['3', '4'])).toBe(3)
    expect(parsePageQuery('1.5')).toBe(1)
    expect(parsePageQuery('-2')).toBe(1)
    expect(parsePageQuery('Infinity')).toBe(1)
    expect(parsePageQuery('not-a-number')).toBe(1)
  })

  it('uses a canonical URL without page=1', () => {
    expect(isCanonicalPageQuery(undefined, 1)).toBe(true)
    expect(isCanonicalPageQuery('1', 1)).toBe(false)
    expect(isCanonicalPageQuery('2', 2)).toBe(true)
    expect(withPageQuery({ tag: 'go', page: '2' }, 1)).toEqual({ tag: 'go' })
    expect(withPageQuery({ tag: 'go' }, 3)).toEqual({ tag: 'go', page: '3' })
  })

  it('keeps the pagination bar compact at every boundary', () => {
    expect(createPaginationItems(3, 1)).toEqual([1, 2, 3])
    expect(createPaginationItems(10, 1)).toEqual([1, 2, 3, 'ellipsis', 10])
    expect(createPaginationItems(10, 5)).toEqual([1, 'ellipsis', 5, 'ellipsis', 10])
    expect(createPaginationItems(10, 10)).toEqual([1, 'ellipsis', 8, 9, 10])
  })
})
