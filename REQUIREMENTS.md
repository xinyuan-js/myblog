# 个人博客系统需求说明（V1）

## 1. 项目概述

本项目是一个面向个人或小范围用户使用的动态博客系统。

系统借鉴成熟静态博客的页面风格，但文章数据由自建的 Go 后端动态管理。前端采用最简单的 Vue 单页应用，不考虑服务端渲染（SSR）。评论功能不自行开发，确定接入 Artalk。

第一版的核心目标是：

- 提供简洁、舒适的文章阅读体验；
- 支持管理员在线创建、编辑、发布和删除文章；
- 使用 Artalk 提供完整的评论与评论管理功能；
- 采用前后端分离架构；
- 尽量降低开发、部署和维护复杂度；
- 能够在 2 核 2GB 的低成本云服务器上运行。

## 2. 技术方案

### 2.1 前端

使用 Vue 3 开发单页应用（SPA），可使用 Vite 作为构建工具。

前端构建完成后生成静态文件，由 Nginx 直接托管，不需要单独运行 Node.js 或 Nuxt 服务。

第一版不考虑 SSR，也不以搜索引擎收录效果作为核心目标。

前端负责：

- 首页和文章列表；
- 文章详情页；
- 标签与分类页面；
- Markdown 内容展示；
- 代码高亮；
- 文章目录；
- 深色模式；
- 响应式布局；
- 简单的管理员操作界面；
- 嵌入 Artalk 评论组件。

页面风格可以基于许可证允许修改和再发布的静态博客主题进行改造，优先选择 MIT 或 Apache-2.0 许可证的项目。

### 2.2 博客后端

使用 Go 和 Gin 框架开发轻量级 REST API，采用模块化单体结构，负责：

- 文章的创建、读取、修改和删除；
- 草稿与发布状态管理；
- 标签和分类管理；
- 封面图、正文图片及附件上传；
- GitHub 管理员登录；
- 管理员会话与权限校验；
- 向 Vue 前端提供博客数据。

第一版不引入微服务、Redis、消息队列等额外组件。

### 2.3 数据库

确定使用 MySQL 作为数据库。

博客后端和 Artalk 可以连接同一个 MySQL 实例，但应使用不同的数据库和数据库账号进行隔离：

```text
MySQL
├── blog      博文、标签、分类、管理员会话等数据
└── artalk    评论、回复、评论者及 Artalk 配置数据
```

建议账号划分为：

```text
blog_user    只允许访问 blog 数据库
artalk_user  只允许访问 artalk 数据库
```

第一阶段在同一台云服务器上运行 MySQL，不额外购买云数据库。MySQL 端口不得暴露到公网。

### 2.4 评论系统

评论系统确定使用 Artalk。

Artalk 作为独立的评论后端服务部署，但与博客其他组件运行在同一台服务器上，不需要额外购买服务器。

Artalk 负责：

- 评论和回复；
- 评论者身份；
- 评论审核；
- 验证码与反垃圾；
- 评论通知；
- 评论后台管理。

博客文章详情页嵌入 Artalk 前端评论组件，自建 Go 后端不重复实现评论业务。

### 2.5 Nginx

Nginx 作为统一访问入口，负责：

- 托管 Vue 构建后的静态文件；
- 处理 SPA 路由回退；
- 反向代理 Go API；
- 反向代理 Artalk；
- HTTPS 证书与安全响应头；
- 上传文件或公开媒体资源访问。

## 3. 系统架构

```text
浏览器
  ↓
Nginx
├── /              Vue 静态文件
├── /api/          Go 博客 API
├── /comments/     Artalk 服务
└── /uploads/      MinIO 中公开只读的媒体资源

Go 博客 API ────────→ MySQL / blog
       └────────────→ MinIO / blog-media
Artalk ─────────────→ MySQL / artalk
```

服务器内主要组件如下：

```text
一台 2 核 2GB 云服务器
├── Nginx
├── Vue 静态文件
├── Go 博客 API
├── Artalk
├── MySQL
└── MinIO
```

## 4. 用户与权限

### 4.1 普通访客

普通访客无需注册或登录，可以：

- 浏览文章；
- 查看标签和分类；
- 查看评论。

发表评论、回复、投票、编辑或删除评论等写操作必须先使用 GitHub 登录本站。评论请求统一经过 Gin 网关，不能绕过本站的封禁和限额策略。

### 4.2 管理员

系统设置一个由环境变量指定的站点所有者，并允许所有者按 GitHub 数字用户 ID 授权或撤销普通管理员。所有管理员都通过 GitHub OAuth 登录；只有站点所有者能够管理管理员权限。

Go 后端使用 GitHub 不可变的数字用户 ID 判断管理员身份：

