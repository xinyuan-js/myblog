# Go 博客后端开发文档（由前端需求反推）

> 状态：前端契约基线  
> 对应前端：`apps/web`  
> API 前缀：`/api`  
> 评论服务：Artalk，生产环境由 `/comments` 反向代理，不属于本 API

后端框架固定使用 Gin（当前锁定 `v1.11.0`），Go 模块声明为 Go 1.26.5。项目不再同时维护 `net/http`、Echo、Fiber 等其他路由实现；Gin 负责路由、参数绑定和中间件编排，业务规则仍放在领域/用例层，不能写入 Handler。

## 1. 文档目标

本文档不是抽象的接口设想，而是由已经实现的 Vue 页面、管理表单和数据访问层反推的后端开发契约。Go 后端只要按照本文提供数据，前端即可从模拟模式切换到真实 API，无需修改页面。

前端的数据访问基线位于：

- `apps/web/src/types/blog.ts`：领域类型；
- `apps/web/src/services/api.ts`：请求路径、方法和参数；
- `apps/web/.env.example`：API 与 Artalk 地址。

## 2. 后端职责边界

### Go API 负责

- 站点公开信息；
- 文章、标签、分类的公开查询；
- 文章草稿、发布和定时发布；
- GitHub 统一登录和服务端会话，按不可变数字 ID 授予管理权限；
- 管理端文章、标签和分类 CRUD；
- 封面、正文图片和附件上传；
- 数据校验、权限校验、CSRF 和审计日志。

### Artalk 负责

- 评论、回复、评论者身份；
- 评论审核、验证码、限流和反垃圾；
- 评论通知和评论后台；
- 评论图片上传（若启用）。

Go API 不建立评论表，不代理 Artalk 用户，不把 Artalk 管理接口包装成自己的接口。

## 3. 通用 HTTP 契约

### 3.1 地址与格式

- API 基础路径：`/api`；
- JSON 使用 UTF-8；
- 字段命名统一为 `camelCase`；
- 时间统一返回 RFC 3339，例如 `2026-07-18T09:30:00+08:00`；
- 服务端内部建议全部使用 UTC，序列化时保留明确时区；
- 数据库字符集使用 `utf8mb4`；
- 所有响应应包含 `X-Request-ID`。

### 3.2 成功响应

除 `204 No Content` 外，统一包裹在 `data` 字段：

```json
{
  "data": {
    "id": 1,
    "title": "从一个可靠的 Go 服务开始"
  }
}
```

分页数据也放入 `data`：

```json
{
  "data": {
    "items": [],
    "pagination": {
      "page": 1,
      "pageSize": 10,
      "total": 0,
      "totalPages": 1
    }
  }
}
```

即使没有数据，`totalPages` 也返回 `1`，避免前端出现 `0 / 0`。

### 3.3 错误响应

```json
{
  "code": "VALIDATION_FAILED",
  "message": "提交内容不符合要求",
  "fieldErrors": {
    "slug": "Slug 已被使用"
  },
  "requestId": "01J..."
}
```

稳定错误码至少包括：

| HTTP | code | 场景 |
| --- | --- | --- |
| 400 | `INVALID_ARGUMENT` | 参数类型或格式错误 |
| 400 | `VALIDATION_FAILED` | 表单字段校验失败 |
| 401 | `AUTH_REQUIRED` | 未登录或会话失效 |
| 403 | `ADMIN_REQUIRED` | 当前 GitHub 用户没有管理权限 |
| 403 | `CSRF_INVALID` | CSRF Token 或 Origin 校验失败 |
| 404 | `POST_NOT_FOUND` | 文章不存在或公开端不可见 |
| 404 | `TAG_NOT_FOUND` | 标签不存在 |
| 404 | `CATEGORY_NOT_FOUND` | 分类不存在 |
| 409 | `SLUG_CONFLICT` | Slug 已存在 |
| 409 | `POST_SLUG_LOCKED` | 已发布文章不允许修改 Slug |
| 409 | `TAXONOMY_IN_USE` | 标签或分类仍被文章使用 |
| 413 | `UPLOAD_TOO_LARGE` | 上传超限 |
| 415 | `UNSUPPORTED_MEDIA_TYPE` | 文件类型不允许 |
| 429 | `RATE_LIMITED` | 请求频率超限 |
| 500 | `INTERNAL_ERROR` | 未预期服务端错误 |

