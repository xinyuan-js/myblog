import type { Category, PostDetail, SiteProfile, Tag } from '@/types/blog'
import bannerUrl from '@/assets/home-background.png'

export const mockSiteProfile: SiteProfile = {
  title: 'MyBlog',
  subtitle: '把复杂的事情慢慢说清楚',
  description: '记录工程实践、阅读笔记和生活里值得停留的瞬间。',
  avatarUrl: null,
  bannerUrl,
  authorName: '见山',
  authorBio: 'Go 开发者，长期主义练习生。喜欢可靠的软件，也喜欢不赶时间的文字。',
  aboutMarkdown: '# 你好，我是见山。\n\nGo 开发者，长期主义练习生。\n\n## 关于这个博客\n\n这里记录工程实践、阅读笔记和生活里值得停留的瞬间。',
  socialLinks: [
    { label: 'GitHub', url: 'https://github.com/', icon: 'github' },
    { label: 'Email', url: 'mailto:hello@example.com', icon: 'mail' },
  ],
  icpNumber: null,
}

export const mockTags: Tag[] = [
  { id: 1, name: 'Go', slug: 'go', postCount: 3 },
  { id: 2, name: '工程实践', slug: 'engineering', postCount: 4 },
  { id: 3, name: '数据库', slug: 'database', postCount: 2 },
  { id: 4, name: '随笔', slug: 'essay', postCount: 2 },
  { id: 5, name: 'Vue', slug: 'vue', postCount: 1 },
]

export const mockCategories: Category[] = [
  { id: 1, name: '技术', slug: 'technology', description: '软件工程与实践记录', postCount: 5 },
  { id: 2, name: '生活', slug: 'life', description: '日常观察与生活切片', postCount: 2 },
  { id: 3, name: '阅读', slug: 'reading', description: '书籍、文章和思考', postCount: 1 },
]

const goCategory = mockCategories[0]!
const lifeCategory = mockCategories[1]!
const readingCategory = mockCategories[2]!

