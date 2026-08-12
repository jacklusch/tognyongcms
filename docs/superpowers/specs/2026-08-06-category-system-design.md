# Dulizhan CMS — 独立分类系统 设计

- 日期：2026-08-06
- 状态：已确认（用户逐节审阅通过）
- 前置文档：`docs/superpowers/specs/2026-08-05-cms-design.md`、`2026-08-06-phase4-content-type-builder-design.md`
- 基线：阶段 5 + 前台导航菜单接线已完成；article 现有 `category` select 字段（options: 新闻/产品/关于）硬编码在 schema

## 1. 目标与范围

把分类从"select 字段 options 硬编码"升级为**独立可管理的分类系统**：分类存独立表，后台增删改查，内容通过 relation 字段关联分类，前台有 `/category/<slug>` 归档页。

**包含**：
- categories 表 + CategoryRepo
- relation 字段 `RelationType:"category"` 哨兵关联分类（存分类 id）
- 管理 API（分类 CRUD + 内容数 + 有引用拒删）
- 前台 `/category/<slug>` 归档页（分页）
- 前端：分类管理页、CategoryPicker 选择器、RelationControl 按哨兵分流
- article 的 category select 字段 → 换 relation + 迁移
- 默认主题：single 显示分类、归档页

**排除**：多分类（一内容一分类）；分类层级/父子；分类排序自定义；分类独立 SEO 字段（YAGNI）。

## 2. 数据模型与后端

### 2.1 categories 表（`internal/store/sqlite/migrate.go` 追加）

```sql
CREATE TABLE IF NOT EXISTS categories (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  slug TEXT NOT NULL UNIQUE,
  description TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
```

### 2.2 store 模型与接口（`internal/store/model.go` + `store.go`）

```go
type Category struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type CategoryRepo interface {
	List(ctx) ([]Category, error)
	GetByID(ctx, id) (Category, error)
	GetBySlug(ctx, slug) (Category, error)
	Create(ctx, c *Category) error
	Update(ctx, c *Category) error
	Delete(ctx, id) error
	CountContent(ctx, id) (int, error) // 该分类下内容数（拒删用）
}
```
`Store` 接口加 `CategoryRepo() CategoryRepo`；sqlite 实现。

### 2.3 relation 字段哨兵（`internal/schema`）

- `RelationType:"category"` 表示关联分类（而非内容类型名）
- `schema.ValidateContentType`：relation 字段的 `RelationType` 校验——`"category"` 或 Registry 中存在的内容类型名，两者之一
- 值语义：category 关联存**分类 id**（数字转字符串）；内容关联存 content_id

### 2.4 管理 API（`internal/adminapi/categories.go`）

```
GET    /categories          content_types.manage   列表 {items:[{...category, content_count}]}
POST   /categories          content_types.manage   建分类 {name, slug, description}
PUT    /categories/:id      content_types.manage   改分类 {name, slug, description}
DELETE /categories/:id      content_types.manage   删分类（CountContent>0 拒删 403）
```
- slug 校验 `^[a-z0-9-]+$`（与内容 slug 一致）；slug 唯一
- 拒删：`CountContent(id) > 0` → 403「该分类下仍有内容，请先移除」

### 2.5 前台归档查询（`internal/content` + store）

`Service.ListPublishedByCategory(ctx, typeName, lang, categoryID, page, perPage)`：
- store `ContentRepo.ListByCategory(ctx, typeName, lang, categoryID, offset, limit)` + `CountByCategory`
- SQL：`WHERE t.name=? AND c.lang=? AND c.status='published' AND c.payload LIKE '%"category":"<id>"%'`（参数化 `?`，payload JSON 匹配）

## 3. 前台路由（`internal/server/frontend.go`）

- 2 段路由 `/{lang}/category/<slug>`：首段为 `category` 时 → `renderCategory`
- `renderCategory`：`CategoryRepo.GetBySlug(slug)` → 查该分类下已发布内容（跨类型？——**实现时**：默认当前类型（列表页语义），或遍历类型。**裁定**：`/category/<slug>` 聚合该分类下全部已发布内容（不限类型），按 `published_at` 排序分页）
- 主题：`list` 模板渲染（`data.TypeName` 设 `category`、`data.Page/Total` 分页）

## 4. 前端

### 4.1 分类管理页 `CategoriesView.vue`（`/categories`，adminOnly 菜单）

- 列表：name、slug、description、content_count；操作：编辑、删除（有引用拒删提示）
- 新建/编辑弹层：name、slug（自动从 name 生成 slug，可改）、description
- 数据源 `GET /categories`

### 4.2 `CategoryPicker.vue`（`admin/src/components/CategoryPicker.vue`）

- 弹层列出分类（name/slug）→ 单选 → emit 分类 id（字符串）
- 复用 MediaPicker 模式（`v-model:visible` + 列表 + 确定）

### 4.3 `RelationControl.vue` 分流

- `field.relation_type === 'category'` → 用 `CategoryPicker`（值=分类 id，回显分类名）
- 否则 → `ContentPicker`（值=content_id）——现有逻辑

### 4.4 路由/侧边栏

- router 加 `/categories`（adminOnly）；LayoutSidebar 加"分类"菜单（adminOnly）

## 5. 迁移 + 默认主题

### 5.1 article 字段迁移

- 移除 article 的 `category` select 字段
- 新增 `category` relation 字段：`{name:"category", label:"分类", type:"relation", relation_type:"category"}`
- seed 同步：article 定义含该 relation 字段（`internal/seed/seed.go`）
- 旧数据：已发布内容若有 select 分类值，提供 SQL 映射到 categories 对应记录（当前空库无此风险，文档说明）

### 5.2 默认主题

- `single.html`：显示所属分类名（`{{.Entry.Fields.category}}` 是分类 id 字符串；模板无分类 id→名称映射，**实现时**：后端 `entryFromStore` 或渲染时注入分类名到 `Data` 的辅助字段，或前端在 single 模板直接输出 id——**裁定**：后端在渲染 single 时把分类 id 解析为分类名，注入 `theme.Data` 新字段 `EntryCategory string`，single.html `{{if .EntryCategory}}分类：{{.EntryCategory}}{{end}}`）
- 归档页 `/category/<slug>` 用 `list` 模板渲染分类下文章（导航可指向分类页）

> **裁定**：single.html 显示分类名（不链接，避免模板解析 id→slug）；分类页入口由导航/列表提供。后端渲染 single 时解析分类 id→名称注入 `Data.EntryCategory`。

## 6. 测试策略

- 后端：CategoryRepo CRUD/CountContent 单测；categories API CRUD/拒删单测；ListPublishedByCategory 归档查询单测（含 payload LIKE 匹配）
- 前端：CategoriesView 组件逻辑、CategoryPicker vitest
- 冒烟：建分类（新闻/产品/关于）→ article 换 relation 字段 → 发文章选分类 → 前台 `/category/news` 列出该分类文章 → 管理端分类增删改查 + 有内容拒删

## 7. 验收

1. `go test -count=1 ./...` 全绿 + build/vet/gofmt 干净
2. `cd admin && npx vue-tsc --noEmit && npx vitest run && npx vite build` 通过
3. 端到端冒烟：分类增删改查 + 内容选分类 + 前台归档页渲染

## 8. 工作流约定

- 不 git 提交；superpowers SDD 驱动；前端构建同步 `internal/server/dist`