错误的 `message` 可以直接展示给管理员，但不能包含 SQL、文件绝对路径、密钥或调用栈。

### 3.4 分页与排序

- `page`：从 1 开始，默认 1；
- `pageSize`：公开列表默认 10，管理列表默认 20；
- `pageSize` 最大 100；
- 公开文章固定按 `published_at DESC, id DESC`；
- 管理文章固定按 `updated_at DESC, id DESC`；
- V1 不让前端传任意 SQL 排序字段。

## 4. 前端真正需要的数据模型

### 4.1 SiteProfile

```json
{
  "title": "MyBlog",
  "subtitle": "把复杂的事情慢慢说清楚",
  "description": "记录工程实践、阅读笔记和生活。",
  "avatarUrl": "/uploads/site/avatar.webp",
  "bannerUrl": "/uploads/site/banner.webp",
  "authorName": "示例作者",
  "authorBio": "Go 开发者，长期主义练习生。",
  "socialLinks": [
    { "label": "GitHub", "url": "https://github.com/example", "icon": "github" }
  ],
  "icpNumber": "沪ICP备XXXXXXXX号"
}
```

`icon` 允许值：`github`、`mail`、`rss`、`link`。头像和全站 Banner 由管理端通过 `PUT /api/admin/site/appearance` 更新；请求只接受 `avatarUrl` 和 `bannerUrl`，两者均可为 `null`。URL 必须是本站上传接口返回的公开资源地址，后端不能接受任意文件路径或 HTML。

### 4.2 Tag

```json
{ "id": 1, "name": "Go", "slug": "go", "postCount": 3 }
```

公开接口的 `postCount` 只能统计当前可公开访问的文章；管理页面暂时也可以使用公开计数。

### 4.3 Category

```json
{
  "id": 1,
  "name": "技术",
  "slug": "technology",
  "description": "软件工程与实践记录",
  "postCount": 5
}
```

一篇文章在 V1 中最多属于一个分类，可以拥有多个标签。

### 4.4 PostSummary

```json
{
  "id": 1,
  "title": "从一个可靠的 Go 服务开始",
  "slug": "building-a-reliable-go-service",
  "excerpt": "在业务代码之前，先把地基搭好。",
  "coverUrl": "/uploads/2026/07/cover.webp",
  "status": "published",
  "publishedAt": "2026-07-18T09:30:00+08:00",
  "updatedAt": "2026-07-20T21:10:00+08:00",
  "category": { "id": 1, "name": "技术", "slug": "technology", "description": null, "postCount": 5 },
  "tags": [{ "id": 1, "name": "Go", "slug": "go", "postCount": 3 }],
  "wordCount": 2860,
  "readingTimeMinutes": 9
}
```

约束：

- `excerpt` 不为空；未手工填写时由后端从 Markdown 纯文本截取，建议最多 200 个 Unicode 字符；
- `wordCount` 和 `readingTimeMinutes` 由后端保存文章时计算，不能信任前端；
- 中文阅读时长建议按每分钟 300 字向上取整，最少 1 分钟；
- `coverUrl` 必须是本站允许的公开 URL 或受控外部 URL；
- 公开接口只会返回 `status: published`。

### 4.5 PostDetail

在 `PostSummary` 基础上增加：

```json
{
  "contentMarkdown": "# 正文",
  "previousPost": { "title": "上一篇", "slug": "previous" },
  "nextPost": { "title": "下一篇", "slug": "next" }
}
```

- 前端接收原始 Markdown，禁止后端直接返回未经约束的任意 HTML；
- 前端 Markdown 配置关闭原生 HTML，并再次使用 DOMPurify 清洗；
- 上一篇和下一篇均按公开发布时间计算；
- 不存在时必须返回 JSON `null`，不能省略字段。

### 4.6 PostMutation

```json
{
  "title": "文章标题",
  "slug": "article-slug",
  "excerpt": "文章摘要",
  "contentMarkdown": "# 正文",
  "coverUrl": "/uploads/2026/07/cover.webp",
  "status": "draft",
  "publishedAt": null,
  "categoryId": 1,
  "tagIds": [1, 2]
}
```