export const mockPosts: PostDetail[] = [
  {
    id: 1,
    title: '从一个可靠的 Go 服务开始',
    slug: 'building-a-reliable-go-service',
    excerpt: '在业务代码之前，先把配置、生命周期、错误处理和可观测性这些地基搭好。',
    coverUrl: null,
    status: 'published',
    publishedAt: '2026-07-18T09:30:00+08:00',
    updatedAt: '2026-07-20T21:10:00+08:00',
    category: goCategory,
    tags: [mockTags[0]!, mockTags[1]!],
    wordCount: 2860,
    readingTimeMinutes: 9,
    contentMarkdown: `# 从一个可靠的 Go 服务开始

写一个能够启动的 HTTP 服务并不难，难的是让它在半年后仍然容易理解，在配置错误、数据库抖动和发布中断时仍然给出明确反馈。

## 先定义生命周期

服务启动时依次完成配置读取、依赖初始化和路由注册。任一步失败都应该立即退出，而不是带着不完整状态继续运行。

\`\`\`go
func run(ctx context.Context, cfg Config) error {
    db, err := openDatabase(cfg.Database)
    if err != nil {
        return fmt.Errorf("open database: %w", err)
    }
    defer db.Close()

    server := newHTTPServer(cfg, db)
    return server.ListenAndServe(ctx)
}
\`\`\`

### 让错误携带上下文

错误应该回答“哪个动作失败了”，同时保留原始错误供日志和监控判断。对外 API 则返回稳定的错误码，不把数据库细节暴露给浏览器。

## 配置不是全局变量

配置在程序入口完成解析和校验，然后显式传给需要它的组件。这样测试不依赖进程环境，也不会出现某个包在初始化阶段偷偷读取环境变量。

## 最后才是业务路由

当地基足够安静，文章、标签和认证这些业务代码才有清晰的位置。可靠性不是上线前补的一层外壳，而是项目结构本身。`,
    previousPost: { title: 'MySQL 迁移应该由谁负责', slug: 'who-owns-database-migrations' },
    nextPost: null,
  },
  {
    id: 2,
    title: 'MySQL 迁移应该由谁负责',
    slug: 'who-owns-database-migrations',
    excerpt: '把迁移脚本纳入版本控制，并让部署流程对数据库变化保持敬畏。',
    coverUrl: null,
    status: 'published',
    publishedAt: '2026-07-11T20:00:00+08:00',
    updatedAt: '2026-07-12T10:20:00+08:00',
    category: goCategory,
    tags: [mockTags[1]!, mockTags[2]!],
    wordCount: 1940,
    readingTimeMinutes: 6,
    contentMarkdown: `# MySQL 迁移应该由谁负责

数据库结构和应用代码是同一个版本的一部分。迁移脚本必须进入仓库，并且能够按顺序、可追踪地执行。

## 不依赖 ORM 自动改表

自动迁移适合原型，却很难表达删除字段、回填数据和分阶段发布。生产环境需要显式 SQL 和迁移记录表。

## 向前兼容的发布顺序

先增加新结构，再发布同时兼容新旧结构的程序，完成数据回填后才能删除旧结构。这比一次性替换慢，却给回滚留下了空间。`,
    previousPost: { title: '雨停之后去散步', slug: 'a-walk-after-rain' },
    nextPost: { title: '从一个可靠的 Go 服务开始', slug: 'building-a-reliable-go-service' },
  },
  {
    id: 3,
    title: '雨停之后去散步',
    slug: 'a-walk-after-rain',
    excerpt: '城市在雨后短暂地慢下来，路灯、树叶和便利店都有了更清晰的边缘。',
    coverUrl: null,
    status: 'published',
    publishedAt: '2026-06-29T22:15:00+08:00',
    updatedAt: '2026-06-29T22:15:00+08:00',
    category: lifeCategory,
    tags: [mockTags[3]!],
    wordCount: 780,
    readingTimeMinutes: 3,
    contentMarkdown: `# 雨停之后去散步

雨是在晚饭后停的。窗外没有夕阳，云层却比白天亮了一些。

## 一段没有目的地的路

不带耳机，不看步数，也不急着决定下一个路口。偶尔没有目标，注意力才会重新回到身边。

回家时鞋底沾了一点水，脑子里那些纠缠了一整天的问题倒是松开了。`,
    previousPost: { title: '重新理解“简单”', slug: 'understanding-simplicity-again' },
    nextPost: { title: 'MySQL 迁移应该由谁负责', slug: 'who-owns-database-migrations' },
  },
  {
    id: 4,
    title: '重新理解“简单”',
    slug: 'understanding-simplicity-again',
    excerpt: '简单不是功能少，而是重要关系可以被人看见、解释和维护。',
    coverUrl: null,
    status: 'published',
    publishedAt: '2026-06-15T13:00:00+08:00',
    updatedAt: '2026-06-17T08:40:00+08:00',
    category: readingCategory,
    tags: [mockTags[1]!, mockTags[3]!],
    wordCount: 1210,
    readingTimeMinutes: 4,
    contentMarkdown: `# 重新理解“简单”

很多复杂系统最初都来自合理的小决定。真正困难的不是拒绝功能，而是持续整理这些决定之间的关系。

## 简单是一种可解释性

如果一个新成员能沿着代码找到数据从哪里来、在哪里改变、失败时如何返回，那么系统即使功能很多，也仍然可以是简单的。

## 为未来保留删除的能力

模块之间边界清楚，意味着某个功能不再需要时可以被完整拿走。可删除性往往比可扩展性更能检验设计。`,
    previousPost: null,
    nextPost: { title: '雨停之后去散步', slug: 'a-walk-after-rain' },
  },
  {
    id: 5,
    title: 'Vue 页面如何与 API 契约一起生长',
    slug: 'vue-and-api-contracts',
    excerpt: '先让页面用真实形状的模拟数据运行，再把已验证的数据需求交给后端。',
    coverUrl: null,
    status: 'draft',
    publishedAt: null,
    updatedAt: '2026-07-21T18:30:00+08:00',
    category: goCategory,
    tags: [mockTags[1]!, mockTags[4]!],
    wordCount: 960,
    readingTimeMinutes: 3,
    contentMarkdown: '# Vue 页面如何与 API 契约一起生长\n\n这是一篇尚未发布的草稿。',
    previousPost: null,
    nextPost: null,
  },
]
