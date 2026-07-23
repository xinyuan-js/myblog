import { mockCategories, mockPosts, mockSiteProfile, mockTags } from '@/data/mock'
import type {
  ArtalkSession,
  Administrator,
  AuthState,
  AuthUser,
  Category,
  Paginated,
  PostDetail,
  PostMutation,
  PostQuery,
  PostSummary,
  SiteProfile,
  SiteAppearanceMutation,
  Tag,
  TaxonomyMutation,
  MediaItem,
  MediaQuery,
  UploadResult,
} from '@/types/blog'

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    message: string,
    public readonly fieldErrors?: Record<string, string>,
  ) {
    super(message)
  }
}

export interface BlogApi {
  getSiteProfile(): Promise<SiteProfile>
  updateSiteAppearance(input: SiteAppearanceMutation): Promise<SiteProfile>
  listPosts(query?: PostQuery): Promise<Paginated<PostSummary>>
  getPost(slug: string): Promise<PostDetail>
  listTags(): Promise<Tag[]>
  listCategories(): Promise<Category[]>
  listAdminTags(): Promise<Tag[]>
  listAdminCategories(): Promise<Category[]>
  getAuthState(): Promise<AuthState>
  createArtalkSession(): Promise<ArtalkSession>
  listAdministrators(): Promise<Administrator[]>
  addAdministrator(githubId: number): Promise<Administrator>
  removeAdministrator(githubId: number): Promise<void>
  listAdminPosts(query?: PostQuery): Promise<Paginated<PostSummary>>
  getAdminPost(id: number): Promise<PostDetail>
  createPost(input: PostMutation): Promise<PostDetail>
  updatePost(id: number, input: PostMutation): Promise<PostDetail>
  deletePost(id: number): Promise<void>
  createTag(input: TaxonomyMutation): Promise<Tag>
  updateTag(id: number, input: TaxonomyMutation): Promise<Tag>
  deleteTag(id: number): Promise<void>
  createCategory(input: TaxonomyMutation): Promise<Category>
  updateCategory(id: number, input: TaxonomyMutation): Promise<Category>
  deleteCategory(id: number): Promise<void>
  upload(file: File): Promise<UploadResult>
  listUploads(query?: MediaQuery): Promise<Paginated<MediaItem>>
  getUpload(id: number): Promise<MediaItem>
  trashUpload(id: number): Promise<void>
  restoreUpload(id: number): Promise<MediaItem>
  deleteUploadPermanent(id: number): Promise<void>
  logout(): Promise<void>
  createMockSession?(): Promise<AuthState>
}

const wait = (duration = 180) => new Promise((resolve) => globalThis.setTimeout(resolve, duration))
const clone = <T>(value: T): T => structuredClone(value)

class MockBlogApi implements BlogApi {
  private authenticated = false
  private uploads: MediaItem[] = []
  private administrators: Administrator[] = [
    { githubId: 12345678, isOwner: true, grantedAt: null },
  ]

  async getSiteProfile() {
    await wait()
    return clone(mockSiteProfile)
  }

  async updateSiteAppearance(input: SiteAppearanceMutation) {
    await wait()
    Object.assign(mockSiteProfile, input)
    return clone(mockSiteProfile)
  }

  async listPosts(query: PostQuery = {}) {
    await wait()
    const page = query.page ?? 1
    const pageSize = query.pageSize ?? 10
    let posts = mockPosts
      .filter((post) =>
        (post.status === 'published' || post.status === 'scheduled') &&
        post.publishedAt !== null &&
        new Date(post.publishedAt) <= new Date(),
      )
      .map((post) => ({ ...post, status: 'published' as const }))
    if (query.tag) posts = posts.filter((post) => post.tags.some((tag) => tag.slug === query.tag))
    if (query.category) posts = posts.filter((post) => post.category?.slug === query.category)
    if (query.q) {
      const keyword = query.q.trim().toLocaleLowerCase()
      posts = posts.filter((post) =>
        `${post.title}\n${post.excerpt}\n${post.contentMarkdown}`.toLocaleLowerCase().includes(keyword),
      )
    }
    const start = (page - 1) * pageSize
    return {
      items: clone(posts.slice(start, start + pageSize)),
      pagination: {
        page,
        pageSize,
        total: posts.length,
        totalPages: Math.max(1, Math.ceil(posts.length / pageSize)),
      },
    }
  }