服务端校验：

- `title`：去首尾空格后 1～200 字符；
- `slug`：1～160 字符，正则 `^[a-z0-9]+(?:-[a-z0-9]+)*$`，全局唯一；
- `excerpt`：最多 500 字符；
- `contentMarkdown`：至少 1 字符，建议最大 2 MiB；
- `status`：`draft`、`published`、`scheduled`；
- `scheduled` 必须提供未来的 `publishedAt`；
- `published` 未提供 `publishedAt` 时由服务端写入当前时间；
- `categoryId` 可为 `null`，非空时必须存在；
- `tagIds` 去重，全部必须存在，建议最多 20 个；
- `coverUrl` 可为 `null`。

#### 定时发布不需要后台任务

数据库只需存储 `draft` 或 `published` 两种持久状态：

- 前端提交 `scheduled` 时，后端存为 `published`，同时保存未来的 `published_at`；
- 管理接口序列化时，若数据库状态为 `published` 且 `published_at > NOW()`，返回有效状态 `scheduled`；
- 公开查询条件始终包含 `status = 'published' AND published_at <= NOW()`。

这样不需要定时任务或消息队列。

#### 已发布文章锁定 Slug

文章第一次公开后，不允许直接修改 Slug。原因是外部链接、浏览器收藏和 Artalk 的 `pageKey=/posts/{slug}` 都依赖它。后端收到修改请求时返回：

```http
409 Conflict
```

```json
{ "code": "POST_SLUG_LOCKED", "message": "文章发布后不能修改 Slug" }
```

未来若要支持修改，必须同时设计旧 Slug 重定向和 Artalk 页面迁移，不属于 V1。

## 5. 公开接口

### 5.1 获取站点信息

```http
GET /api/site
```

返回 `SiteProfile`。建议设置短时间公共缓存：

```http
Cache-Control: public, max-age=300
```

### 5.2 文章列表

```http
GET /api/posts?page=1&pageSize=10&tag=go&category=technology
```

查询参数：

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `page` | 否 | 默认 1 |
| `pageSize` | 否 | 默认 10，最大 100 |
| `tag` | 否 | 标签 Slug |
| `category` | 否 | 分类 Slug |

规则：

- 只返回已到发布时间的公开文章；
- `tag` 或 `category` 不存在时返回空列表，不返回 404；
- 列表不返回 `contentMarkdown`；
- SQL 必须消除标签 JOIN 带来的重复文章；
- 标签和分类计数不能引入 N+1 查询。

### 5.3 文章详情

```http
GET /api/posts/{slug}
```

只允许访问已公开文章。草稿、未来定时文章和不存在的文章统一返回 `404 POST_NOT_FOUND`，不能借错误信息泄漏草稿是否存在。

### 5.4 标签列表

```http
GET /api/tags
```

返回 `Tag[]`，建议按 `postCount DESC, name ASC` 排序。没有公开文章的标签可以不返回。

### 5.5 分类列表

```http
GET /api/categories
```

返回 `Category[]`，建议按 `name ASC` 或管理端定义的稳定顺序排序。没有公开文章的分类可以保留。

## 6. GitHub 管理员认证

系统统一使用 GitHub OAuth 登录，不提供用户名密码登录和用户注册。环境变量 `ADMIN_GITHUB_ID` 指定站点所有者；所有者可在管理端按 GitHub 数字 ID维护授权管理员名单，只有所有者能变更该名单。

### 6.1 发起登录

```http
GET /api/auth/github?return_to=/admin
```

后端行为：

1. 只接受站内相对路径形式的 `return_to`，拒绝完整外部 URL，防止开放重定向；
2. 生成至少 128 bit 随机 OAuth `state`；
3. 将 `state`、`return_to` 和短过期时间保存在签名 Cookie 或服务端临时记录；
4. 重定向到 GitHub OAuth。

前端会把原页面作为 `return_to` 传入，例如未登录访问文章编辑页时，管理员登录完成后返回同一编辑页。普通用户请求管理路径时后端改为返回首页。前后端都只接受站内相对路径，但后端仍必须独立执行上述校验。

### 6.2 OAuth 回调

