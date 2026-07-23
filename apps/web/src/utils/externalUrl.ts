import type { SiteProfile } from '@/types/blog'

export function isSafeExternalUrl(value: string): boolean {
  if (/^mailto:[^@\s]+@[^@\s]+\.[^@\s]+$/i.test(value)) return true
  try {
    const parsed = new URL(value)
    return parsed.protocol === 'https:' || parsed.protocol === 'http:'
  } catch {
    return false
  }
}

export function sanitizeSiteProfile(profile: SiteProfile): SiteProfile {
  return {
    ...profile,
    socialLinks: profile.socialLinks.filter((link) => isSafeExternalUrl(link.url)),
  }
}