  async getPost(slug: string) {
    await wait()
    const post = mockPosts.find((item) => item.slug === slug && item.status === 'published')
    if (!post) throw new ApiError(404, 'POST_NOT_FOUND', '文章不存在或尚未发布')
    return clone(post)
  }

  async listTags() {
    await wait()
    return clone(mockTags)
  }

  async listCategories() {
    await wait()
    return clone(mockCategories)
  }

  async listAdminTags() { await wait(); return clone(mockTags) }
  async listAdminCategories() { await wait(); return clone(mockCategories) }

  async getAuthState(): Promise<AuthState> {
    await wait(80)
    if (!this.authenticated) {
      return { authenticated: false, csrfToken: null, user: null }
    }
    return {
      authenticated: true,
      csrfToken: 'mock-csrf-token',
      user: {
        githubId: 12345678,
        login: 'demo-admin',
        name: '见山',
        avatarUrl: 'https://avatars.githubusercontent.com/u/583231?v=4',
        isAdmin: true,
        isOwner: true,
      },
    }
  }

  async createArtalkSession(): Promise<ArtalkSession> {
    await wait(80)
    if (!this.authenticated) throw new ApiError(401, 'AUTH_REQUIRED', '请先登录')
    return {
      token: 'mock-artalk-token',
      user: { id: 1, name: '见山', email: 'demo@example.com', is_admin: true },
    }
  }

  async listAdministrators() {
    await wait()
    return clone(this.administrators)
  }

  async addAdministrator(githubId: number) {
    await wait()
    const existing = this.administrators.find((item) => item.githubId === githubId)
    if (existing) return clone(existing)
    const added: Administrator = { githubId, isOwner: false, grantedAt: new Date().toISOString() }
    this.administrators.push(added)
    return clone(added)
  }

  async removeAdministrator(githubId: number) {
    await wait()
    const index = this.administrators.findIndex((item) => item.githubId === githubId && !item.isOwner)
    if (index < 0) throw new ApiError(404, 'ADMINISTRATOR_NOT_FOUND', '管理员不存在')
    this.administrators.splice(index, 1)
  }

  async listAdminPosts(query: PostQuery = {}) {
    await wait()
    const page = query.page ?? 1
    const pageSize = query.pageSize ?? 20
    const posts = query.status ? mockPosts.filter((post) => post.status === query.status) : mockPosts
    return {
      items: clone(posts.slice((page - 1) * pageSize, page * pageSize)),
      pagination: {
        page,
        pageSize,
        total: posts.length,
        totalPages: Math.max(1, Math.ceil(posts.length / pageSize)),
      },
    }
  }

  async getAdminPost(id: number) {
    await wait()
    const post = mockPosts.find((item) => item.id === id)
    if (!post) throw new ApiError(404, 'POST_NOT_FOUND', '文章不存在')
    return clone(post)
  }

  async createPost(input: PostMutation) {
    await wait()
    if (mockPosts.some((post) => post.slug === input.slug)) {
      throw new ApiError(409, 'SLUG_CONFLICT', 'Slug 已被使用', { slug: 'Slug 已被使用' })
    }
    const created = this.fromMutation(Math.max(...mockPosts.map((post) => post.id)) + 1, input)
    mockPosts.unshift(created)
    return clone(created)
  }

  async updatePost(id: number, input: PostMutation) {
    await wait()
    const index = mockPosts.findIndex((post) => post.id === id)
    if (index < 0) throw new ApiError(404, 'POST_NOT_FOUND', '文章不存在')
    const existing = mockPosts[index]!
    const wasPublished = existing.status === 'published' && existing.publishedAt !== null && new Date(existing.publishedAt) <= new Date()
    if (wasPublished && input.slug !== existing.slug) {
      throw new ApiError(409, 'POST_SLUG_LOCKED', '文章发布后不能修改 Slug')
    }
    if (mockPosts.some((post) => post.id !== id && post.slug === input.slug)) {
      throw new ApiError(409, 'SLUG_CONFLICT', 'Slug 已被使用', { slug: 'Slug 已被使用' })
    }
    const updated = this.fromMutation(id, input, existing)
    mockPosts[index] = updated
    return clone(updated)
  }

