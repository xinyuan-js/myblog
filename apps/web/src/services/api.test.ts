import { describe, expect, it } from 'vitest'
import { ApiError, api, isMockApiEnabled, sanitizeAdminReturnTo, sanitizeReturnTo } from './api'

describe('mock blog api', () => {
  it('uses the real API unless mock mode is explicitly enabled', () => {
    expect(isMockApiEnabled(undefined)).toBe(false)
    expect(isMockApiEnabled('false')).toBe(false)
    expect(isMockApiEnabled('true')).toBe(true)
  })

  it('public list never exposes drafts', async () => {
    const result = await api.listPosts({ pageSize: 100 })
    expect(result.items.length).toBeGreaterThan(0)
    expect(result.items.every((post) => post.status === 'published')).toBe(true)
  })

  it('filters public posts by tag slug', async () => {
    const result = await api.listPosts({ tag: 'database', pageSize: 100 })
    expect(result.items.length).toBeGreaterThan(0)
    expect(result.items.every((post) => post.tags.some((tag) => tag.slug === 'database'))).toBe(true)
  })

  it('searches published post content', async () => {
    const result = await api.listPosts({ q: 'MySQL', pageSize: 100 })
    expect(result.items.length).toBeGreaterThan(0)
    expect(result.items.every((post) =>
      `${post.title}\n${post.excerpt}`.toLocaleLowerCase().includes('mysql'),
    )).toBe(true)
  })

  it('paginates public posts without duplicates', async () => {
    const first = await api.listPosts({ page: 1, pageSize: 2 })
    const second = await api.listPosts({ page: 2, pageSize: 2 })
    expect(first.pagination).toMatchObject({ page: 1, pageSize: 2 })
    expect(second.pagination).toMatchObject({ page: 2, pageSize: 2 })
    expect(first.pagination.total).toBe(second.pagination.total)
    expect(new Set([...first.items, ...second.items].map((post) => post.id)).size).toBe(first.items.length + second.items.length)
  })

  it('returns stable not-found errors', async () => {
    await expect(api.getPost('missing-post')).rejects.toMatchObject({
      status: 404,
      code: 'POST_NOT_FOUND',
    } satisfies Partial<ApiError>)
  })

  it('admin list includes non-public posts', async () => {
    const result = await api.listAdminPosts({ pageSize: 100 })
    expect(result.items.some((post) => post.status === 'draft')).toBe(true)
  })

  it('updates the public site appearance', async () => {
    const original = await api.getSiteProfile()
    const updated = await api.updateSiteAppearance({
      ...original,
      avatarUrl: '/uploads/test-avatar.webp',
      bannerUrl: '/uploads/test-background.webp',
    })
    expect(updated.avatarUrl).toBe('/uploads/test-avatar.webp')
    expect(updated.bannerUrl).toBe('/uploads/test-background.webp')
    expect(await api.getSiteProfile()).toMatchObject({
      avatarUrl: '/uploads/test-avatar.webp',
      bannerUrl: '/uploads/test-background.webp',
    })
    await api.updateSiteAppearance({
      ...original,
    })
  })

  it('mock login and logout update the authenticated session', async () => {
    await api.logout()
    expect(await api.getAuthState()).toMatchObject({ authenticated: false, user: null, csrfToken: null })
    expect(api.createMockSession).toBeTypeOf('function')
    const result = await api.createMockSession!()
    expect(result.authenticated).toBe(true)
    expect(result.csrfToken).toBeTruthy()
    await api.logout()
    expect((await api.getAuthState()).authenticated).toBe(false)
  })

  it('only accepts local admin return paths', () => {
    expect(sanitizeAdminReturnTo('/admin/posts/1/edit?tab=content')).toBe('/admin/posts/1/edit?tab=content')
    expect(sanitizeAdminReturnTo('https://evil.example/admin')).toBe('/admin')
    expect(sanitizeAdminReturnTo('//evil.example/admin')).toBe('/admin')
    expect(sanitizeAdminReturnTo('/admin/login')).toBe('/admin')
    expect(sanitizeAdminReturnTo('/posts/public')).toBe('/admin')
  })

  it('accepts safe public return paths for unified login', () => {
    expect(sanitizeReturnTo('/posts/hello?reply=1')).toBe('/posts/hello?reply=1')
    expect(sanitizeReturnTo('/about')).toBe('/about')
    expect(sanitizeReturnTo('//evil.example/posts')).toBe('/')
    expect(sanitizeReturnTo('https://evil.example/posts')).toBe('/')
    expect(sanitizeReturnTo('/login?returnTo=/admin')).toBe('/')
  })

  it('rejects changing the slug of an already published post', async () => {
    const post = await api.getAdminPost(1)
    await expect(api.updatePost(post.id, {
      title: post.title,
      slug: 'changed-after-publish',
      excerpt: post.excerpt,
      contentMarkdown: post.contentMarkdown,
      coverUrl: post.coverUrl,
      status: post.status,
      publishedAt: post.publishedAt,
      categoryId: post.category?.id ?? null,
      tagIds: post.tags.map((tag) => tag.id),
    })).rejects.toMatchObject({ status: 409, code: 'POST_SLUG_LOCKED' } satisfies Partial<ApiError>)
  })

  it('does not expose a future scheduled post publicly', async () => {
    const created = await api.createPost({
      title: '未来文章',
      slug: 'future-scheduled-test',
      excerpt: '只用于契约测试',
      contentMarkdown: '# 未来文章',
      coverUrl: null,
      status: 'scheduled',
      publishedAt: '2099-01-01T00:00:00Z',
      categoryId: null,
      tagIds: [],
    })
    const publicPosts = await api.listPosts({ pageSize: 100 })
    expect(publicPosts.items.some((post) => post.id === created.id)).toBe(false)
    await api.deletePost(created.id)
  })
})
