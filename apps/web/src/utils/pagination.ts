export type PaginationItem = number | 'ellipsis'

export function parsePageQuery(value: unknown): number {
  const raw = Array.isArray(value) ? value[0] : value
  if (typeof raw !== 'string' && typeof raw !== 'number') return 1
  const page = Number(raw)
  return Number.isSafeInteger(page) && page >= 1 ? page : 1
}

export function isCanonicalPageQuery(value: unknown, page: number): boolean {
  if (page === 1) return value === undefined
  return typeof value === 'string' && value === String(page)
}

export function withPageQuery(query: LocationQuery | LocationQueryRaw, page: number): LocationQueryRaw {
  const result: LocationQueryRaw = { ...query }
  if (page <= 1) delete result.page
  else result.page = String(page)
  return result
}

export function createPaginationItems(totalPages: number, currentPage: number): PaginationItem[] {
  const total = Math.max(1, Math.floor(totalPages))
  const current = Math.min(total, Math.max(1, Math.floor(currentPage)))
  if (total <= 5) return Array.from({ length: total }, (_, index) => index + 1)
  if (current <= 3) return [1, 2, 3, 'ellipsis', total]
  if (current >= total - 2) return [1, 'ellipsis', total - 2, total - 1, total]
  return [1, 'ellipsis', current, 'ellipsis', total]
}
import type { LocationQuery, LocationQueryRaw } from 'vue-router'
