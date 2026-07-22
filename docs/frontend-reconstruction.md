# 前端重构方案

## 1. 目标

以 Fuwari 的阅读体验和视觉语言为基础，使用 Vue 3 重写动态博客前端；文章、标签、分类和站点信息全部来自 Go API，评论由独立部署的 Artalk 提供。

本项目不会把 Fuwari 的 Astro 工程直接作为运行时依赖，也不会修改或内嵌 Artalk 后端源码。

## 2. 上游基线

| 项目 | 参考提交 | 许可证 | 用途 |
| --- | --- | --- | --- |
| saicaca/fuwari | `6d39b0dec41282e7852e23e032998a5789abee28` | MIT | 页面布局、色彩变量、卡片、响应式和动效 |
| ArtalkJS/Artalk | `f875fc4023f02a6d774182459257fbbb0417585f` | MIT | 评论前端接入、Docker/MySQL/反向代理配置参考 |

若复制上游的实质性代码或资源，发布时必须保留对应的 MIT 版权声明。

## 3. Fuwari 迁移边界

### 保留的设计语言

- 顶部悬浮卡片式导航栏；
- 全站 Banner：桌面首页 `65vh`，其他公开页面及移动端 `35vh`；
- 顶部栏在接近内容区时隐藏，桌面回顶按钮在滚动超过 `35vh` 后显示；
- 桌面端左侧个人资料、分类和标签，移动端改为单列；
- 浅色/深色 OKLCH 色彩变量及可调主题色；
- 大圆角纯色内容卡片、浮层阴影和缩放反馈；
- 文章卡片的标题、摘要、元信息、阅读时长和封面布局；
- 文章详情页超宽屏目录、返回顶部和 Markdown 排版；
- 归档、标签、分类、关于页面的内容组织方式。

### 重写的实现

- Astro 页面和内容集合改为 Vue Router 与远程 API；
- Svelte 交互组件改为 Vue Composition API；
- 静态 Pagefind 搜索暂不迁移，V1 不实现全文搜索；
- Swup 页面切换改为 Vue Router 过渡；
- Astro Markdown 编译改为浏览器端 Markdown 渲染与 HTML 清洗；
- 图片地址从相对内容文件改为后端返回的公开 URL。

## 4. Artalk 接入边界

- Vue 只负责在文章详情页创建和销毁 Artalk 实例；
- `pageKey` 固定使用 `/posts/{slug}`，不能使用文章数据库 ID 或完整域名；
- `server` 生产环境使用同源 `/comments`；
- `site` 使用稳定站点名，并与 Artalk 服务端配置一致；
- 深色模式变化时调用 Artalk 实例同步更新；
- 评论审核、评论用户、验证码和反垃圾全部由 Artalk 管理；
- 博客 Go API 不代理或复制评论业务数据。

## 5. 前端工程约束

- Vue 3 + TypeScript + Vite；
- 数据访问统一经过 `services`，页面不得直接拼接 API URL；
- 公共 API 和管理 API 共享领域类型；
- 默认启用模拟 API，使后端开发前所有页面可独立验收；
- 后端上线后仅切换环境变量，不改页面代码；
- Markdown 输出必须经过 DOMPurify 清洗；
- 所有路由具备加载、空数据和错误状态；
- 管理操作必须携带 Cookie，不能保存管理员 Token 到 localStorage。

## 6. 页面范围

### 前台

- `/` 首页文章列表；
- `/posts/:slug` 文章详情；
- `/archive` 文章归档；
- `/tags` 与 `/tags/:slug`；
- `/categories` 与 `/categories/:slug`；
- `/about` 关于页；
- 未匹配路由的 404 页面。

### 管理端

- `/admin/login` 登录入口；
- `/admin` 管理概览；
- `/admin/posts` 文章列表；
- `/admin/posts/new` 新建文章；
- `/admin/posts/:id/edit` 编辑文章；
- `/admin/taxonomies` 标签与分类维护。

## 7. 实施顺序

1. 建立设计变量、领域类型、路由和模拟 API；
2. 完成前台布局与公开页面；
3. 完成 Markdown、目录、主题和响应式交互；
4. 接入 Artalk；
5. 完成管理员页面与表单；
6. 根据所有前端请求形成后端 API 和数据模型文档；
7. 完成构建、类型检查与浏览器验收。
