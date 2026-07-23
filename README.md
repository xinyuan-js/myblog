# MyBlog

MyBlog 是一个面向个人或少量作者的动态博客：Vue 3 单页前端、Gin API、MySQL、部署在同一服务器并使用本地云盘的 MinIO，以及独立的 Artalk 评论系统。生产环境由 Nginx 统一提供 HTTPS 入口。

## 已实现功能

- 公开首页、文章详情、归档、标签、分类、深色模式、响应式布局和 404 页面；
- Markdown 安全渲染、代码高亮、文章目录、上一篇/下一篇和 Artalk 评论；
- GitHub OAuth 统一登录，不提供注册或密码登录；所有登录用户可评论，指定账号额外获得站点与评论管理权限；
- 文章 CRUD、草稿、立即发布、定时发布、Slug 锁定、关联完整保留的回收站、标签和分类管理；
- 头像、Banner、封面和正文图片上传，以及带引用保护、回收站和永久删除的媒体库；
- 所有者专属的管理操作审计页，记录管理员写请求的操作者、路径、结果、来源 IP 和请求 ID；
- MySQL 内嵌迁移、MinIO 对象存储、健康检查、限流、CSRF 防护和后台清理；
- 单机 Docker Compose、HTTP/HTTPS Nginx 配置和数据库/媒体备份脚本。

完整需求见 [REQUIREMENTS.md](REQUIREMENTS.md)，API 契约见 [docs/openapi.yaml](docs/openapi.yaml)。
安全边界、审计证据和仍需在发布前执行的镜像门禁见 [docs/security-review.md](docs/security-review.md)。

## 目录结构

```text
apps/web/                 Vue 3 + Vite
apps/api/                 Go + Gin
.github/workflows/ci.yml  无生产密钥的持续集成门禁
deploy/nginx/             HTTP 与 HTTPS 配置
deploy/mysql/             Artalk 数据库与最小权限桥接账号初始化
deploy/minio/             固定 MinIO 构建与最小权限账号初始化
deploy/check-secrets.sh   启动前凭据强度与复用检查
deploy/backup.sh          MySQL 与 MinIO 备份
deploy/verify-backup.sh   备份校验、结构和路径安全检查
deploy/reload-web-certificate.sh  证书更新后重建 Web 容器
compose.yml               基础服务编排
compose.https.yml         HTTPS 覆盖配置
```

## 前端开发

需要 Node.js 22.12 或更新版本。默认连接真实 API；如果只想预览界面，可以在本地环境文件中设置 `VITE_USE_MOCK_API=true`：

```bash
cd apps/web
npm ci
npm run dev
```

管理端入口为 `http://localhost:5173/admin/login`；模拟模式提供“进入模拟管理端”按钮。需要覆盖本地连接参数时，将 `apps/web/.env.example` 复制为 `apps/web/.env`。连接真实服务使用：

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

API 默认监听 `:8080`。`GET /api/healthz` 表示进程存活，`GET /api/readyz` 同时检查博客 MySQL、MinIO、Artalk 数据库和 Artalk HTTP 服务。服务启动时会自动、加锁执行数据库迁移。

不使用 Compose 单独启动后端时，需要预先创建 `blog-media` Bucket 并配置公开只读策略；API 凭据只负责对象读写，不负责创建 Bucket 或修改 Bucket 策略。

## GitHub OAuth

1. 在 GitHub 的 Developer settings 中创建 OAuth App；
2. Homepage URL 填生产站点，例如 `https://blog.example.com`；
3. Authorization callback URL 填 `https://blog.example.com/api/auth/github/callback`；
4. 将 Client ID、Client Secret 写入服务器 `.env`；
5. `ADMIN_GITHUB_ID` 必须填写站点所有者不可变的数字 GitHub ID，而不是用户名；
6. 使用至少 32 字节随机值作为 `OAUTH_STATE_SECRET`。

可用以下命令生成密钥：

```bash
openssl rand -hex 32
```

`ADMIN_GITHUB_ID` 是唯一能管理其他管理员权限的站点所有者。所有者可在“管理端 → 管理员权限”中按 GitHub 数字 ID 添加或移除普通管理员；授权变更会在下一次请求时立即生效。生产环境强制 HTTPS、Secure Cookie，并拒绝默认 MinIO 凭据。

