# Dulizhan CMS — 阶段 3：Vue 管理端 SPA 设计

- 日期：2026-08-06
- 状态：已确认（用户逐节审阅通过）
- 前置文档：`docs/superpowers/specs/2026-08-05-cms-design.md`（权威设计）、`docs/superpowers/specs/2026-08-06-restore-phase1-phase2-design.md`（阶段 1/2 恢复基线）
- 基线：阶段 1/2 源码已恢复并验收通过，`go test -count=1 ./...` 全绿，可作为阶段 3 开发基线

## 1. 目标与范围

构建 Vue 3 管理端 SPA，让**浏览器全程可视化操作内容**（设计文档阶段 3 验收点）。

**包含**：
- 登录（Bearer token）
- 仪表盘（内容统计 + 最近发布）
- 内容搜索
- 内容管理（列表筛选/搜索/分页、动态表单编辑、多语言 Tab、草稿/发布切换）
- 媒体库（上传/预览/删除/选择引用）
- 菜单管理
- 设置

**排除**（属阶段 4/5）：
- 可视化内容类型构建器（阶段 4）
- 用户/角色管理界面（阶段 5）
- relation/repeat 字段类型（阶段 4 增强）

**动态表单字段类型范围**：text/textarea/richtext/number/boolean/date/datetime/select/multiselect/image/file/slug（共 12 种；relation/repeat 除外，属阶段 4）。

## 2. 技术栈与集成

| 项 | 选择 | 理由 |
|---|---|---|
| 框架 | Vue 3 + TypeScript + Vite | 设计文档既定 |
| UI 库 | Element Plus | Vue 3 生态最成熟，表格/表单/分页/上传/弹层组件齐全 |
| 路由/状态 | Vue Router + Pinia | Vue 3 标准组合，登录守卫 + 轻量状态 |
| 富文本 | wangEditor 5（`@wangeditor/editor-for-vue`） | 内容正文用富文本；国产开箱即用、无 SSR 依赖、输出 HTML 与 `data.content` 契约一致；前台 `raw` 渲染 |
| 目录 | 仓库内 `admin/` | 单体单二进制 |
| 嵌入 | `vite build` 产物 `go:embed` 进二进制，`/admin` 托管 | 设计文档既定 |

### 目录结构

```
admin/
├── index.html
├── vite.config.ts          # dev 代理 /api /media /themes → 后端 :8080
├── package.json  tsconfig.json
└── src/
    ├── main.ts             # 挂载 App + Router + Pinia + Element Plus
    ├── App.vue             # 布局壳（侧边栏 + 顶栏 + router-view）
    ├── api/                # HTTP 客户端封装 + 各资源 API
    │   ├── client.ts       # fetch 封装：baseURL=/api、Bearer token、401→跳登录、错误 {code,message} 归一
    │   ├── auth.ts  content.ts  media.ts  menu.ts  settings.ts  search.ts  stats.ts
    ├── stores/
    │   ├── auth.ts         # token(localStorage) + user + role 两级
    │   └── settings.ts     # 站点设置缓存
    ├── router/index.ts     # 路由表 + 登录守卫
    ├── dynamic-form/       # 动态表单引擎（见第 3 节）
    ├── views/
    │   ├── LoginView.vue
    │   ├── DashboardView.vue
    │   ├── ContentListView.vue
    │   ├── ContentEditView.vue    # 含多语言 Tab
    │   ├── MediaLibraryView.vue
    │   ├── MenuManageView.vue
    │   └── SettingsView.vue
    └── components/
        ├── LayoutSidebar.vue
        └── MediaPicker.vue         # 弹层选媒体，被表单控件与媒体库共用
```

### 集成细节

- **开发**：`vite dev`（默认 5173），`vite.config.ts` 代理 `/api`、`/media`、`/themes` 到 `http://localhost:8080`。后端用 `go run ./cmd/dulizhan` 起在 8080。
- **生产**：`vite build` → `admin/dist`，构建流程复制到 `internal/server/dist/`。`internal/server` 的 `embed.go` 声明 `//go:embed dist/*`，注册：
  - `GET /admin` 与 `GET /admin/*`（非 `/admin/assets/*`）→ 返回 `index.html`（前端路由 fallback）
  - `GET /admin/assets/*` → 从嵌入 FS 服务静态资源（带正确 MIME）
