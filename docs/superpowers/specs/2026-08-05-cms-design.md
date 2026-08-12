# Dulizhan CMS 设计文档

- 日期：2026-08-05
- 状态：已确认（用户逐节审阅通过）

## 1. 目标

用 Go + Gin 实现一个通用的 CMS 系统，用于个人/公司官网场景。核心诉求：

- **通用**：支持可视化内容类型构建器，运行时定义任意内容类型并动态生效
- **可换主题**：前台服务端渲染模板主题，主题是一个目录，切换配置即生效
- **SEO 友好**：非 SPA，服务端渲染；内置 sitemap、robots、hreflang、canonical、Open Graph、JSON-LD
- **多语言**：内容级多语言，URL 语言前缀，hreflang 互联

## 2. 架构总览

单体单二进制。Vue 管理端 SPA 构建后 `go:embed` 进二进制，运行零依赖。

```
dulizhan/
├── cmd/dulizhan/main.go        # 入口：加载配置、启动 HTTP
├── config.yaml                 # 站点配置（语言、主题、存储、端口）
├── internal/
│   ├── config/                 # 配置加载（YAML + 环境变量覆盖）
│   ├── server/                 # Gin 路由装配、中间件编排
│   ├── schema/                 # 内容类型 schema 模型 + 字段类型定义 + 校验规则
│   ├── content/                # 内容 CRUD 服务、发布状态、翻译管理
│   ├── store/                  # repository 接口 + SQLite/PG/MySQL 实现
│   ├── theme/                  # 主题加载器、模板引擎、模板函数
│   ├── i18n/                   # 语言注册表、URL 前缀、hreflang 生成
│   ├── auth/                   # 用户、会话、RBAC 中间件
│   ├── media/                  # MediaStore 接口：本地磁盘 + S3/MinIO
│   ├── seo/                    # sitemap、robots、canonical、OG、JSON-LD
│   └── adminapi/               # 管理端 REST API（供 Vue 调用）
├── admin/                      # Vue 3 + TS + Vite 管理端 SPA（go:embed 进二进制）
├── themes/
│   └── default/                # 默认主题：theme.yaml + templates/ + static/ + locales/
└── docs/
```

**三条路由面：**
- 公开前台：`GET /` `/{lang}/` `/article/...` —— 服务端渲染模板，SEO 核心
- 管理 API：`/api/*` —— 给 Vue 管理端调用的 REST/JSON
- 管理界面：`/admin` —— 内嵌 SPA

**设计原则：** 模块间通过接口通信（store、media、theme 均为可插拔点），内部实现可独立替换；模块间不互相引用实现。

## 3. 数据模型与存储

**核心设计：动态 schema + JSON 文档存储。** 内容类型 schema 存数据库，内容本体按 schema 序列化为 JSON 文档存通用表。动态增删字段无需迁移表结构，SQLite/PG/MySQL 行为一致。

### 数据表

| 表 | 用途 | 关键字段 |
|---|---|---|
| `content_types` | 内容类型定义 | id, name(如 article), label, fields(JSON), config |
| `content` | 内容实体 | id, content_type_id, content_id(翻译分组), lang, slug, status(draft/published), created_by, published_at, payload(JSON 字段值) |
| `users` | 用户 | id, username, password_hash, role_id |
| `roles` | 角色 | id, name, permissions(JSON 权限点列表) |
| `sessions` | 会话 | token, user_id, expires_at |
| `menus` | 导航菜单 | id, name, lang, items(JSON) |
| `settings` | 站点配置 | key, value —— 语言列表、默认语言、当前主题、站点名称等 |
| `media` | 媒体元数据 | id, filename, url, mime, size, created_at |

### 字段类型（schema 引擎内置，可扩展）

`text` `textarea` `richtext(markdown/html)` `number` `boolean` `date` `datetime` `select` `multiselect` `image(媒体引用)` `file(附件)` `slug` `relation(关联其他内容)` `repeat(可重复组)`。

每字段配置：校验规则（必填/长度/正则）、默认值、是否可翻译。

### 多语言模型

同一内容在 `content` 表按 `(content_id, lang)` 一行一语言，`content_id` 为翻译组关联键；slug 按语言唯一。翻译可见性由前台主题控制。

### 查询取舍

自定义字段存 JSON。常用排序字段（标题/时间等）在 schema 中标记为"索引列"，冗余到通用列以便列表查询。不做复杂字段级过滤——官网场景不需要。