```env
ADMIN_GITHUB_ID=12345678
```

管理员登录成功后，由 Go 后端建立会话，并通过 Cookie 保存登录状态。Cookie 至少应设置：

- `HttpOnly`；
- `Secure`；
- `SameSite=Lax`。

GitHub Client Secret 只能存放在后端环境变量中，不能发送给 Vue 前端。管理员身份必须由后端校验，不能信任前端传入的角色字段。

考虑到中国大陆访问 GitHub 可能不稳定，可以在后续增加一个受严格保护的本地管理员紧急恢复方式，但不作为第一版主要登录入口。

### 4.3 评论用户

博客不提供注册、密码登录、找回密码或独立个人主页，但会保存登录过的 GitHub 数字 ID、用户名、昵称、头像和已验证邮箱，用于本站会话与 Artalk SSO。评论正文和审核状态仍由 Artalk 管理。

管理端可以搜索评论用户、禁止普通用户执行评论写操作、填写内部封禁原因，并覆盖该用户的每日评论额度。默认额度由服务器配置；站点所有者和管理员不受封禁与每日额度限制。

## 5. 前台功能

V1 需要实现：

- 首页；
- 文章列表及分页；
- 文章详情；
- Markdown 渲染；
- 代码高亮；
- 文章目录；
- 标签和分类；
- 发布时间与更新时间；
- 上一篇与下一篇；
- Artalk 评论区；
- 响应式页面；
- 深色模式；
- 404 页面；
- 基础页面标题和描述信息。

V1 暂不强求：

- SSR；
- 复杂 SEO 优化；
- 全文搜索；
- RSS；
- 点赞和收藏；
- 多语言；
- 普通用户中心。

## 6. 管理端功能

管理端保持简单，与博客前台使用同一个 Vue 项目即可。

V1 需要实现：

- GitHub 管理员登录；
- 查看文章列表；
- 创建文章；
- 编辑文章；
- 将文章移入回收站、完整恢复或永久删除；
- Markdown 编辑；
- 草稿和发布状态切换；
- 自定义文章 Slug；
- 标签和分类管理；
- 封面图和正文图片上传；
- 设置发布时间；
- 修改主页头像和全站 Banner；
- 查看当前登录状态；
- 退出登录。

评论审核和评论管理直接使用 Artalk 自带后台，不在自建管理端重复开发。

## 7. 初步 API

公开接口：

```text
GET /api/posts
GET /api/posts/:slug
GET /api/tags
GET /api/categories
```

管理员接口：

```text
POST   /api/admin/posts
PUT    /api/admin/posts/:id
DELETE /api/admin/posts/:id
POST   /api/admin/uploads
GET    /api/admin/uploads
GET    /api/admin/uploads/:id
DELETE /api/admin/uploads/:id
POST   /api/admin/uploads/:id/restore
DELETE /api/admin/uploads/:id/permanent
```

认证接口：

```text
GET  /api/auth/github
GET  /api/auth/github/callback
GET  /api/auth/me
POST /api/auth/logout
POST /api/auth/artalk/session
```

不提供注册、用户名密码登录或找回密码接口。所有管理员接口必须由 Go 后端校验登录会话和 GitHub ID。

评论用户管理接口：

```text
GET /api/admin/users
PUT /api/admin/users/:githubId/comment-policy
```

## 8. 文件存储

文章、标签、分类和评论等结构化数据保存在 MySQL 中；封面、正文图片、头像和 Banner 由部署在同一台服务器上的 MinIO 管理，底层仍使用服务器本地云盘持久化：

```text
/data/mysql/       MySQL 持久化数据
/data/minio/       MinIO 对象数据
```

MinIO 不是额外购买的云存储，V1 只是给本地云盘增加一层标准化的对象管理接口。浏览器不能直接持有 MinIO 管理凭据或绕过 Gin 上传。Gin 负责管理员鉴权、文件大小与真实类型校验、生成对象键，并通过 S3 兼容接口写入私有的 MinIO 管理端点。Nginx 的 `/uploads/` 仅提供指定 Bucket 的公开只读访问，MinIO Console 和 API 端口不得暴露到公网。

MySQL 的 `uploads` 表保存对象键、公开 URL、原文件名、内容类型、尺寸、哈希、状态和上传时间；`upload_references` 保存媒体被站点头像、Banner、文章封面和正文引用的位置。管理端提供媒体库、引用查看和回收站：正在被引用的媒体不能删除，未引用媒体先进入回收站，超过保留期后才从 MinIO 永久删除。