  async deletePost(id: number) {
    await wait()
    const index = mockPosts.findIndex((post) => post.id === id)
    if (index < 0) throw new ApiError(404, 'POST_NOT_FOUND', '文章不存在')
    mockPosts.splice(index, 1)
  }

  async createTag(input: TaxonomyMutation) {
    await wait()
    const tag = { id: Math.max(...mockTags.map((item) => item.id)) + 1, ...input, postCount: 0 }
    mockTags.push(tag)
    return clone(tag)
  }

  async updateTag(id: number, input: TaxonomyMutation) {
    await wait()
    const tag = mockTags.find((item) => item.id === id)
    if (!tag) throw new ApiError(404, 'TAG_NOT_FOUND', '标签不存在')
    Object.assign(tag, input)
    return clone(tag)
  }

  async deleteTag(id: number) {
    await wait()
    const index = mockTags.findIndex((item) => item.id === id)
    if (index < 0) throw new ApiError(404, 'TAG_NOT_FOUND', '标签不存在')
    if (mockPosts.some((post) => post.tags.some((tag) => tag.id === id))) {
      throw new ApiError(409, 'TAXONOMY_IN_USE', '标签仍被文章使用')
    }
    mockTags.splice(index, 1)
  }

  async createCategory(input: TaxonomyMutation) {
    await wait()
    const category = {
      id: Math.max(...mockCategories.map((item) => item.id)) + 1,
      name: input.name,
      slug: input.slug,
      description: input.description ?? null,
      postCount: 0,
    }
    mockCategories.push(category)
    return clone(category)
  }

  async updateCategory(id: number, input: TaxonomyMutation) {
    await wait()
    const category = mockCategories.find((item) => item.id === id)
    if (!category) throw new ApiError(404, 'CATEGORY_NOT_FOUND', '分类不存在')
    Object.assign(category, input)
    return clone(category)
  }

  async deleteCategory(id: number) {
    await wait()
    const index = mockCategories.findIndex((item) => item.id === id)
    if (index < 0) throw new ApiError(404, 'CATEGORY_NOT_FOUND', '分类不存在')
    if (mockPosts.some((post) => post.category?.id === id)) {
      throw new ApiError(409, 'TAXONOMY_IN_USE', '分类仍被文章使用')
    }
    mockCategories.splice(index, 1)
  }

  async upload(file: File) {
    await wait(500)
    const item: MediaItem = {
      id: Date.now(),
      url: URL.createObjectURL(file),
      filename: file.name,
      contentType: file.type,
      size: file.size,
      width: 1,
      height: 1,
      status: 'active',
      usageCount: 0,
      references: [],
      createdAt: new Date().toISOString(),
      trashedAt: null,
    }
    this.uploads.unshift(item)
    return clone(item)
  }

  async listUploads(query: MediaQuery = {}) {
    await wait()
    const page = query.page ?? 1
    const pageSize = query.pageSize ?? 20
    let items = this.uploads.filter((item) => item.status === (query.status ?? 'active'))
    if (query.usage === 'used') items = items.filter((item) => item.usageCount > 0)
    if (query.usage === 'unused') items = items.filter((item) => item.usageCount === 0)
    if (query.q) items = items.filter((item) => item.filename.toLowerCase().includes(query.q!.toLowerCase()))
    return { items: clone(items.slice((page - 1) * pageSize, page * pageSize)), pagination: { page, pageSize, total: items.length, totalPages: Math.max(1, Math.ceil(items.length / pageSize)) } }
  }
  async getUpload(id: number) { const item = this.uploads.find((value) => value.id === id); if (!item) throw new ApiError(404, 'UPLOAD_NOT_FOUND', '媒体不存在'); return clone(item) }
  async trashUpload(id: number) { const item = this.uploads.find((value) => value.id === id); if (!item) throw new ApiError(404, 'UPLOAD_NOT_FOUND', '媒体不存在'); if (item.usageCount) throw new ApiError(409, 'UPLOAD_IN_USE', '媒体正在使用'); item.status = 'trashed'; item.trashedAt = new Date().toISOString() }
  async restoreUpload(id: number) { const item = this.uploads.find((value) => value.id === id); if (!item) throw new ApiError(404, 'UPLOAD_NOT_FOUND', '媒体不存在'); item.status = 'active'; item.trashedAt = null; return clone(item) }
  async deleteUploadPermanent(id: number) { const index = this.uploads.findIndex((value) => value.id === id && value.status === 'trashed'); if (index < 0) throw new ApiError(409, 'UPLOAD_STATE_INVALID', '媒体状态不允许删除'); this.uploads.splice(index, 1) }