```http
GET /api/auth/github/callback?code=...&state=...
```

后端必须：

- 校验 state 且只能使用一次；
- 服务端使用 Client Secret 换取 GitHub Token；
- 调用 GitHub 用户接口取得不可变数字 `id`；
- 使用 GitHub 数字 ID 判断配置所有者或数据库中的授权管理员；
- 不接受前端传入的管理员角色；
- 登录完成后立即丢弃 GitHub Access Token，V1 不需要持久化；
- 创建博客自己的随机会话；
- 重定向到已校验的 `return_to`。

失败时重定向 `/admin/login?error={code}`，前端识别以下稳定错误码：

- `access_denied`：用户取消 GitHub 授权；
- `state_invalid`：state 缺失、过期或已使用；
- `oauth_failed`：GitHub 换取 Token 或读取用户失败。

### 6.3 当前登录状态

```http
GET /api/auth/me
```

已登录：

```json
{
  "data": {
    "authenticated": true,
    "user": {
      "githubId": 12345678,
      "login": "demo-admin",
      "name": "示例管理员",
      "avatarUrl": "https://avatars.githubusercontent.com/..."
    },
    "csrfToken": "base64url-token"
  }
}
```

未登录仍返回 `200`：

```json
{
  "data": {
    "authenticated": false,
    "user": null,
    "csrfToken": null
  }
}
```

### 6.4 退出登录

```http
POST /api/auth/logout
X-CSRF-Token: ...
```

撤销服务端会话、清除 Cookie，返回 `204`。重复退出也返回 `204`。

### 6.5 会话与 CSRF

Cookie 最低要求：

```text
HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=604800
```

建议会话实现：

- Cookie 仅保存 256 bit 随机会话原文；
- 数据库只保存 `SHA-256(session_token)`；
- 会话默认 7 天，访问时可限制刷新频率；
- 登录、退出和权限失败写安全审计日志；
- 所有 `/api/admin/*` 和 `POST /api/auth/logout` 同时校验 Cookie、`X-CSRF-Token` 和 `Origin`；
- CSRF Token 由 `/auth/me` 返回，只保存在前端内存，不写 localStorage；
- 前端已经在所有非 GET 请求上自动附加该 Header。

## 7. 管理接口

以下接口全部需要管理员会话和 CSRF 校验。

### 7.1 管理文章列表

```http
GET /api/admin/posts?page=1&pageSize=20&status=draft
```

`status` 可选：`draft`、`published`、`scheduled`。返回 `Paginated<PostSummary>`，包含草稿和未来文章。

### 7.2 获取管理文章详情

```http
GET /api/admin/posts/{id}
```

返回完整 `PostDetail`，无论是否公开。`previousPost`、`nextPost` 对管理编辑器无实际用途，但为保持统一类型仍需返回，可以为 `null`。

### 7.3 创建文章

```http
POST /api/admin/posts
Content-Type: application/json
X-CSRF-Token: ...
```

请求体为 `PostMutation`，成功返回 `201` 和完整 `PostDetail`，并设置：

```http
Location: /api/admin/posts/{id}
```

### 7.4 更新文章

```http
PUT /api/admin/posts/{id}
```

使用完整替换语义，请求体为完整 `PostMutation`。文章、分类和标签关联必须在同一数据库事务中更新。

### 7.5 删除文章

```http
DELETE /api/admin/posts/{id}
```

返回 `204`。建议使用软删除 `deleted_at`，避免误操作立即造成不可恢复的数据损失。V1 不要求恢复 UI，但备份和运维可以恢复。关联上传文件不要在事务中直接删除，先保留为未引用资源，后续维护任务再清理。

### 7.6 标签 CRUD

```text
POST   /api/admin/tags
PUT    /api/admin/tags/{id}
DELETE /api/admin/tags/{id}
```

请求：

```json
{ "name": "Go", "slug": "go" }
```

名称和 Slug 均全局唯一。删除仍被文章引用的标签时，V1 建议返回 `409 TAXONOMY_IN_USE`，由管理员先处理文章关系。

### 7.7 分类 CRUD

```text
POST   /api/admin/categories
PUT    /api/admin/categories/{id}
DELETE /api/admin/categories/{id}
```

请求：