- **go:embed 约束**：`//go:embed` 只能嵌声明它的 Go 文件所在目录内的相对路径。方案：`admin/` 下新建 `internal/server/dist/` 目录，构建流程把 `vite build` 产物（`admin/dist`）复制到 `internal/server/dist/`；`internal/server/embed.go`（`package server`）声明 `//go:embed dist/*`。构建顺序：`vite build` → 复制产物 → `go build`。开发时 dist 可能为空，`/admin` 仅在产物存在时注册（embed FS 空则不注册 admin 路由，避免启动报错）。

## 3. 认证与权限

- **认证**：`POST /api/auth/login` 拿 token → 存 `localStorage`（key `dlz_token`）；`client.ts` 每次请求带 `Authorization: Bearer <token>`；响应 401 → 清 token → 跳 `/login`。退出调 `POST /api/auth/logout` + 清 token。
- **当前用户**：登录后 `GET /api/auth/me` 取 `{user:{id, username, role}}`，存 Pinia auth store。
- **权限模型（角色名两级）**：
  - admin → 显示全部菜单（仪表盘/内容/媒体/菜单/设置）
  - 非 admin（editor/author）→ 隐藏 设置 菜单；内容页根据 role 控制操作：editor 可见 发布/撤回 按钮；author 不可见（后端仍权威校验，越权返回 403 时前端提示）
- **路由守卫**：未登录访问受保护路由 → 重定向 `/login`；已登录访问 `/login` → 重定向首页。

## 4. 动态表单引擎（SPA 核心）

独立可复用层，输入内容类型 schema（字段列表），输出表单 + 校验。阶段 4 构建器直接复用。

```
dynamic-form/
├── DynamicForm.vue      # 入口：遍历字段 → 注册表查控件渲染 + 校验 + 值收集
├── registry.ts          # 字段类型 → {component, validate} 注册表
├── types.ts             # FieldType / SchemaField / FormValue
└── controls/
    ├── TextControl.vue  TextareaControl.vue  RichTextControl.vue
    ├── NumberControl.vue  BooleanControl.vue  DateControl.vue  DateTimeControl.vue
    ├── SelectControl.vue  MultiSelectControl.vue  SlugControl.vue
    ├── ImageControl.vue  FileControl.vue      # 弹 MediaPicker 选 id
```

**注册表机制**（对齐设计文档"新增字段类型只加组件注册表"）：
```ts
// registry.ts
type FieldControl = { component: Component; validate: (v: any, f: SchemaField) => string | null }
const registry: Record<FieldType, FieldControl> = { ... }
```
- 每种控件负责渲染 + 本地校验（必填/长度/正则/数字范围/日期格式，镜像后端 schema 校验规则）
- 提交前整体校验，错误字段下内联提示（"前后端双校验：前端即时提示，后端权威"）
- **Translatable 字段**：多语言 Tab 下非 translatable 字段跨语言共享同一值；translatable 字段每语言独立编辑
- **richtext**：`RichTextControl` 用 wangEditor 5，输出 HTML 字符串
- **image/file**：控件内置"选择媒体"按钮 → `MediaPicker` 弹层 → 控件值存媒体 id（与模板 `media` 函数解析 URL 的契约一致）

### 与后端契约

- 表单回填：`GET /api/content/:id` 返回 Entry `{content:{..., payload}, type_name, fields}`——用 `fields` 回填表单值
- 保存：`PUT /api/content/:id` body `{type, lang, data}`，data 为字段值 map
- 新建：`POST /api/content` body `{type, lang, data}`（草稿）

## 5. 页面流程与交互

### 登录页
账号密码表单 → 登录 → 拉 me → 跳仪表盘。

### 仪表盘
`GET /api/stats` → 统计卡片（各内容类型 published/draft 数）+ 最近发布列表。

### 内容管理（核心）
- **列表页** `ContentListView.vue`：
  - 筛选：类型（下拉，来自 `GET /api/content-types`）、语言、状态（全部/草稿/已发布）
  - 搜索框：`GET /api/search?type=&lang=&q=`（防抖）
  - 分页表格：标题、slug、状态、语言、更新时间；操作：编辑、删除
  - 新建按钮 → 选类型 → 进编辑页
