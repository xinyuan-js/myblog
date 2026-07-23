# 前端完成度验收

本清单对应 `REQUIREMENTS.md` 的 V1 前台、管理端和安全要求。它用于确认前端完成后可以冻结契约并进入 Go 后端开发。

## 公开页面

| 需求 | 实现证据 | 状态 |
| --- | --- | --- |
| 首页、文章列表、分页 | `HomePage.vue`、`PostList.vue`、`PaginationBar.vue`；非法和越界页规范化、旧请求隔离 | 完成 |
| 全站 Banner 与滚动动效 | 原版 `65vh/35vh` 高度、700ms 图片渐显缩放、300ms 组件入场 | 完成 |
| 顶部栏滚动状态 | 按 Banner 高度与正文重叠量计算阈值，接近正文时上移淡出 | 完成 |
| 返回顶部 | 桌面端超过 `35vh` 后在正文右侧显示，平滑回顶；移动端隐藏 | 完成 |
| 文章详情 | `PostDetailPage.vue` | 完成 |
| Markdown 渲染 | `MarkdownRenderer.vue`，关闭原生 HTML | 完成 |
| XSS 清洗 | `MarkdownRenderer.vue` 使用 DOMPurify | 完成 |
| 代码高亮 | highlight.js 核心版，按语言注册 | 完成 |
| 文章目录 | 自动提取二、三级标题，`TableOfContents.vue` | 完成 |
| 标签和分类 | 列表页、详情筛选页、侧边栏入口 | 完成 |
| 发布时间与更新时间 | `PostMeta.vue` | 完成 |
| 上一篇与下一篇 | `PostDetailPage.vue` | 完成 |
| Artalk 评论区 | `ArtalkComments.vue`，真实环境按需加载 | 完成 |
| 响应式页面 | 全局、导航、侧边栏、文章和管理端断点 | 完成 |
| 深色模式 | `useTheme.ts`，本地保存并同步 Artalk | 完成 |
| 404 页面 | `NotFoundPage.vue` 和兜底路由 | 完成 |
| 页面标题和描述 | `useDocumentMeta.ts`、`index.html` | 完成 |
| 归档 | `ArchivePage.vue` | 完成 |
| 关于页面 | `AboutPage.vue` | 完成 |

## 管理端

| 需求 | 实现证据 | 状态 |
| --- | --- | --- |
| GitHub 统一登录 | `AdminLoginPage.vue`、`githubLoginUrl`，保留并校验站内 `returnTo`；普通用户评论，管理员额外进入管理端 | 完成 |
| 不开放注册 | 无注册路由或接口，所有用户统一使用 GitHub 登录 | 完成 |
| 登录状态与路由保护 | Router Guard；网络失败仍可打开登录页；未登录保留原管理路径 | 完成 |
| OAuth 错误反馈 | 登录页识别取消授权、state 失效和 GitHub 失败 | 完成 |
| 退出登录 | `AdminLayout.vue`；清理模拟会话或服务端会话并返回登录页 | 完成 |
| 文章列表 | `AdminPostsPage.vue` | 完成 |
| 创建文章 | `AdminPostEditorPage.vue` | 完成 |
| 编辑文章 | `AdminPostEditorPage.vue` | 完成 |
| 删除文章 | `AdminPostsPage.vue`，带用户确认 | 完成 |
| Markdown 编辑与预览 | 编辑器左右分区 | 完成 |
| 草稿、发布和定时发布 | 状态选择与发布时间校验 | 完成 |
| 自定义 Slug | 编辑字段、格式约束、发布后锁定 | 完成 |
| 标签和分类管理 | `AdminTaxonomiesPage.vue` | 完成 |
| 封面上传 | 编辑器封面上传 | 完成 |
| 正文图片上传 | 上传后插入光标所在 Markdown 位置 | 完成 |
| 设置发布时间 | `datetime-local` 与 RFC 3339 转换 | 完成 |
| 当前管理员信息 | 管理侧边栏显示 GitHub 用户信息 | 完成 |
| 主页头像与全站 Banner | `AdminSiteSettingsPage.vue` 上传、预览、清除和保存 | 完成 |

## 数据和安全边界

| 要求 | 实现证据 | 状态 |
| --- | --- | --- |
| 页面不直接拼 API URL | `services/api.ts` 统一访问层 | 完成 |
| 前后端可独立开发 | 强类型 Mock API 与真实 HTTP API 同接口 | 完成 |
| Cookie 会话 | 所有请求 `credentials: include` | 完成 |
| CSRF | `/auth/me` 获取内存 Token，写请求自动带 `X-CSRF-Token` | 完成 |
| 不在浏览器保存管理员 Token | 前端没有 localStorage Token | 完成 |
| 草稿不进入公开页面 | Mock 契约测试覆盖，后端文档定义相同条件 | 完成 |
| 未来文章不提前公开 | Mock 契约测试覆盖 | 完成 |
| 已发布 Slug 不破坏评论 | 编辑器锁定、Mock 409、后端 409 契约 | 完成 |
| Artalk 与博客后端解耦 | 独立 `/comments` 配置，无评论 API | 完成 |
| 第三方许可证 | `THIRD_PARTY_NOTICES.md` | 完成 |

## 自动验证结果

- TypeScript 严格类型检查通过；
- Vitest 13 项测试通过，包含分页边界、公开列表分页、登录/退出状态与安全返回路径；
- Vite 生产构建通过；
- 真实 API + Artalk 模式构建通过；
- 首页、文章详情深层路由和管理端深层路由均通过本地 HTTP 200 检查；
- OpenAPI YAML 可解析，包含 16 个路径对象；
- `git diff --check` 无空白错误。

## 进入后端阶段的冻结项

以下内容已经成为后端实现输入，修改时需要同步更新 TypeScript 类型、Mock API、后端文档和 OpenAPI：

- URL、HTTP 方法和查询参数；
- 成功与错误响应结构；
- 文章状态和定时发布语义；
- Slug 锁定规则；
- 会话与 CSRF 交互；
- 上传响应；
- Artalk `pageKey` 规则。
