import { createRouter, createWebHistory } from 'vue-router'
import { api, sanitizeAdminReturnTo } from '@/services/api'

const router = createRouter({
  history: createWebHistory(),
  scrollBehavior(to, _from, savedPosition) {
    if (savedPosition) return savedPosition
    if (to.hash) return { el: to.hash, top: 96, behavior: 'smooth' }
    return { top: 0 }
  },
  routes: [
    { path: '/', name: 'home', component: () => import('@/pages/HomePage.vue') },
    { path: '/posts/:slug', name: 'post', component: () => import('@/pages/PostDetailPage.vue') },
    { path: '/archive', name: 'archive', component: () => import('@/pages/ArchivePage.vue') },
    { path: '/tags', name: 'tags', component: () => import('@/pages/TagsPage.vue') },
    { path: '/tags/:slug', name: 'tag-posts', component: () => import('@/pages/TaxonomyPostsPage.vue') },
    { path: '/categories', name: 'categories', component: () => import('@/pages/CategoriesPage.vue') },
    { path: '/categories/:slug', name: 'category-posts', component: () => import('@/pages/TaxonomyPostsPage.vue') },
    { path: '/about', name: 'about', component: () => import('@/pages/AboutPage.vue') },
    { path: '/admin/login', name: 'admin-login', meta: { layout: 'bare' }, component: () => import('@/pages/admin/AdminLoginPage.vue') },
    { path: '/admin', name: 'admin-dashboard', meta: { layout: 'admin', requiresAuth: true }, component: () => import('@/pages/admin/AdminDashboardPage.vue') },
    { path: '/admin/posts', name: 'admin-posts', meta: { layout: 'admin', requiresAuth: true }, component: () => import('@/pages/admin/AdminPostsPage.vue') },
    { path: '/admin/posts/new', name: 'admin-post-new', meta: { layout: 'admin', requiresAuth: true }, component: () => import('@/pages/admin/AdminPostEditorPage.vue') },
    { path: '/admin/posts/:id/edit', name: 'admin-post-edit', meta: { layout: 'admin', requiresAuth: true }, component: () => import('@/pages/admin/AdminPostEditorPage.vue') },
    { path: '/admin/taxonomies', name: 'admin-taxonomies', meta: { layout: 'admin', requiresAuth: true }, component: () => import('@/pages/admin/AdminTaxonomiesPage.vue') },
    { path: '/admin/media', name: 'admin-media', meta: { layout: 'admin', requiresAuth: true }, component: () => import('@/pages/admin/AdminMediaPage.vue') },
    { path: '/admin/site', name: 'admin-site', meta: { layout: 'admin', requiresAuth: true }, component: () => import('@/pages/admin/AdminSiteSettingsPage.vue') },
    { path: '/:pathMatch(.*)*', name: 'not-found', component: () => import('@/pages/NotFoundPage.vue') },
  ],
})

router.beforeEach(async (to) => {
  if (to.name === 'admin-login') {
    try {
      const auth = await api.getAuthState()
      if (auth.authenticated) return sanitizeAdminReturnTo(to.query.returnTo)
    } catch {
      // 登录页必须在 API 暂时不可用时仍可打开并显示登录入口。
    }
    return true
  }

  if (!to.meta.requiresAuth) return true
  try {
    const auth = await api.getAuthState()
    if (auth.authenticated) return true
    return { name: 'admin-login', query: { returnTo: to.fullPath } }
  } catch {
    return { name: 'admin-login', query: { returnTo: to.fullPath, error: 'session_check_failed' } }
  }
})

export default router