- **编辑页** `ContentEditView.vue`：
  - 左侧多语言 Tab（默认语言 + 其他语言）
  - 未翻译语言 Tab 首次切换 → `POST /api/content/:id/translate` 创建该语言草稿（透传父 content_id）
  - 已翻译语言 Tab → `PUT /api/content/:id` 更新
  - 发布/撤回 → `POST /api/content/:id/publish|unpublish`（author 无此按钮）
  - 翻译列表：`GET /api/content/:id/translations`

### 媒体库
上传（multipart）、列表（分页）、预览、删除；`MediaPicker` 弹层复用。

### 菜单管理
按语言列出菜单，编辑 items（导航项 label/url 可视化编辑）。

### 设置
站点名称/描述/主题/语言等键值编辑（`GET/PUT /api/settings`）。

## 6. 后端配套改动

### 新增 `/api/search`
- 路由：`GET /api/search`，权限 `content.read`
- 参数：`type`、`lang`、`q`、`page`、`per_page`
- 实现：
  - `internal/store`：`ContentRepo` 新增 `SearchByTypeLang(ctx, typeName, lang, q, offset, limit) ([]Content, error)` 与 `CountSearch(ctx, typeName, lang, q) (int, error)`（SQLite `LIKE '%q%'` 匹配 title/slug，**参数化查询**）
  - `internal/content`：`Service.Search(ctx, typeName, lang, q, page, perPage) ([]Entry, int, error)`（复用 entryFromStore）
  - `internal/adminapi`：`search.go` 新增 `HandleSearch`，注册到 `register.go`
  - 测试：sqlite 层 + content 层 + adminapi 层（含搜索命中/无结果/缺 q）

### 新增 `/api/stats`
- 路由：`GET /api/stats`，权限 `content.read`
- 响应：`{ by_type: [{type_name, published, draft}], recent: [entry...] }`
- 实现：
  - `internal/content`：`Service.Stats(ctx) (Stats, error)`——遍历 `AllTypes`，对每个类型用 `CountByTypeLangStatus`（published/draft 各计）；recent 用 `ListAdmin` 各类型默认语言取前 N（如每类型 5 条）
  - `internal/adminapi`：`stats.go` 新增 `HandleStats`，注册到 `register.go`
  - 测试：content 层 + adminapi 层

### 最小修正（仅当 SPA 受阻，实现时核对）
- `HandleContentList` 返回完整 Entry（含 content/slug/status/type_name/fields）——预期足够，不改
- `HandleContentGet` 返回 Entry（含 fields 供表单回填）——已确认足够
- `HandleTranslations` 返回 items 数组——核对后若与列表页翻译组需求不符则微调
- 其余一律不动，保持恢复基线的"缺口重建契约"不被破坏

## 7. 错误处理

- 前端：`client.ts` 统一归一后端 `{code, message}` 错误，弹 ElMessage 提示
- 后端错误码映射（现有契约）：404 资源不存在、403 无权限、422 校验失败、401 未登录
- 前后端双校验：前端即时提示 + 后端权威（表单提交失败显示后端 message）

## 8. 测试策略

- **后端**（Go）：新增 search/stats 的 store/content/adminapi 单测，遵循现有 `-count=1` 风格；`go test -count=1 ./...` 全绿
- **前端**（Vue）：核心逻辑（registry 校验、client 401 拦截、翻译组状态）用 Vitest 单测；页面级用 Vue Test Utils 冒烟（渲染不报错）
- **验收冒烟**：`go run ./cmd/dulizhan seed` + 启动，浏览器访问 `/admin` 完成 登录→建类型内容→发布→前台渲染 全流程；`/admin` 静态资源可加载

## 9. 工作流约定

- 按仓库约定：**不 git 提交**，直接 master 开发，文件级审查
- superpowers SDD（subagent-driven-development）驱动：先 writing-plans 产出实现计划，任务走 TDD，审查意见记入 `progress.md`
- 前端新增 `npm` 工作流（`admin/` 内独立），Go 侧 `go:embed` 依赖构建产物；验证命令含 `npm run build` 与 `go test ./...`