## 4. 前台主题系统

主题 = 一个目录，站点设置中 `theme` 配置切换。开发模式热重载，生产启动时加载一次。

```
themes/default/
├── theme.yaml            # 主题清单
├── templates/
│   ├── index.html        # 首页
│   ├── single.html       # 内容详情（按 content_type 映射）
│   ├── list.html         # 内容列表/归档
│   ├── page.html         # 静态页面
│   ├── search.html       # 搜索
│   ├── partials/         # header/footer/nav 等公共片段
│   └── layouts.html
├── static/               # CSS/JS/图片，通过 /themes/{name}/ 路由
└── locales/              # zh.yaml, en.yaml —— 主题自身 UI 文案
```

### theme.yaml 关键内容

```yaml
name: default
version: 1.0.0
templates:
  index: index.html
  single: single.html
  list: list.html
type_map:                   # 内容类型 → 模板映射（可多个类型共用一个模板）
  article: single.html
  page: page.html
fields:                     # 主题声明支持的字段类型
  - text
  - richtext
```

### 模板函数（Go html/template 扩展）

- `t "menu.home"` —— 界面文案翻译
- `url "article" .` —— 生成带语言前缀的 URL
- `media .cover` —— 媒体 URL 解析
- `asset "/css/main.css"` —— 带版本号的静态资源 URL
- `hreflang .` —— 渲染翻译语言切换链接

### 渲染流程

请求 → URL 解析出语言前缀 → 查内容（按 slug + lang）→ 定位主题模板（type_map）→ 渲染 HTML + SEO 标签。

### 主题切换

改 `settings.theme` 即生效。主题不认的内容类型自动用通用模板回退。

## 5. 管理后台与管理 API

### 技术栈

Vue 3 + TypeScript + Vite，构建产物 `go:embed` 进二进制，`/admin` 路由托管。API 与 SPA 同源，`/api/*` 前缀。

### 管理 API（REST + JSON）

- `/api/auth/login` `logout` `me` —— 会话与当前用户
- `/api/content-types` —— schema 的 CRUD（构建器读写）
- `/api/content` —— 内容 CRUD + 翻译管理 + 状态切换
- `/api/media` —— 上传、列表、删除
- `/api/menus` —— 导航管理
- `/api/settings` —— 站点配置
- `/api/users` `/api/roles` —— 用户与角色管理
- `/api/search` —— 内容检索（管理端）

### SPA 界面模块

1. 登录 —— 账号密码
2. 仪表盘 —— 内容统计、最近发布
3. 内容类型构建器 —— 可视化定义字段（添加字段、选类型、设校验、标记可翻译/索引），保存即运行时生效
4. 内容管理 —— 列表（筛选/搜索/分页）、动态表单编辑（按 schema 渲染）、多语言 Tab 切换、草稿/发布切换
5. 媒体库 —— 上传、预览、选择引用
6. 导航菜单 —— 可视化编排菜单项
7. 用户与角色 —— 用户 CRUD、角色权限点勾选
8. 设置 —— 站点信息、语言开关、主题切换、存储配置查看

### 动态表单引擎（SPA 核心）

通用渲染组件，输入字段 schema，输出表单控件 + 校验。每种字段类型对应一个表单组件，新增字段类型只加组件注册表，不改表单引擎。

## 6. 认证与权限（RBAC）

### 认证

管理员登录后签发会话（随机 token 存 `sessions` 表 + HttpOnly Cookie），中间件校验；密码 bcrypt 存储。前台公开访问无认证。

### 内置角色

| 角色 | 权限 |
|---|---|
| 管理员 admin | 全部权限 |
| 编辑 editor | 所有内容类型 CRUD + 发布 + 媒体 + 菜单 + 内容类型管理 |
| 作者 author | 创建/编辑自己的内容，不能发布（草稿提交，待编辑发布） |

### 权限点

- `content.read/write/publish/delete`（可细化到具体内容类型）
- `content_types.manage`
- `media.upload/delete`
- `menus.manage`
- `settings.manage`
- `users.manage`
- `roles.manage`

### 中间件

`RequireAuth` → `RequirePerm("content.publish")` 两层，作用于 `/api/*` 路由分组。作者的内容归属校验在 service 层（只能操作 `created_by` 自己的记录）。未登录返回 401，无权限返回 403。

## 7. 媒体存储