  async logout() {
    await wait(80)
    this.authenticated = false
  }

  async createMockSession(): Promise<AuthState> {
    await wait(120)
    this.authenticated = true
    return this.getAuthState()
  }

  private fromMutation(id: number, input: PostMutation, existing?: PostDetail): PostDetail {
    const contentText = input.contentMarkdown.replace(/[#*`>\[\]()_-]/g, ' ')
    const wordCount = contentText.replace(/\s+/g, '').length
    return {
      id,
      ...input,
      category: mockCategories.find((item) => item.id === input.categoryId) ?? null,
      tags: mockTags.filter((item) => input.tagIds.includes(item.id)),
      updatedAt: new Date().toISOString(),
      wordCount,
      readingTimeMinutes: Math.max(1, Math.ceil(wordCount / 300)),
      previousPost: existing?.previousPost ?? null,
      nextPost: existing?.nextPost ?? null,
    }
  }
}

class HttpBlogApi implements BlogApi {
  private readonly baseUrl = import.meta.env.VITE_API_BASE_URL || '/api'
  private csrfToken: string | null = null

  getSiteProfile = () => this.request<SiteProfile>('/site')
  updateSiteAppearance = (input: SiteAppearanceMutation) =>
    this.request<SiteProfile>('/admin/site/appearance', { method: 'PUT', body: input })
  listPosts = (query: PostQuery = {}) => this.request<Paginated<PostSummary>>(`/posts${this.query(query)}`)
  getPost = (slug: string) => this.request<PostDetail>(`/posts/${encodeURIComponent(slug)}`)
  listTags = () => this.request<Tag[]>('/tags', { cache: 'no-cache' })
  listCategories = () => this.request<Category[]>('/categories', { cache: 'no-cache' })
  listAdminTags = () => this.request<Tag[]>('/admin/tags')
  listAdminCategories = () => this.request<Category[]>('/admin/categories')
  async getAuthState() {
    const state = await this.request<AuthState>('/auth/me')
    this.csrfToken = state.csrfToken
    return state
  }
  createArtalkSession = () => this.request<ArtalkSession>('/auth/artalk/session', { method: 'POST' })
  listAdministrators = () => this.request<Administrator[]>('/admin/administrators')
  addAdministrator = (githubId: number) =>
    this.request<Administrator>('/admin/administrators', { method: 'POST', body: { githubId } })
  removeAdministrator = (githubId: number) =>
    this.request<void>(`/admin/administrators/${githubId}`, { method: 'DELETE' })
  listAdminPosts = (query: PostQuery = {}) =>
    this.request<Paginated<PostSummary>>(`/admin/posts${this.query(query)}`)
  getAdminPost = (id: number) => this.request<PostDetail>(`/admin/posts/${id}`)
  createPost = (input: PostMutation) => this.request<PostDetail>('/admin/posts', { method: 'POST', body: input })
  updatePost = (id: number, input: PostMutation) =>
    this.request<PostDetail>(`/admin/posts/${id}`, { method: 'PUT', body: input })
  deletePost = (id: number) => this.request<void>(`/admin/posts/${id}`, { method: 'DELETE' })
  createTag = (input: TaxonomyMutation) => this.request<Tag>('/admin/tags', { method: 'POST', body: input })
  updateTag = (id: number, input: TaxonomyMutation) =>
    this.request<Tag>(`/admin/tags/${id}`, { method: 'PUT', body: input })
  deleteTag = (id: number) => this.request<void>(`/admin/tags/${id}`, { method: 'DELETE' })
  createCategory = (input: TaxonomyMutation) =>
    this.request<Category>('/admin/categories', { method: 'POST', body: input })
  updateCategory = (id: number, input: TaxonomyMutation) =>
    this.request<Category>(`/admin/categories/${id}`, { method: 'PUT', body: input })
  deleteCategory = (id: number) => this.request<void>(`/admin/categories/${id}`, { method: 'DELETE' })
  async logout() {
    try {
      await this.request<void>('/auth/logout', { method: 'POST' })
    } finally {
      this.csrfToken = null
    }
  }

  async upload(file: File) {
    const form = new FormData()
    form.set('file', file)
    return this.request<UploadResult>('/admin/uploads', { method: 'POST', form })
  }
  listUploads = (query: MediaQuery = {}) => this.request<Paginated<MediaItem>>(`/admin/uploads${this.query(query)}`)
  getUpload = (id: number) => this.request<MediaItem>(`/admin/uploads/${id}`)
  trashUpload = (id: number) => this.request<void>(`/admin/uploads/${id}`, { method: 'DELETE' })
  restoreUpload = (id: number) => this.request<MediaItem>(`/admin/uploads/${id}/restore`, { method: 'POST' })
  deleteUploadPermanent = (id: number) => this.request<void>(`/admin/uploads/${id}/permanent`, { method: 'DELETE' })

  private query(query: PostQuery | MediaQuery) {
    const params = new URLSearchParams()
    Object.entries(query).forEach(([key, value]) => {
      if (value !== undefined && value !== '') params.set(key, String(value))
    })
    const value = params.toString()
    return value ? `?${value}` : ''
  }

  private async request<T>(
    path: string,
    options: { method?: string; body?: unknown; form?: FormData; cache?: RequestCache } = {},
  ): Promise<T> {
    const headers = new Headers({ Accept: 'application/json' })
    if (options.body !== undefined) headers.set('Content-Type', 'application/json')
    if (options.method && !['GET', 'HEAD', 'OPTIONS'].includes(options.method) && this.csrfToken) {
      headers.set('X-CSRF-Token', this.csrfToken)
    }
    const response = await fetch(`${this.baseUrl}${path}`, {
      method: options.method ?? 'GET',
      credentials: 'include',
      cache: options.cache,
      headers,
      body: options.form ?? (options.body === undefined ? undefined : JSON.stringify(options.body)),
    })

    if (!response.ok) {
      const payload = await response.json().catch(() => ({ code: 'UNKNOWN_ERROR', message: '请求失败' }))
      throw new ApiError(response.status, payload.code, payload.message, payload.fieldErrors)
    }
    if (response.status === 204) return undefined as T
    const payload = await response.json()
    return payload.data as T
  }
}

export function isMockApiEnabled(value: string | undefined) {
  return value === 'true'
}

export const mockApiEnabled = isMockApiEnabled(import.meta.env.VITE_USE_MOCK_API)
export const api: BlogApi = mockApiEnabled ? new MockBlogApi() : new HttpBlogApi()

export function sanitizeReturnTo(value: unknown, fallback = '/') {
  if (
    typeof value !== 'string' ||
    !value.startsWith('/') ||
    value.startsWith('//') ||
    value.startsWith('/\\') ||
    value.includes('\r') ||
    value.includes('\n') ||
    value.startsWith('/login') ||
    value.startsWith('/admin/login')
  ) {
    return fallback
  }
  return value
}

export function sanitizeAdminReturnTo(value: unknown) {
  const safe = sanitizeReturnTo(value, '/admin')
  if (!/^\/admin(?:\/|$|\?)/.test(safe)) {
    return '/admin'
  }
  return safe
}

export function githubLoginUrl(returnTo: unknown = '/') {
  const safeReturnTo = sanitizeReturnTo(returnTo)
  if (mockApiEnabled) return safeReturnTo
  const base = import.meta.env.VITE_API_BASE_URL || '/api'
  return `${base}/auth/github?return_to=${encodeURIComponent(safeReturnTo)}`
}

export type { AuthUser }
