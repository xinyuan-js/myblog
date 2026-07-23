# MyBlog 安全审查

审查日期：2026-07-23  
范围：Vue SPA、Gin API、GitHub OAuth、MySQL、MinIO、Artalk、Nginx、Docker Compose、备份恢复和 Git 交付边界。

## 结论状态

应用发布门禁已关闭。业务代码、前端依赖、Go 可达漏洞、认证授权、上传边界、数据隔离、容器镜像、运行态权限矩阵和隔离恢复均已通过验证。最终五个运行镜像使用同一份有效期内的 Trivy 数据库逐个扫描，可修复的 HIGH/CRITICAL 均为 0；完整 Compose 栈已使用这些镜像重建并保持健康。

这表示当前制品具备上线条件，不表示互联网基础设施自动安全。正式开放前仍需由运维完成真实域名证书、GitHub OAuth 生产回调、异地加密备份、主机防火墙与补丁、ICP 备案等外部事项。

## 威胁边界

- 公网入口只有 Nginx 的 80/443；API、MySQL、MinIO 和 Artalk 不发布宿主机端口。
- 访客可读取公开文章、媒体和评论，并可通过 Artalk 发表评论。
- 所有用户通过 GitHub OAuth 登录；配置中的不可变数字 GitHub ID 是站点所有者，可授权其他 GitHub 数字 ID 为管理员。
- 管理写请求必须同时通过数据库会话、Origin 和 CSRF Token 校验。
- 浏览器不持有 GitHub Client Secret、MinIO 凭据或管理员 Bearer Token。
- 用户提交的 Markdown、评论内容、文件名、图片和上游 OAuth/API 响应均视为不可信输入。
- 单机云盘故障、宿主机失陷和 Docker 管理权限失陷不在应用容器能够自行恢复的范围内，必须依赖异地加密备份和主机加固。

## 已验证控制

### 身份与会话

- 不存在注册、密码登录或找回密码接口。
- OAuth state 带 HMAC、过期时间、一次性数据库记录和 SameSite Cookie。
- 会话随机值只保存在 HttpOnly Cookie，数据库只保存 SHA-256 哈希。
- Cookie 使用 `HttpOnly`、生产环境 `Secure` 和 `SameSite=Lax`。
- 每次管理员请求都从配置所有者和数据库授权名单重新计算权限；授权和撤销无需重新登录即可生效。
- 只有配置中的站点所有者可调用管理员权限接口，且所有者不能通过接口删除。
- 所有管理写接口和退出登录均验证 Origin 与 CSRF。
- OAuth 返回路径只允许站内相对路径；普通用户请求管理路径时会返回首页，防止开放重定向和越权跳转。
- 登出只删除本站数据库会话和本站 Cookie，不操作 GitHub 登录；失败的 Origin/CSRF 校验不会提前清除 Cookie。
- 评论 SSO Token 带用途、HMAC 和五分钟有效期；Artalk 管理员身份由当前博客权限同步，撤销博客管理员时同步撤销评论审核权限。

### 输入、内容与上传

- Markdown 禁止原生 HTML，并经过 DOMPurify 清洗。
- 社交链接在前端统一过滤，只允许绝对 HTTP/HTTPS URL，拒绝 `javascript:`、`data:` 和相对路径。
- 上传请求和 JSON 请求均有限制体积。
- 图片同时校验扩展名、声明 MIME、文件头、可解码格式、尺寸和 10MiB 上限。
- 对象键由加密随机值生成，不使用原始文件名作为路径。
- 正在被头像、Banner、封面或正文引用的媒体不能删除；未引用媒体先进入回收站。
- MinIO 管理端点和 Console 不对公网开放；`/uploads/` 仅允许公开读取 Bucket 对象。

### 数据、网络与运行时

- `blog` 和 `artalk` 使用不同数据库及账号，MySQL 不发布 3306。
- API 访问 Artalk 仅使用 `blog_artalk_bridge`，其权限限定为 `SELECT, UPDATE ON artalk.users`。
- API、Artalk 和 MinIO 使用只读根文件系统、`no-new-privileges`、能力集删除、PID/内存限制和 tmpfs。
- Nginx 配置 CSP、HSTS（HTTPS）、frame、MIME、Referrer 和 Permissions Policy。
- API 配置读取、写入和空闲超时；固定窗口限流器的客户端状态上限为 4096。
- Nginx 对全部 API 和 Artalk 评论入口按客户端 IP 限流；登录和上传另有 Gin 应用层限流。
- 容器日志限制为 10MiB × 3，避免磁盘无限增长。
- Nginx 访问日志只记录 URI，不记录查询参数或 Referrer；实测 OAuth/JWT 查询标记未进入日志。
- 生产配置拒绝 HTTP Origin、非 Secure Cookie、默认 MinIO 凭据、弱 OAuth state 密钥和错误 OAuth 回调来源。