```json
{
  "name": "技术",
  "slug": "technology",
  "description": "软件工程与实践记录"
}
```

删除仍被文章引用的分类时返回 `409 TAXONOMY_IN_USE`。

### 7.8 文件上传

```http
POST /api/admin/uploads
Content-Type: multipart/form-data
X-CSRF-Token: ...

file=<binary>
```

成功返回：

```json
{
  "data": {
    "id": 42,
    "url": "/uploads/2026/07/01J....webp",
    "filename": "cover.png",
    "contentType": "image/png",
    "size": 245102
  }
}
```

V1 上传规则：

- 最大 10 MiB；
- 图片允许 JPEG、PNG、WebP、GIF；
- 附件若开放，使用独立白名单，不允许 HTML、SVG、JS 和可执行文件；
- 同时检查扩展名、声明 MIME 和真实文件头；
- 服务端生成随机文件名，不使用用户文件名作为磁盘路径；
- 文件通过 S3 兼容接口写入 MinIO 的 `blog-media` Bucket，对象键使用 `YYYY/MM/<随机文件名>`；
- 防止路径穿越；
- 校验完成后再写入 MinIO，上传失败不能写入数据库记录；
- MinIO API 和 Console 只允许内网访问，浏览器不得获得管理凭据；
- Nginx 将 `/uploads/` 映射为 Bucket 的公开只读访问，不允许列目录或写入；
- 建议返回 `X-Content-Type-Options: nosniff`。

MinIO 与博客服务部署在同一台服务器，Bucket 数据持久化到本地云盘的 `/data/minio/`；它是本地文件之上的 S3 兼容管理层，而不是 V1 额外购买的云存储。Go 业务层只依赖 `Storage` 接口。对象存储只负责文件本身；鉴权、媒体元数据、引用关系、回收站和永久删除仍由博客后端管理。

## 8. MySQL 数据模型

建议迁移表如下。

### 8.1 `posts`

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `id` | BIGINT UNSIGNED | PK, AUTO_INCREMENT |
| `title` | VARCHAR(200) | NOT NULL |
| `slug` | VARCHAR(160) | NOT NULL, UNIQUE |
| `excerpt` | VARCHAR(500) | NOT NULL |
| `content_markdown` | MEDIUMTEXT | NOT NULL |
| `cover_url` | VARCHAR(2048) | NULL |
| `category_id` | BIGINT UNSIGNED | NULL, FK categories |
| `status` | VARCHAR(20) | `draft` / `published` |
| `published_at` | DATETIME(6) | NULL |
| `word_count` | INT UNSIGNED | NOT NULL |
| `reading_time_minutes` | SMALLINT UNSIGNED | NOT NULL |
| `created_at` | DATETIME(6) | NOT NULL |
| `updated_at` | DATETIME(6) | NOT NULL |
| `deleted_at` | DATETIME(6) | NULL |

索引：

```sql
UNIQUE KEY uk_posts_slug (slug),
KEY idx_posts_public (status, published_at, id),
KEY idx_posts_category_public (category_id, status, published_at),
KEY idx_posts_updated (updated_at, id),
KEY idx_posts_deleted (deleted_at)
```

所有文章查询默认带 `deleted_at IS NULL`。

### 8.2 `tags`

```text
id BIGINT UNSIGNED PK
name VARCHAR(80) NOT NULL UNIQUE
slug VARCHAR(80) NOT NULL UNIQUE
created_at DATETIME(6) NOT NULL
updated_at DATETIME(6) NOT NULL
```

### 8.3 `categories`

```text
id BIGINT UNSIGNED PK
name VARCHAR(80) NOT NULL UNIQUE
slug VARCHAR(80) NOT NULL UNIQUE
description VARCHAR(300) NULL
created_at DATETIME(6) NOT NULL
updated_at DATETIME(6) NOT NULL
```

### 8.4 `post_tags`

```text
post_id BIGINT UNSIGNED NOT NULL
tag_id BIGINT UNSIGNED NOT NULL
PRIMARY KEY (post_id, tag_id)
KEY idx_post_tags_tag (tag_id, post_id)
```

外键删除策略建议：文章软删除不触发关联删除；标签删除使用 `RESTRICT`。

