export type ISODateTime = string

export interface SiteProfile {
  title: string
  subtitle: string
  description: string
  avatarUrl: string | null
  bannerUrl: string | null
  authorName: string
  authorBio: string
  socialLinks: Array<{
    label: string
    url: string
    icon: 'github' | 'mail' | 'rss' | 'link'
  }>
  icpNumber: string | null
}

export interface SiteAppearanceMutation {
  avatarUrl: string | null
  bannerUrl: string | null
}

export interface Tag {
  id: number
  name: string
  slug: string
  postCount: number
}

export interface Category {
  id: number
  name: string
  slug: string
  description: string | null
  postCount: number
}

export type PostStatus = 'draft' | 'published' | 'scheduled'

export interface PostSummary {
  id: number
  title: string
  slug: string
  excerpt: string
  coverUrl: string | null
  status: PostStatus
  publishedAt: ISODateTime | null
  updatedAt: ISODateTime
  category: Category | null
  tags: Tag[]
  wordCount: number
  readingTimeMinutes: number
}

export interface HeadingItem {
  id: string
  text: string
  level: 2 | 3
}

export interface PostDetail extends PostSummary {
  contentMarkdown: string
  previousPost: Pick<PostSummary, 'title' | 'slug'> | null
  nextPost: Pick<PostSummary, 'title' | 'slug'> | null
}

export interface PaginationMeta {
  page: number
  pageSize: number
  total: number
  totalPages: number
}

export interface Paginated<T> {
  items: T[]
  pagination: PaginationMeta
}

export interface PostQuery {
  page?: number
  pageSize?: number
  tag?: string
  category?: string
  status?: PostStatus
}

export interface AdminUser {
  githubId: number
  login: string
  name: string
  avatarUrl: string
}

export interface AuthState {
  authenticated: boolean
  user: AdminUser | null
  csrfToken: string | null
}

export interface PostMutation {
  title: string
  slug: string
  excerpt: string
  contentMarkdown: string
  coverUrl: string | null
  status: PostStatus
  publishedAt: ISODateTime | null
  categoryId: number | null
  tagIds: number[]
}

export interface TaxonomyMutation {
  name: string
  slug: string
  description?: string | null
}

export interface UploadResult {
  id: number
  url: string
  filename: string
  contentType: string
  size: number
}

export type UploadStatus = 'active' | 'trashed'

export interface UploadReference {
  resourceType: 'site_avatar' | 'site_banner' | 'post_cover' | 'post_content'
  resourceId: number | null
  field: string
  label: string
}

export interface MediaItem extends UploadResult {
  width: number
  height: number
  status: UploadStatus
  usageCount: number
  references: UploadReference[]
  createdAt: ISODateTime
  trashedAt: ISODateTime | null
}

export interface MediaQuery {
  page?: number
  pageSize?: number
  status?: UploadStatus
  usage?: 'all' | 'used' | 'unused'
  q?: string
}

export interface ApiErrorPayload {
  code: string
  message: string
  fieldErrors?: Record<string, string>
  requestId?: string
}
