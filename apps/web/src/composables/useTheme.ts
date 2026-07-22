import { computed, ref } from 'vue'

export type ColorScheme = 'light' | 'dark' | 'auto'

const storedTheme = localStorage.getItem('blog-theme') as ColorScheme | null
const darkMedia = window.matchMedia('(prefers-color-scheme: dark)')
const systemDark = ref(darkMedia.matches)
const scheme = ref<ColorScheme>(['light', 'dark', 'auto'].includes(storedTheme ?? '') ? storedTheme! : 'auto')
const storedHue = Number(localStorage.getItem('blog-hue'))
const hue = ref(Number.isFinite(storedHue) && storedHue >= 0 && storedHue <= 360 ? storedHue : 250)

function applyTheme() {
  const dark = scheme.value === 'dark' || (scheme.value === 'auto' && systemDark.value)
  document.documentElement.classList.toggle('dark', dark)
  document.documentElement.style.setProperty('--hue', String(hue.value))
  document.documentElement.style.colorScheme = dark ? 'dark' : 'light'
  document.querySelector('meta[name="theme-color"]')?.setAttribute(
    'content',
    dark ? '#202126' : '#f4f5fb',
  )
}

applyTheme()
darkMedia.addEventListener('change', (event) => {
  systemDark.value = event.matches
  if (scheme.value === 'auto') applyTheme()
})

export function useTheme() {
  const isDark = computed(() => scheme.value === 'dark' || (scheme.value === 'auto' && systemDark.value))

  function toggleTheme() {
    const sequence: ColorScheme[] = ['light', 'dark', 'auto']
    scheme.value = sequence[(sequence.indexOf(scheme.value) + 1) % sequence.length]!
    localStorage.setItem('blog-theme', scheme.value)
    applyTheme()
  }

  function setTheme(value: ColorScheme) {
    scheme.value = value
    localStorage.setItem('blog-theme', value)
    applyTheme()
  }

  function setHue(value: number) {
    hue.value = Math.max(0, Math.min(360, value))
    localStorage.setItem('blog-hue', String(hue.value))
    applyTheme()
  }

  return { scheme, isDark, hue, toggleTheme, setTheme, setHue }
}