管理员的 `POST`、`PUT`、`PATCH` 和 `DELETE` 请求会写入持久化审计事件，包含操作者、请求路径、HTTP 结果、来源 IP 和请求 ID，不包含请求正文、Cookie、Token 或密钥。只有站点所有者能在“管理端 → 操作审计”查询，后台任务默认删除一年前的记录。审计写入与具体业务事务并非原子提交；数据库不可用时原操作响应不会被审计写入失败覆盖，服务端会另行记录错误日志。

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
- `NGINX_SERVER_NAME`：小写域名，不带协议、端口或路径；生产环境必须与 `APP_ORIGIN` 的主机完全一致；
- `BACKEND_SUBNET`：Docker 内部网段；若服务器已有同网段网络，改为未使用的私有 `/24`；
- 四个互不复用的 MySQL 密码、两组互不复用的 MinIO 根/应用凭据和 `ARTALK_APP_KEY`；
- GitHub OAuth 四项配置；
- `TLS_FULLCHAIN_FILE` 和 `TLS_PRIVATE_KEY_FILE`：宿主机证书文件路径。

`BLOG_DB_PASSWORD`、`ARTALK_DB_PASSWORD` 和 `ARTALK_BRIDGE_DB_PASSWORD` 会进入 Go/MySQL DSN，必须使用至少 32 个 ASCII 字母或数字；推荐分别执行 `openssl rand -hex 32` 生成。其他值如包含 `$`，Docker Compose 的 `.env` 文件里需要写成 `$$`。先执行配置解析，确认没有缺失变量：

```bash
docker compose --env-file .env -f compose.yml -f compose.https.yml config --quiet
```

从旧版本升级且已经存在 MySQL 数据目录时，添加独立的 `ARTALK_BRIDGE_DB_PASSWORD` 后直接重建完整服务即可。一次性 `artalk-bridge-init` 会等待 Artalk 建好 `users` 表，再通过只在 MySQL 与该容器之间共享的 Unix Socket 自动创建/轮换桥接账号并授予表级 `SELECT, UPDATE`；API 会等待它成功退出，不需要开放 root 网络登录或手工执行 SQL。

旧版本的 `MINIO_ACCESS_KEY`/`MINIO_SECRET_KEY` 曾被同时用作根凭据。升级时保留它们作为 API 应用凭据，再新增一组不同的 `MINIO_ROOT_USER`/`MINIO_ROOT_PASSWORD`；`minio-init` 会在启动时自动创建最小权限用户，不需要手工进入 Console。

### 3. HTTPS 启动

`compose.https.yml` 会保留 80 端口用于跳转，并增加 443：

```bash
docker compose --env-file .env -f compose.yml -f compose.https.yml up -d --build
docker compose --env-file .env -f compose.yml -f compose.https.yml ps
```

证书续期或替换后必须重建 Web 容器，使只读证书挂载指向新文件：

```bash
./deploy/reload-web-certificate.sh
```

使用 Certbot 时可把它配置为续期部署钩子（项目默认使用根目录下的 `.env`）：

```bash
sudo certbot renew --deploy-hook '/实际项目路径/deploy/reload-web-certificate.sh'
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

备案完成前可把服务器入口仅绑定到回环地址，并通过 SSH 隧道验收：

```dotenv
APP_ENV=development
APP_ORIGIN=http://127.0.0.1:8088
HTTP_PORT=127.0.0.1:8088
SESSION_COOKIE_SECURE=false
```

服务器只启动基础 Compose，个人电脑建立隧道后访问 `http://127.0.0.1:8088`：

```bash
ssh -L 8088:127.0.0.1:8088 用户名@服务器IP
```

此阶段 GitHub OAuth App 的 Homepage/Callback 也要临时使用 `http://127.0.0.1:8088` 和 `http://127.0.0.1:8088/api/auth/github/callback`。不要把 `HTTP_PORT` 改成 `8088`（那会监听所有公网网卡），也不要在安全组开放 8088。

## Artalk

Artalk 使用同一 MySQL 实例中的独立 `artalk` 数据库和 `artalk_user`。默认开启验证码，并将新评论设为待审核；匿名评论附件默认关闭，避免绕过博客媒体上传策略。

评论使用与博客相同的 GitHub 登录。本站持久化 GitHub 数字 ID 与 Artalk 用户 ID 的映射；即使用户更换 GitHub 主邮箱，后续登录也会迁移原评论身份，不会创建第二个评论账户。站点所有者和其授权的管理员会自动同步为 Artalk 评论管理员，无需再执行 `artalk admin` 或维护另一套密码。评论审核、反垃圾和通知在 Artalk 控制中心完成；GitHub 评论用户的封禁和每日额度在博客“管理端 → 用户管理”中维护。

