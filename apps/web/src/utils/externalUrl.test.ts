import { describe, expect, it } from 'vitest'
import { isSafeExternalUrl, sanitizeSiteProfile } from './externalUrl'
import type { SiteProfile } from '@/types/blog'

describe('external URL safety', () => {
  it('allows absolute HTTP and HTTPS links', () => {
    expect(isSafeExternalUrl('https://github.com/example')).toBe(true)
    expect(isSafeExternalUrl('http://example.test/profile')).toBe(true)
  })

  it('rejects executable, relative and malformed links', () => {
    expect(isSafeExternalUrl('javascript:alert(1)')).toBe(false)
    expect(isSafeExternalUrl('data:text/html,unsafe')).toBe(false)
    expect(isSafeExternalUrl('/admin')).toBe(false)
    expect(isSafeExternalUrl('not a URL')).toBe(false)
  })

  it('removes unsafe social links from a site profile', () => {
    const profile: SiteProfile = {
      title: 'MyBlog',
      subtitle: '',
      description: '',
      avatarUrl: null,
      bannerUrl: null,
      authorName: 'Admin',
      authorBio: '',
      socialLinks: [
        { label: 'GitHub', url: 'https://github.com/example', icon: 'github' },
        { label: 'Unsafe', url: 'javascript:alert(1)', icon: 'link' },
      ],
      icpNumber: null,
    }

    expect(sanitizeSiteProfile(profile).socialLinks).toEqual([profile.socialLinks[0]])
  })
})
