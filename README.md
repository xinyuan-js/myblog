# MyBlog

MyBlog 是一个面向个人或少量作者的动态博客：Vue 3 单页前端、Gin API、MySQL、部署在同一服务器并使用本地云盘的 MinIO，以及独立的 Artalk 评论系统。生产环境由 Nginx 统一提供 HTTPS 入口。

## 已实现功能

- 公开首页、文章详情、归档、标签、分类、深色模式、响应式布局和 404 页面；
- Markdown 安全渲染、代码高亮、文章目录、上一篇/下一篇和 Artalk 评论；
- GitHub OAuth 单管理员登录，不提供注册或密码登录；
- 文章 CRUD、草稿、立即发布、定时发布、Slug 锁定、标签和分类管理；
- 头像、Banner、封面和正文图片上传，以及带引用保护、回收站和永久删除的媒体库；
- MySQL 内嵌迁移、MinIO 对象存储、健康检查、限流、CSRF 防护和后台清理；
- 单机 Docker Compose、HTTP/HTTPS Nginx 配置和数据库/媒体备份脚本。

完整需求见 [REQUIREMENTS.md](REQUIREMENTS.md)，API 契约见 [docs/openapi.yaml](docs/openapi.yaml)。

## 目录结构

```text
apps/web/                 Vue 3 + Vite
apps/api/                 Go + Gin
deploy/nginx/             HTTP 与 HTTPS 配置
deploy/mysql/             Artalk 数据库初始化
deploy/backup.sh          MySQL 与 MinIO 备份
compose.yml               基础服务编排
compose.https.yml         HTTPS 覆盖配置
```

## 前端开发

需要 Node.js 22。默认使用模拟数据，不依赖后端：

```bash
cd apps/web
npm ci
npm run dev
```

管理端入口为 `http://localhost:5173/admin/login`；模拟模式提供“进入模拟管理端”按钮。要连接真实服务，将 `apps/web/.env.example` 复制为 `apps/web/.env`，并设置：

```env
VITE_USE_MOCK_API=false
VITE_API_BASE_URL=/api
VITE_ARTALK_SERVER=/comments
VITE_ARTALK_SITE=MyBlog
```

## 后端开发

需要 Go 1.26.5 或更高补丁版本、MySQL 和 MinIO。复制 `apps/api/.env.example` 为不提交的本地环境文件，按实际地址修改后将变量导入当前 shell，再启动：

```bash
cd apps/api
set -a
. ./.env
set +a
go run ./cmd/server
```

API 默认监听 `:8080`。`GET /api/healthz` 表示进程存活，`GET /api/readyz` 同时检查 MySQL 和 MinIO。服务启动时会自动、加锁执行数据库迁移。

## GitHub OAuth

1. 在 GitHub 的 Developer settings 中创建 OAuth App；
2. Homepage URL 填生产站点，例如 `https://blog.example.com`；
3. Authorization callback URL 填 `https://blog.example.com/api/auth/github/callback`；
4. 将 Client ID、Client Secret 写入服务器 `.env`；
5. `ADMIN_GITHUB_ID` 必须填写管理员不可变的数字 GitHub ID，而不是用户名；
6. 使用至少 32 字节随机值作为 `OAUTH_STATE_SECRET`。

可用以下命令生成密钥：

```bash
openssl rand -hex 32
```

修改 `ADMIN_GITHUB_ID` 会使旧管理员会话在下一次请求时立即失效。生产环境强制 HTTPS、Secure Cookie，并拒绝默认 MinIO 凭据。

## 生产部署

服务器只需要向公网开放 80 和 443；MySQL 3306、MinIO 9000/9001、Artalk 23366 和 API 8080 均只存在于 Docker 内部网络。

### 1. 数据云盘

将云盘挂载到稳定的绝对路径，例如 `/mnt/cloud-disk/myblog`。MinIO 仍运行在同一台服务器，图片实际写入该云盘：

```text
/mnt/cloud-disk/myblog/mysql   MySQL 数据
/mnt/cloud-disk/myblog/minio   图片对象
/mnt/cloud-disk/myblog/artalk  Artalk 配置和日志
```

不要把 `DATA_DIR` 放在容器文件系统中，也不要使用会随实例释放的临时盘。

### 2. 环境变量

```bash
cp .env.example .env
chmod 600 .env
```

至少修改以下项目：

- `DATA_DIR`：云盘绝对路径；
- `APP_ORIGIN`：唯一的 HTTPS 站点 Origin，不带尾部 `/`；
- 三个 MySQL 密码、两个 MinIO 凭据和 `ARTALK_APP_KEY`；
- GitHub OAuth 四项配置；
- `TLS_FULLCHAIN_FILE` 和 `TLS_PRIVATE_KEY_FILE`：宿主机证书文件路径。