所有 Artalk 写请求以及携带 Artalk Token 的通知/审核读取都必须同时持有有效本站会话；网关会向 Artalk 核对 Token 对应邮箱与当前 GitHub 会话一致。退出本站后，即使浏览器或其他标签页仍残留 Artalk Token，也不能继续读取私有通知或执行管理操作。

公网入口按接口限制请求体：媒体上传 11 MiB、文章 JSON 3 MiB、站点设置 JSON 2 MiB、评论 1 MiB，其余 API 64 KiB。Go API 还会再次执行字段级限制，超限 JSON 统一返回 `413 REQUEST_TOO_LARGE`。

## MinIO 与媒体

浏览器不持有 MinIO 凭据。根凭据只进入一次性 `minio-init` 容器；Gin API 仅持有 `blog-media` Bucket 所需的独立应用凭据。上传必须经过管理员会话、CSRF、大小、扩展名、文件头、MIME、单边尺寸和总像素校验，再由 API 写入 Bucket。multipart 请求只能包含一个 `file` 文件，API 采用单次内存读取并最多同时处理两个上传，避免并发复制大文件导致容器 OOM。MinIO 请求不读取环境代理，连接、响应和单次操作均有截止时间；10 MiB 媒体固定使用单对象 PUT，进程中断不会留下数据库无法管理的 multipart 分片。Nginx 的 `/uploads/` 只允许公开读取；MinIO API 和 Console 不对公网开放。

媒体元数据和引用关系保存在 MySQL。上传先写入内部 `uploading` 记录，再写 MinIO 并激活；进程中断留下的记录和对象会由维护任务回收，避免生成无法管理的孤儿对象。被头像、Banner、文章封面或正文引用的图片不能删除；未引用图片先进入回收站，30 天后由后台任务永久清理。单个对象清理失败不会阻塞同批其他对象，失败项会保留在回收站供重试。回收站用于可恢复管理，不等于立即撤下对象：已有公开 URL 在永久删除前仍可访问。若对象已删除但数据库收尾失败，记录会以“清理未完成”继续显示在回收站，只允许重试永久清理，避免错误恢复一个已经不存在的对象。

MinIO 和 `mc` 由 `deploy/minio/Dockerfile` 从固定的上游提交构建，并强制使用项目锁定的 Go 安全补丁版本；不会使用浮动源码。该构建依赖较多，2GB 生产机应配置 Swap，最好在 CI 或本地完成镜像构建并推送到私有 Registry，再在服务器拉取相同摘要。每次修改固定提交或 Go 版本后都必须重新运行镜像漏洞扫描和备份恢复演练。

使用预构建镜像时，在 `.env` 中把 `API_IMAGE`、`WEB_IMAGE`、`MYSQL_IMAGE`、`MINIO_IMAGE` 设置为 Registry 的不可变 `@sha256:` 引用，然后执行：

```bash
docker compose --env-file .env -f compose.yml -f compose.https.yml pull
docker compose --env-file .env -f compose.yml -f compose.https.yml up -d --no-build
```

MySQL 使用 `deploy/mysql/Dockerfile` 基于固定的 8.4.10 LTS 摘要构建。镜像移除了服务端不使用的 MySQL Shell/Python 环境，并使用固定 gosu 1.19 源码和 Go 1.26.5 重建入口权限切换工具。

API 生产模式会独立校验 MySQL DSN、密码强度以及连接/读/写超时，数据库网络半断时不会无限占用连接池。每个新迁移执行前还会写入持久化 attempt marker；若 MySQL DDL 中途失败，后续启动会停止并要求先恢复升级前备份或人工核对 schema，不会自动盲目重跑。Compose 给 API 20 秒停止宽限期，其中应用最多使用 10 秒完成请求和后台任务收尾。

## 备份与恢复

创建备份：

```bash
BACKUP_DIR=/mnt/backup/myblog ./deploy/backup.sh
```

脚本在 Linux 上使用内核 `flock` 防止并发备份覆盖，进程被强制终止或主机重启也不会留下永久死锁；不支持 `flock` 的开发机使用保守目录锁。它以权限 `0600` 创建压缩包和 SHA-256 校验文件，包含 `blog`、`artalk` 两个 SQL 导出、`blog-media` Bucket、完整非敏感部署模板、Git 提交号、备份格式清单，以及实际运行容器的镜像引用和镜像 ID。为获得 MySQL 与 MinIO 一致的恢复点，脚本会短暂停止 API 和 Artalk 写入，完成或异常退出时自动恢复原本正在运行的服务；备份媒体只使用 API 的最小权限 Bucket 凭据，不使用 MinIO 根账号。