### 接口抽象（可插拔）

```go
type MediaStore interface {
    Save(ctx, key string, r io.Reader, opts ...) (MediaInfo, error)
    Delete(ctx, key string) error
    URL(key string) string
}
```

- **LocalStore**：存 `{data_dir}/media/{key}`，`/media/{key}` 静态路由访问（带 Cache-Control）
- **S3Store**：兼容 S3 协议（AWS S3 / MinIO / 阿里 OSS），改配置切换
- 配置：`media.driver: local | s3`

### 媒体库功能

- 上传（限制类型与大小）、列表（分页 + 类型筛选）、删除（仅当无内容引用时）
- 图片生成缩略图（本地内置，S3 用对象存储自带或跳过）
- 元数据存 `media` 表；schema 的 `image/file` 字段存媒体 id，模板函数 `media` 解析 URL

## 8. SEO 特性

### 每内容 SEO 字段

schema 构建器中可选"SEO 组"字段，选中后内容表单出现：独立 title / description / canonical / Open Graph（og:title/description/image/type）。未填写时自动回退到标题 + 摘要。

### 自动生成能力

1. **sitemap.xml** —— `/sitemap.xml`，遍历所有已发布内容 + 所有语言变体，`lastmod` 用 `published_at`
2. **robots.txt** —— `/robots.txt`，指向 sitemap，管理端可配置 Disallow 规则
3. **hreflang** —— 每个页面 `<head>` 输出所有语言版本 `<link rel="alternate" hreflang="zh">`，含 `x-default`
4. **canonical** —— 默认当前完整 URL，可被内容 SEO 字段覆盖
5. **JSON-LD** —— 根据内容类型输出 `Article` / `BreadcrumbList` / `Organization`（站点级配置）
6. **语义化 HTML** —— 主题负责 `article`/`time`/`h1` 等标签，框架提供 URL 规范化、语言前缀处理、分页标签

### 多语言 URL 约定

- 默认语言无前缀（如 `/article/slug`），其他语言带前缀（`/en/article/slug`）——可配置
- 非法语言前缀 → 404；`/` 重定向到当前语言首页

## 9. 错误处理与测试

### 错误处理

- service 层返回领域错误（`ErrNotFound` / `ErrForbidden` / `ErrValidation`），HTTP 层统一映射：404 / 403 / 422 + JSON `{code, message}`
- 前台页面错误 → 主题模板渲染（主题内可选 404.html/500.html，缺失用内置兜底）
- 日志分级：访问日志 + 错误日志（slog），生产输出 JSON
- 前后端双校验：前端即时提示，后端权威

### 测试策略

- store 层：repository 契约测试，SQLite 内存库跑；PG/MySQL 留集成测试标记
- schema 引擎：字段校验规则表驱动单测
- content service：多语言、发布状态、权限归属单测（mock repository）
- theme 渲染：模板渲染快照测试 + 主题模板冒烟测试
- auth：会话、RBAC 权限点中间件测试
- admin API：httptest 端到端
- SEO：sitemap 内容、hreflang 输出断言

## 10. 实现阶段划分

**阶段 1 —— 骨架与前台渲染**
项目结构、配置加载、Gin 装配、store 层（SQLite）+ repository 接口；内容类型固定起步（article/page）JSON 存储，schema 引擎 v1（字段类型 + 校验）；主题系统 + 默认主题 + 语言前缀 URL + hreflang + sitemap/robots + OG。验收：跑通"配置主题 → 写文章 → SEO 友好的前台页面"。

**阶段 2 —— 管理 API 与基础管理功能**
会话认证 + RBAC 中间件；内容 CRUD API + 翻译管理 + 草稿/发布；菜单、设置、媒体（本地）API。验收：通过 API 完成内容完整生命周期管理。

**阶段 3 —— Vue 管理端 SPA**
登录、内容列表、动态表单编辑器（多语言 Tab）、媒体库、菜单、设置。验收：浏览器全程可视化操作内容。

**阶段 4 —— 可视化内容类型构建器**
SPA schema 构建器 + 动态表单引擎完整化；新增字段类型（relation/repeat 等）；权限点细化。验收：后台定义全新内容类型，前台主题通过 type_map 渲染。

**阶段 5 —— 收尾加固**
用户/角色界面、S3 媒体实现、主题热重载完善、PG/MySQL 验证；测试补全、文档、Dockerfile、单二进制打包。