业务代码依赖内部 `Storage` 接口，不直接依赖 MinIO 细节。以后可使用同一套 S3 接口迁移到阿里云 OSS、Cloudflare R2 或其他对象存储，而无需修改文章和媒体管理逻辑。

## 9. 部署要求

### 9.1 服务器

计划使用：

- 阿里云 ECS；
- 2 vCPU；
- 2GB 内存；
- 40GB ESSD 云盘；
- 3Mbps 公网带宽；
- Linux 操作系统。

### 9.2 部署方式

建议使用 Docker Compose 管理：

- Go 博客 API；
- Artalk；
- MySQL；
- MinIO；
- Nginx。

Vue 只在开发或构建环境中使用 Node.js。生产环境只部署构建后的静态文件，不运行 Vue 开发服务器。

MySQL 与 MinIO 数据目录必须挂载到宿主机持久化路径，避免容器更新或重建导致数据丢失。

2GB 内存能够满足小范围博客使用，但需要限制 MySQL、Artalk 和容器的资源占用，并建议配置约 2GB Swap。不要在生产服务器上频繁执行较重的前端构建任务。

## 10. 域名与合规

如果服务器位于中国大陆：

- 域名需要完成实名认证；
- 网站正式对外开放前需要完成 ICP 备案；
- 网站底部展示 ICP 备案号；
- DNS 可以使用免费的阿里云 DNS；
- HTTPS 使用免费的 Let's Encrypt 证书。

网站开放评论后，需要对用户生成内容进行管理。Artalk 建议启用：

- 评论审核；
- CAPTCHA；
- 接口访问频率限制；
- 反垃圾机制；
- 必要的敏感内容过滤。

## 11. 安全要求

- 公网只开放 80、443 和受限制的 SSH 端口；
- 使用 SSH 密钥登录服务器；
- MySQL 只监听内部网络或 Docker 内部网络，不开放 3306 公网端口；
- 不开放 Docker API；
- GitHub OAuth 密钥通过环境变量或安全配置文件保存；
- 管理接口需要进行会话认证和 CSRF 防护；
- 上传文件需要限制大小、扩展名和真实内容类型；
- Markdown 输出需要进行安全过滤，防止 XSS；
- API、登录回调和评论接口需要配置合理的限流；
- 使用 VS Code Remote SSH 进行日常管理；
- 阿里云 Workbench 作为紧急连接方式。

## 12. 备份要求

必须备份：

- `blog` 数据库；
- `artalk` 数据库；
- MinIO 的 `blog-media` Bucket；
- 必要的部署配置和环境变量模板。

建议每天使用 `mysqldump` 分别导出两个数据库，并使用 MinIO Client 镜像或同步命令备份 `blog-media` Bucket。数据库和对象备份必须保存到服务器之外，例如阿里云 OSS 或管理员本地设备。

备份任务需要设置保留周期，并定期进行恢复测试。云盘快照只能作为辅助措施，不能替代数据库导出和异地备份。

## 13. V1 明确不做

- Nuxt 和 SSR；
- 多管理员及复杂权限系统；
- 自建普通用户注册和登录体系；
- 点赞、收藏和关注；
- 微服务拆分；
- Redis；
- 消息队列；
- 独立云数据库；
- 独立评论服务器；
- 复杂主题或插件系统。

## 14. 推荐项目结构

```text
blog/
├── apps/
│   ├── web/          Vue 3 前端
│   └── api/          Go 博客 API
├── deploy/
│   ├── nginx/        Nginx 配置
│   ├── mysql/        MySQL 初始化脚本
│   └── compose.yml   Docker Compose 配置
├── docs/             项目文档
└── README.md
```

Artalk 使用官方镜像部署，不需要将其源码放入项目仓库。

## 15. 推荐开发顺序

1. 确定页面风格和可合法修改的静态博客主题；
2. 设计 MySQL 数据表；
3. 搭建 Go API 和数据库迁移机制；
4. 实现只读文章、标签和分类接口；
5. 搭建 Vue 3 前端并接入公开 API；
6. 完成文章展示、Markdown、代码高亮和响应式页面；
7. 接入 GitHub 管理员登录；
8. 开发文章管理和文件上传；
9. 部署并接入 Artalk；
10. 编写 Docker Compose 和 Nginx 配置；
11. 配置域名、HTTPS、ICP备案和自动备份；
12. 完成安全检查和数据库恢复测试。

## 16. 最终交付目标

最终交付一个采用 Vue 3 前端、Go 后端、MySQL 数据库、MinIO 媒体存储和 Artalk 评论系统的轻量级动态博客。系统采用前后端分离架构，所有组件能够部署在一台 2 核 2GB 云服务器上，并具备文章管理、管理员登录、图片上传与管理、评论管理、HTTPS 和数据备份能力。