归档发布前会自动执行完整校验；复制或恢复前也可再次运行：

```bash
./deploy/verify-backup.sh /mnt/backup/myblog/myblog-时间戳.tar.gz
```

校验器验证 SHA-256、必需文件、非空 SQL、允许的顶层目录，并拒绝绝对路径、路径穿越、符号链接、硬链接和设备条目。备份不会把生产 `.env` 明文放进压缩包；生产密钥应单独保存在加密密码库中。执行后应立即确认 API 和 Artalk 已恢复健康，若脚本报告重启失败则先处理运行服务，再复制备份。

备份包含评论邮箱等敏感信息，传到服务器外之前必须再用 `age`、GPG 或等效方案加密。服务器本地默认保留 14 天，可通过 `BACKUP_RETENTION_DAYS` 修改；异地副本的保留策略由外部备份系统负责。

恢复演练必须在隔离环境进行：

1. 使用 `deploy/verify-backup.sh` 校验归档，再解压到隔离临时目录；
2. 用新的 `.env` 启动空 MySQL 和 MinIO；
3. 分别将 `blog.sql`、`artalk.sql` 导入对应数据库；
4. 用 `mc mirror --overwrite --remove` 将 `media/` 恢复到 `blog-media`；
5. 启动 API、Artalk 和 Web；
6. 验证 `/api/readyz`、GitHub 登录、管理权限、文章、评论、图片读取和上传；
7. 记录恢复时间和结果后销毁隔离环境。

云盘快照只能作为辅助，不能替代 SQL 导出、对象备份和异地副本。

## 质量检查

`.github/workflows/ci.yml` 会在 `main` 推送和 Pull Request 上自动执行以下门禁，并且不读取仓库 Secrets：

- 检查 Git 索引中没有 `.env`、私钥、运行数据、缓存、依赖和构建产物；
- 使用隔离 MySQL 8.4 真实执行 Go 集成测试和 1→14 迁移；
- 执行 Go race test、`go vet`、前端类型检查、Vitest、生产构建和 `npm audit`；
- 验证部署脚本、备份校验器、HTTP/HTTPS Compose；
- 在前述检查通过后构建 API 和 Web 容器镜像。

公开仓库应在 GitHub 分支保护中把该工作流设为 `main` 的必需检查；CI 通过只证明提交可构建，不会自动部署或接触生产凭据。

```bash
cd apps/api
go test ./...
go vet ./...
govulncheck ./...

cd ../web
npm ci
npm run type-check
npm test
npm run build
npm audit --audit-level=high
```

Compose 与脚本检查：

```bash
./deploy/check-repository.sh
./deploy/test-scripts.sh
docker compose --env-file .env -f compose.yml config --quiet
docker compose --env-file .env -f compose.yml -f compose.https.yml config --quiet
sh -n deploy/backup.sh deploy/verify-backup.sh deploy/check-secrets.sh deploy/reload-web-certificate.sh deploy/mysql/init-artalk.sh deploy/mysql/init-artalk-bridge.sh deploy/minio/init.sh
```

发布镜像还需要执行高危和严重漏洞门禁，例如：

```bash
trivy image --ignore-unfixed --severity HIGH,CRITICAL --exit-code 1 myblog-api
trivy image --ignore-unfixed --severity HIGH,CRITICAL --exit-code 1 myblog-web
trivy image --ignore-unfixed --severity HIGH,CRITICAL --exit-code 1 myblog-minio:RELEASE.2025-10-15T17-29-55Z
```

MySQL 和 Artalk 也必须使用 Compose 中的完整镜像引用扫描，不能只扫描应用自己构建的两个镜像。

## 安全运维要点

- `.env`、备份明文、数据库和云盘目录都不能提交 Git；
- 只允许密钥 SSH，关闭公网 MySQL、MinIO、Artalk、API 和 Docker API；
- 定期更新固定版本镜像和依赖，更新前先备份并在隔离环境验证；
- 证书续期后运行 `deploy/reload-web-certificate.sh`，让文件挂载重新解析到新证书；
- 定期测试 GitHub 登录、CSRF、评论审核、媒体引用保护和备份恢复；
- 中国大陆公开部署需完成 ICP 备案；取得公安联网备案号后，也可在站点设置中填写并自动展示官方查询链接。

## 第三方项目

- [saicaca/fuwari](https://github.com/saicaca/fuwari)：MIT，视觉和阅读体验参考；
- [ArtalkJS/Artalk](https://github.com/ArtalkJS/Artalk)：MIT，评论系统。

版权说明见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