### 备份与交付

- 备份包含两个 SQL 数据库、`blog-media` Bucket 和非敏感部署模板。
- 生产 `.env` 不进入备份压缩包；备份以 `0600` 创建并生成 SHA-256 校验文件。
- 已完成空 Bucket、非空 Bucket 和隔离恢复演练；异地备份仍属于生产运维责任。
- Git HEAD 不包含 `.env`、缓存、`node_modules`、`dist`、数据库、MinIO 数据或二进制编译产物。

## 已执行验证

| 检查 | 结果 |
| --- | --- |
| `go test -count=1 ./...` | 通过 |
| `go vet ./...` | 通过 |
| `govulncheck ./...` | 0 个可达漏洞（2026-07-23，包含 Goldmark） |
| Vue 严格类型检查 | 通过 |
| Vitest | 19 项通过 |
| Vite 生产构建 | 通过 |
| `npm audit --omit=dev --audit-level=high` | 0 个漏洞（2026-07-23） |
| OpenAPI 与 Gin 路由一致性测试 | 通过 |
| HTTP/HTTPS Compose 解析 | 通过 |
| HTTP Nginx 语法检查 | 通过 |
| Shell 脚本语法检查 | 通过 |
| 桌面端和 390px 移动端浏览器验收 | 通过，无控制台错误或横向溢出 |
| 最终运行栈 MySQL/MinIO/API/Artalk/Web 健康检查 | 通过 |
| 管理端文章、分类、标签、媒体、引用保护端到端测试 | 通过 |
| 权限矩阵（未登录/普通用户/管理员/所有者） | 401/403/200 边界符合预期 |
| 动态管理员授权与撤销 | `403 → 201 → 200 → 204 → 403`，无需重新登录 |
| CSRF/Origin/登出运行态测试 | 缺失或错误来源 403；正确 CSRF 到达处理器；登出后会话 401 |
| 内部 Artalk userinfo 公网暴露检查 | Nginx 仅返回 SPA HTML，不代理内部接口 |
| 敏感查询参数日志检查 | 标记出现次数 0 |
| 真实备份与隔离恢复 | SHA-256 通过；文章 10、媒体 3、评论 1、评论用户 2 均一致 |

## 镜像审计记录

初扫使用 2026-07-22 更新的 Trivy 数据库，并只统计有修复版本的 HIGH/CRITICAL：

| 镜像 | 初扫发现 | 处理 |
| --- | ---: | --- |
| `myblog-api` | 12 HIGH | `x/crypto`、`x/net`、`x/sys` 升级到修复版本 |
| `myblog-web`（旧 Nginx 1.28） | 32 HIGH、2 CRITICAL | 基础镜像升级到 Nginx 1.31.3 Alpine |
| MySQL 8.4.5 | 多个 OS/Python/gosu 高危及严重项 | 基于固定 MySQL 8.4.10 摘要构建；移除未使用的 513MiB MySQL Shell/Python 环境，以 Go 1.26.5 重建固定 gosu 1.19 |
| MinIO 2025-04-22 | 多个 OS、Go 标准库和依赖高危及严重项 | 固定 2025-10-15 MinIO/MC 源码提交、Go 1.26.5 和 Alpine 3.24.1，并固定升级存在公告的 Go 模块 |
| Artalk 2.9.1 稳定镜像 | 55 HIGH、6 CRITICAL | 固定官方 nightly 提交 `75a35cc` 的镜像摘要；该替代镜像复扫为 0 |

最终复扫使用 Trivy DB `UpdatedAt=2026-07-22T19:04:49Z`（扫描时仍在 `NextUpdate` 有效窗口内），且设置 `--ignore-unfixed --severity HIGH,CRITICAL --exit-code 1`：

| 最终镜像 | 可修复 HIGH/CRITICAL |
| --- | ---: |
| `myblog-api:latest` | 0 |
| `myblog-web:latest` | 0 |
| `myblog-minio:RELEASE.2025-10-15T17-29-55Z` | 0 |
| `myblog-mysql:8.4.10-hardened` | 0 |
| `artalk/artalk-go:nightly` 固定摘要 | 0 |

## 上线运维清单

以下项目依赖真实服务器或第三方账号，不属于本地应用制品：

1. 用真实域名、GitHub OAuth App 和正式证书执行一次 HTTPS 验收，检查 HSTS、Secure Cookie 和 OAuth 回调。
2. 只开放 80/443，并启用主机自动安全更新、SSH 密钥登录和云厂商防火墙。
3. 将 SQL 和媒体备份用 `age`、GPG 或等效方案加密后复制到服务器之外，并定期从异地副本恢复。
4. 为日志、磁盘、证书到期、备份失败和容器健康配置告警。
5. 中国大陆服务器正式开放前完成 ICP 备案，并确认评论审核与通知邮箱可用。