### 8.5 `admin_sessions`

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | BIGINT UNSIGNED | 主键 |
| `token_hash` | BINARY(32) | 会话 Token SHA-256，唯一 |
| `csrf_token_hash` | BINARY(32) | CSRF Token SHA-256 |
| `github_id` | BIGINT UNSIGNED | GitHub 不可变 ID |
| `github_login` | VARCHAR(100) | 展示用 |
| `display_name` | VARCHAR(200) | 展示用 |
| `avatar_url` | VARCHAR(2048) | 展示用 |
| `expires_at` | DATETIME(6) | 过期时间，建立索引 |
| `last_seen_at` | DATETIME(6) | 最近访问 |
| `created_at` | DATETIME(6) | 创建时间 |

### 8.6 `uploads`

```text
id BIGINT UNSIGNED PK
storage_key VARCHAR(512) NOT NULL UNIQUE
public_url VARCHAR(2048) NOT NULL
original_filename VARCHAR(255) NOT NULL
content_type VARCHAR(100) NOT NULL
size BIGINT UNSIGNED NOT NULL
sha256 BINARY(32) NOT NULL
created_at DATETIME(6) NOT NULL
deleted_at DATETIME(6) NULL
```

补充保存图片的 `width`、`height` 和状态字段。状态至少包含 `active` 与 `trashed`，进入回收站只更新状态和时间，不立即删除 MinIO 对象。

### 8.6.1 `upload_references`

```text
id BIGINT UNSIGNED PK
upload_id BIGINT UNSIGNED NOT NULL FK uploads
resource_type VARCHAR(32) NOT NULL
resource_id BIGINT UNSIGNED NULL
field VARCHAR(32) NOT NULL
created_at DATETIME(6) NOT NULL
```

`resource_type` 至少支持 `site_avatar`、`site_banner`、`post_cover` 和 `post_content`。保存文章或站点外观时在同一业务事务中同步引用关系；存在引用时删除接口返回 `409 UPLOAD_IN_USE`。

### 8.7 `site_settings`

使用单行设置表保存站点资料及 `avatar_url`、`banner_url`。更新站点外观时应在事务内锁定该行并更新时间；必须经过结构化校验，不能向前端透传任意 HTML。删除展示图片只清空引用，上传文件的回收由独立的孤立文件清理任务处理，避免误删仍被文章引用的资源。

## 9. 查询和事务要求

### 9.1 公开文章列表

核心条件：

```sql
WHERE posts.deleted_at IS NULL
  AND posts.status = 'published'
  AND posts.published_at <= UTC_TIMESTAMP(6)
```

标签、分类筛选使用参数化查询。文章分页应先取得文章 ID，再批量加载标签，避免 JOIN 后直接 `LIMIT` 造成分页条数错误。

### 9.2 保存文章事务

创建或更新必须在一个事务中完成：

1. 校验分类和全部标签存在；
2. 校验 Slug 唯一及锁定规则；
3. 写入文章；
4. 删除旧标签关联并批量写入新关联；
5. 提交；
6. 返回重新查询得到的完整对象。

任何一步失败都回滚。

### 9.3 上一篇和下一篇

以当前文章的 `(published_at, id)` 为游标分别查询，且使用与公开文章相同的可见性条件。不能根据数据库 ID 简单加减。

## 10. 缓存与条件请求

V1 不引入 Redis。

- `GET /site`、`GET /tags`、`GET /categories` 可返回短公共缓存；
- 文章详情可生成 `ETag`，内容更新后变化；
- 管理接口和 `/auth/me` 必须 `Cache-Control: no-store`；
- 写操作成功后无需主动清理分布式缓存，因为 V1 没有该缓存层。

## 11. 限流建议

单机内存令牌桶即可，按真实客户端 IP 和路由分组：

| 路由 | 建议限制 |
| --- | --- |
| 公开 GET | 120 次/分钟/IP |
| `/auth/github` | 10 次/10 分钟/IP |
| OAuth callback | 30 次/10 分钟/IP |
| 管理写接口 | 60 次/分钟/会话 |
| 上传 | 20 次/小时/会话 |

Nginx 也可增加外层限制，但 Go 后端仍需要保护认证和上传端点。

## 12. Artalk 部署契约

前端初始化参数：