密码中如包含 `$`，Docker Compose 的 `.env` 文件里需要写成 `$$`。先执行配置解析，确认没有缺失变量：

```bash
docker compose --env-file .env -f compose.yml -f compose.https.yml config --quiet
```

### 3. HTTPS 启动

`compose.https.yml` 会保留 80 端口用于跳转，并增加 443：

```bash
docker compose --env-file .env -f compose.yml -f compose.https.yml up -d --build
docker compose --env-file .env -f compose.yml -f compose.https.yml ps
```

验收：

```bash
curl -fsS https://blog.example.com/api/healthz
curl -fsS https://blog.example.com/api/readyz
```

查看日志：

```bash
docker compose --env-file .env -f compose.yml -f compose.https.yml logs --tail=200 api web artalk mysql minio
```

仅在本地或受信任内网做 HTTP 联调时，可以只使用 `compose.yml`，同时将 `APP_ENV=development`、`APP_ORIGIN=http://实际地址`、`SESSION_COOKIE_SECURE=false`。不要把该模式用于公网生产环境。

## Artalk

Artalk 使用同一 MySQL 实例中的独立 `artalk` 数据库和 `artalk_user`。默认开启验证码，并将新评论设为待审核；匿名评论附件默认关闭，避免绕过博客媒体上传策略。

首次启动后创建或更新 Artalk 管理员：

```bash
docker compose --env-file .env -f compose.yml exec artalk artalk admin
```

该命令会交互式读取用户名、邮箱和密码。评论审核、反垃圾、通知和评论用户管理均在 Artalk 管理端完成，不在博客管理端重复实现。

## MinIO 与媒体

浏览器不持有 MinIO 凭据。上传必须经过 Gin 的管理员会话、CSRF、大小、扩展名、文件头、MIME 和图片尺寸校验，再由 API 写入 `blog-media` Bucket。Nginx 的 `/uploads/` 只允许公开读取；MinIO API 和 Console 不对公网开放。

媒体元数据和引用关系保存在 MySQL。被头像、Banner、文章封面或正文引用的图片不能删除；未引用图片先进入回收站，30 天后由后台任务永久清理。

## 备份与恢复

创建备份：

```bash
BACKUP_DIR=/mnt/backup/myblog ./deploy/backup.sh
```

脚本以权限 `0600` 创建压缩包和 SHA-256 校验文件，包含 `blog`、`artalk` 两个 SQL 导出、`blog-media` Bucket 和非敏感部署模板。它不会把生产 `.env` 明文放进压缩包；生产密钥应单独保存在加密密码库中。

备份包含评论邮箱等敏感信息，传到服务器外之前必须再用 `age`、GPG 或等效方案加密。服务器本地默认保留 14 天，可通过 `BACKUP_RETENTION_DAYS` 修改；异地副本的保留策略由外部备份系统负责。

恢复演练必须在隔离环境进行：

1. 校验 `.sha256` 并解压到临时目录；
2. 用新的 `.env` 启动空 MySQL 和 MinIO；
3. 分别将 `blog.sql`、`artalk.sql` 导入对应数据库；
4. 用 `mc mirror --overwrite --remove` 将 `media/` 恢复到 `blog-media`；
5. 启动 API、Artalk 和 Web；
6. 验证 `/api/readyz`、管理员登录、文章、评论、图片读取和上传；
7. 记录恢复时间和结果后销毁隔离环境。

云盘快照只能作为辅助，不能替代 SQL 导出、对象备份和异地副本。

## 质量检查

```bash
cd apps/api
go test ./...
go vet ./...

cd ../web
npm ci
npm run type-check
npm test
npm run build
npm audit
```

Compose 与脚本检查：

```bash
docker compose --env-file .env -f compose.yml config --quiet
docker compose --env-file .env -f compose.yml -f compose.https.yml config --quiet
sh -n deploy/backup.sh deploy/mysql/init-artalk.sh
```

## 安全运维要点

- `.env`、备份明文、数据库和云盘目录都不能提交 Git；
- 只允许密钥 SSH，关闭公网 MySQL、MinIO、Artalk、API 和 Docker API；
- 定期更新固定版本镜像和依赖，更新前先备份并在隔离环境验证；
- 证书续期后重建或重启 Web 容器，让文件挂载重新解析到新证书；
- 定期测试 GitHub 登录、CSRF、评论审核、媒体引用保护和备份恢复；
- 中国大陆公开部署需完成 ICP 备案，并在站点设置中展示备案号。

## 第三方项目

- [saicaca/fuwari](https://github.com/saicaca/fuwari)：MIT，视觉和阅读体验参考；
- [ArtalkJS/Artalk](https://github.com/ArtalkJS/Artalk)：MIT，评论系统。

版权说明见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
