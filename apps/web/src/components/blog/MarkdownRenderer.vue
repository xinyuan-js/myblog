<script setup lang="ts">
import { ref, watch } from 'vue'
import DOMPurify from 'dompurify'
import hljs from 'highlight.js/lib/core'
import bash from 'highlight.js/lib/languages/bash'
import css from 'highlight.js/lib/languages/css'
import go from 'highlight.js/lib/languages/go'
import javascript from 'highlight.js/lib/languages/javascript'
import json from 'highlight.js/lib/languages/json'
import sql from 'highlight.js/lib/languages/sql'
import typescript from 'highlight.js/lib/languages/typescript'
import xml from 'highlight.js/lib/languages/xml'
import MarkdownIt from 'markdown-it'
import type { HeadingItem } from '@/types/blog'

const props = withDefaults(defineProps<{ source: string; stripLeadingH1?: boolean }>(), { stripLeadingH1: false })
const emit = defineEmits<{ headings: [value: HeadingItem[]] }>()

hljs.registerLanguage('bash', bash)
hljs.registerLanguage('sh', bash)
hljs.registerLanguage('css', css)
hljs.registerLanguage('go', go)
hljs.registerLanguage('javascript', javascript)
hljs.registerLanguage('js', javascript)
hljs.registerLanguage('json', json)
hljs.registerLanguage('sql', sql)
hljs.registerLanguage('typescript', typescript)
hljs.registerLanguage('ts', typescript)
hljs.registerLanguage('html', xml)
hljs.registerLanguage('xml', xml)

function escapeHtml(value: string) {
  return value.replace(/[&<>\"]/g, (character) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[character]!)
}

const markdown: MarkdownIt = new MarkdownIt({
  html: false,
  linkify: true,
  typographer: true,
  highlight(code: string, language: string): string {
    if (language && hljs.getLanguage(language)) {
      return `<pre class="hljs"><code>${hljs.highlight(code, { language }).value}</code></pre>`
    }
    return `<pre class="hljs"><code>${escapeHtml(code)}</code></pre>`
  },
})

function slugify(value: string) {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^\p{Letter}\p{Number}]+/gu, '-')
    .replace(/(^-|-$)/g, '') || 'section'
}

const rendered = ref('')

watch(() => props.source, (source) => {
  const raw = markdown.render(source)
  const documentValue = new DOMParser().parseFromString(raw, 'text/html')
  if (props.stripLeadingH1 && documentValue.body.firstElementChild?.tagName === 'H1') {
    documentValue.body.firstElementChild.remove()
  }
  const used = new Map<string, number>()
  const headings: HeadingItem[] = []

  documentValue.querySelectorAll('h2, h3').forEach((element) => {
    const base = slugify(element.textContent ?? '')
    const count = used.get(base) ?? 0
    used.set(base, count + 1)
    const id = count ? `${base}-${count + 1}` : base
    element.id = id
    headings.push({ id, text: element.textContent ?? '', level: Number(element.tagName.slice(1)) as 2 | 3 })
  })
  documentValue.querySelectorAll('a').forEach((element) => {
    if (element.host && element.host !== window.location.host) {
      element.target = '_blank'
      element.rel = 'noopener noreferrer'
    }
  })

  emit('headings', headings)
  rendered.value = DOMPurify.sanitize(documentValue.body.innerHTML, {
    ADD_ATTR: ['target'],
  })
}, { immediate: true })
</script>

<template>
  <!-- eslint-disable-next-line vue/no-v-html -->
  <div class="markdown-body" v-html="rendered" />
</template>

<style>
.markdown-body { color: var(--text-main); font-size: 1rem; line-height: 1.85; overflow-wrap: anywhere; }
.markdown-body > :first-child { margin-top: 0; }
.markdown-body > :last-child { margin-bottom: 0; }
.markdown-body h1,
.markdown-body h2,
.markdown-body h3,
.markdown-body h4 { color: var(--text-strong); line-height: 1.35; scroll-margin-top: 6rem; }
.markdown-body h1 { margin: 0 0 1.5rem; font-size: clamp(2rem, 5vw, 2.8rem); letter-spacing: -0.035em; }
.markdown-body h2 { margin: 2.5rem 0 1rem; font-size: 1.55rem; }
.markdown-body h3 { margin: 1.8rem 0 0.7rem; font-size: 1.22rem; }
.markdown-body p { margin: 1rem 0; }
.markdown-body a { padding: 0.1rem; margin: -0.1rem; border-radius: 0.3rem; color: var(--primary); text-decoration: underline dashed; text-decoration-color: var(--link-underline); text-decoration-thickness: 1px; text-underline-offset: 0.25rem; }
.markdown-body a:hover { background: var(--plain-hover); text-decoration-color: transparent; }
.markdown-body blockquote { position: relative; margin: 1.4rem 0; padding: 0.25rem 1rem; border: 0; color: var(--text-main); background: transparent; }
.markdown-body blockquote::before { content: ''; position: absolute; top: 0; bottom: 0; left: -0.25rem; width: 0.25rem; border-radius: 999px; background: var(--button-bg); }
.markdown-body :not(pre) > code { padding: 0.125rem 0.25rem; border-radius: 0.375rem; color: var(--button-content); background: var(--button-bg); font-size: 0.9em; }
.markdown-body pre { margin: 1.4rem 0; padding: 1.1rem 1.2rem; overflow-x: auto; border-radius: 0.8rem; background: var(--code-bg); font-size: 0.9rem; line-height: 1.65; }
.markdown-body pre code { background: transparent; }
.markdown-body img { margin: 1.5rem auto; border-radius: 0.8rem; }
.markdown-body table { width: 100%; margin: 1.5rem 0; border-collapse: collapse; overflow: hidden; }
.markdown-body th,
.markdown-body td { padding: 0.65rem 0.8rem; border: 1px solid var(--line-color); text-align: left; }
.markdown-body th { color: var(--text-strong); background: var(--card-muted); }
</style>