```ts
Artalk.init({
  el: '#artalk-comments',
  pageKey: `/posts/${post.slug}`,
  pageTitle: post.title,
  server: '/comments',
  site: 'MyBlog',
  locale: 'zh-CN',
  darkMode: isDark,
})
```

部署要求：

- 使用固定版本 `artalk/artalk-go:<version>`；
- 内部端口 `23366`；
- 数据库连接 MySQL 的独立 `artalk` 库和 `artalk_user`；
- `site_default`、`site_url` 和前端 `site` 保持一致；
- `app_key` 使用高熵密钥；
- 配置 `trusted_domains`；
- 默认启用图片验证码和评论审核策略；
- Nginx 将 `/comments/` 正确反代到 Artalk；
- Artalk 管理后台仍由 Artalk 自己提供。

需要在正式 Nginx 配置阶段验证“子路径部署”是否与选定 Artalk 版本完全兼容；若版本对资源路径支持不完整，改为同域子域名 `comments.example.com`，前端只需修改 `VITE_ARTALK_SERVER`。

## 13. Go 工程建议

后端保持基于 Gin 的模块化单体：

```text
apps/api/
├── cmd/server/              程序入口
├── internal/
│   ├── config/              配置解析与校验
│   ├── httpapi/             路由、Handler、Middleware
│   ├── auth/                GitHub OAuth、会话、CSRF
│   ├── post/                文章领域与用例
│   ├── taxonomy/            标签和分类
│   ├── upload/              上传校验与存储
│   ├── repository/          MySQL 查询实现
│   └── clock/               时间抽象，便于测试定时发布
├── migrations/              有序 SQL 迁移
└── go.mod
```

HTTP 入口使用 `gin.New()`，显式安装请求 ID、访问日志和恢复中间件，不使用带默认日志器的 `gin.Default()`。默认不信任任何代理头；生产环境只通过 `TRUSTED_PROXIES` 配置真实 Nginx 地址或 CIDR。

基础探针：

- `GET /api/healthz`：进程存活检查；
- `GET /api/readyz`：依赖就绪检查，接入 MySQL 后必须实际执行轻量级数据库检查。

V1 不需要 Redis、消息队列、微服务或通用插件系统。

## 14. 后端开发顺序

1. 配置、日志、HTTP 生命周期、健康检查；
2. MySQL 连接、迁移器和基础数据表；
3. 公开站点、标签、分类接口；
4. 公开文章列表和详情；
5. GitHub OAuth、数据库会话、CSRF；
6. 管理文章读写和事务；
7. 标签、分类管理；
8. 上传服务；
9. 限流、安全响应头和审计日志；
10. Docker、Nginx、Artalk 和备份。

## 15. 验收清单

后端完成不能只以“接口能返回 200”为标准，至少验证：

- [ ] 草稿和未到时间的文章不能从任何公开接口泄漏；
- [ ] 标签、分类筛选分页正确且无重复文章；
- [ ] 上一篇、下一篇跳过草稿和未来文章；
- [ ] 重复 Slug 返回稳定的 409 错误；
- [ ] 已发布文章修改 Slug 被拒绝；
- [ ] 定时文章到时间后无需任务即可公开；
- [ ] 文章和标签关联在事务失败时完整回滚；
- [ ] 普通 GitHub 用户可以创建评论会话，但访问管理接口返回 403；
- [ ] OAuth state 不匹配、过期或重放均失败；
- [ ] 管理 Cookie 满足 HttpOnly、Secure、SameSite=Lax；
- [ ] 缺少或错误 CSRF Token 的写请求被拒绝；
- [ ] 外部 Origin 的管理写请求被拒绝；
- [ ] 文件伪造扩展名、超限和路径穿越被拒绝；
- [ ] 删除仍在使用的分类返回 409；
- [ ] 所有 SQL 使用参数化查询；
- [ ] 公开错误不包含敏感内部信息；
- [ ] `/auth/me` 和管理响应禁止缓存；
- [ ] 前端设置 `VITE_USE_MOCK_API=false` 后无需修改源码即可工作；
- [ ] Artalk 评论在深色模式和文章切换后正确销毁、重建；
- [ ] MySQL 的 `blog_user` 无法访问 `artalk` 数据库，反之亦然。
