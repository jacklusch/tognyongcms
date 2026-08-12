# 分类子分类功能 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 为分类系统增加任意层级子分类（parent 自引用树）：后台 CRUD（树形表格 + 行内添加子分类 + 上级分类级联选择），前台两级路径 URL `/category/products/chopper` + 父分类递归聚合子孙内容 + 子分类导航。

**架构：** `categories` 表加 `parent_id` 列（0=顶级），`CategoryRepo` 扩展 `ListChildren`/`Descendants`；adminapi 返回树形 + 扁平 `all`（带 path）；前台 `renderCategory` 支持任意段路径，用 `Descendants` 递归聚合归档；管理端 CategoriesView 改树形表格，CategoryPicker 改级联；seed 补子分类演示数据。

**技术栈：** Go 1.26、Gin、SQLite（递归 CTE `WITH RECURSIVE`）、Vue 3 + Element Plus（el-table 树形 / el-cascader）、Vitest。

**规格：** `docs/superpowers/specs/2026-08-12-subcategories-design.md`

**仓库约定（重要）：**
- **不 git 提交**（用户约定：直接 master 开发，文件级审查）。计划中"Commit"步骤替换为"更新账本"。
- 验证命令（每任务必跑）：
  - Go：`go test -count=1 ./...`、`go build ./...`、`go vet ./...`、`gofmt -l .`
  - 前端：`cd admin && npx vue-tsc --noEmit && npx vitest run`；改 SPA 后 `npx vite build` + 同步 `internal/server/dist/`
- **勿运行 `go mod tidy`**。新依赖用 `go get <pkg>@<精确版本>`。
- 冒烟前确认 8080 无残留 dulizhan 进程。
- 错误消息/文案用中文；日志用 slog。
- store 模型 JSON tag 小写 snake_case。
- 领域错误：`errs.ErrNotFound/ErrForbidden/ErrValidation`，HTTP 层映射 404/403/422。

---

### 任务 1：sqlite store 层 —— categories 表加 parent_id + ListChildren/Descendants + IN 查询

**文件：**
- 修改：`internal/store/sqlite/migrate.go`（schema 追加 ALTER 守卫）
- 修改：`internal/store/model.go`（Category 加 ParentID）
- 修改：`internal/store/store.go`（CategoryRepo 接口 + contentRepo 接口新增方法）
- 修改：`internal/store/sqlite/sqlite.go`（categoryRepo 读写 parent_id、contentRepo ListByCategories/CountByCategories）
- 修改：`internal/store/sqlite/sqlite_test.go`
- 修改：`internal/content/service.go`（ListPublishedByCategory 改 ids 切片）

- [ ] **步骤 1：编写失败的测试**

在 `internal/store/sqlite/sqlite_test.go` 追加：

```go
func TestCategoryParentChildren(t *testing.T) {
	// 建顶级 A、B，A 下子 A1，A1 下孙 A1a
	// ListChildren(A) 返回 [A1]；ListChildren(A1) 返回 [A1a]；ListChildren(B) 返回 []
	// Descendants(A) 返回 [A1, A1a]
	// Descendants(A1) 返回 [A1a]
	// 无 parent 的分类读回 ParentID==0
}

func TestContentListByCategories(t *testing.T) {
	// 两篇文章分别归属 cat 5 和 cat 8（payload category 字段）
	// ListByCategories(ctx, "article", "zh", []int64{5,8}, 0, 10) 返回 2 篇
	// CountByCategories 返回 2
	// ListByCategories(ctx, ..., []int64{}, ...) 返回空
}
```

在 `internal/content/service_test.go` 追加 `TestListPublishedByCategories`（多 id、空 id、错误类型）：
```go
// svc.ListPublishedByCategories(ctx, "article", "zh", []int64{5,8}, 1, 10)
// svc.ListPublishedByCategories(ctx, "nope", "zh", []int64{5}, 1, 10) → errs.ErrNotFound
```

在 `internal/store/sqlite/migrate_test.go` 或 sqlite_test 加：重复 `Open`（重新执行 schema）不报错，`parent_id` 列存在。

- [ ] **步骤 2：运行测试验证失败**

运行：`go test -count=1 ./internal/store/sqlite/ ./internal/content/`
预期：编译失败（ParentID/ListChildren/Descendants/ListByCategories 未定义）

- [ ] **步骤 3：实现 schema + 模型 + store**

`internal/store/sqlite/migrate.go` schema 追加：
```go
ALTER TABLE categories ADD COLUMN parent_id INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_categories_parent ON categories (parent_id);
```
（注意：`CREATE TABLE IF NOT EXISTS` 不改存量表，故必须追加 ALTER；新库首次创建时 categories 表无 parent_id，ALTER 会补列，OK。）

`internal/store/model.go` Category 加：
```go
ParentID int64 `json:"parent_id"`
```

`internal/store/store.go` CategoryRepo 接口新增：
```go
ListChildren(ctx context.Context, parentID int64) ([]Category, error)
Descendants(ctx context.Context, id int64) ([]Category, error)
```
contentRepo 接口：
```go
ListByCategories(ctx context.Context, typeName, lang string, ids []int64, offset, limit int) ([]Content, error)
CountByCategories(ctx context.Context, typeName, lang string, ids []int64) (int, error)
```
删除 `ListByCategory`/`CountByCategory`。

`internal/store/sqlite/sqlite.go`：
- categoryRepo 所有 SELECT 加 parent_id，scanCategory 加 &c.ParentID
- Create/Update 写 parent_id
- ListChildren：`SELECT ... FROM categories WHERE parent_id = ? ORDER BY id`
- Descendants：递归 CTE
```go
func (r *categoryRepo) Descendants(ctx context.Context, id int64) ([]store.Category, error) {
	rows, err := r.db.QueryContext(ctx, `
		WITH RECURSIVE descs(id) AS (
			SELECT id FROM categories WHERE parent_id = ?
			UNION ALL
			SELECT c.id FROM categories c JOIN descs d ON c.parent_id = d.id
		)
		SELECT c.id, c.parent_id, c.name, c.slug, c.description, c.created_at
		FROM categories c JOIN descs d ON c.id = d.id ORDER BY c.id`, id)
	// scan 同上
}
```
- contentRepo：`ListByCategories`/`CountByCategories` 用 IN 查询（ids 为空直接返回空）：
```go
func placeholders(n int) string { return strings.TrimSuffix(strings.Repeat("?,", n), ",") }
// WHERE t.name=? AND c.lang=? AND c.status='published' AND c.payload LIKE '%"category":"ID"%' OR c.payload LIKE ... 
```
实现：对每个 id 生成一个 `c.payload LIKE ?` 条件（`"%\"category\":\"<id>\"%"`），用 OR 连接，避免构造复杂 IN。空 ids 返回空切片 / 0。
- 删除旧 `ListByCategory`/`CountByCategory`。

`internal/content/service.go`：`ListPublishedByCategory` → `ListPublishedByCategories(ctx, typeName, lang string, ids []int64, page, perPage int)`，内部调 `ListByCategories`/`CountByCategories`。删除旧方法。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test -count=1 ./internal/store/sqlite/ ./internal/content/`
预期：PASS

- [ ] **步骤 5：同步调用点 + 全量验证**

`internal/server/frontend.go:114` 现在调 `s.content.ListPublishedByCategory(..., cat.ID, 1, 1000)` —— 本任务暂改为 `ListPublishedByCategories(..., []int64{cat.ID}, 1, 1000)` 保证编译（任务 3 才改聚合逻辑）。`internal/seed` 若有调用同步。

运行：`go test -count=1 ./...`、`go build ./...`、`go vet ./...`、`gofmt -l .`
预期：全绿、干净

- [ ] **步骤 6：更新账本**

追加到 `.superpowers/sdd/2026-08-12-subcategories/progress.md`：Task 1 完成说明 + 验证输出摘要。

---

### 任务 2：adminapi —— 树形返回 + parent_id CRUD + 防环

**文件：**
- 修改：`internal/adminapi/categories.go`
- 修改：`internal/adminapi/categories_test.go`
- 修改：`internal/adminapi/register.go`（无需改路由，仅确认）

- [ ] **步骤 1：编写失败的测试**

`internal/adminapi/categories_test.go` 追加：
```go
// 场景：建父分类（parent_id=0）→ 建子分类（parent_id=父）→ 建孙分类（parent_id=子）
// GET /categories 返回树：items[父].children[0].children[0] 存在；all 含 {id:父, path:"产品"},{id:子, path:"产品/斩拌机"}
// 防环：PUT 父分类 parent_id=子分类 id → 422
//       PUT 父分类 parent_id=自身 id → 422
// 删除有子分类：DELETE 父 → 403
// 删除无子分类有内容 → 403（现状）
// 创建不填 slug → 自动生成（"斩拌机"→"zhanbanji" 或按 slugify 规则）；撞车 → 后缀
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test -count=1 ./internal/adminapi/`
预期：FAIL（无 parent_id 支持、树结构、防环）

- [ ] **步骤 3：实现**

`internal/adminapi/categories.go`：
- `categoryReq` 加 `ParentID int64 \`json:"parent_id"\``
- `slugify(name)` 纯函数（小写、非 `[a-z0-9]` 替换为 `-`、去首尾 `-`；空则返回空）
- `buildCategoryTree(cats []store.Category) (roots []gin.H, all []gin.H)`：单次遍历组 map[id]→node，node 含 id/parent_id/name/slug/description/content_count/children；`all` 为扁平 `{id, path}`，path 用父链拼接（`父path/名`）。content_count 由调用方传入（每个 id 的 CountContent）。
- `validateParent(ctx, id, parentID)`：parentID==0 合法；parent 必须存在（GetByID）；`id!=0 && parentID==id` → 防环；`parentID` 在 `Descendants(id)` 中 → 防环。返回 error（`errs.ErrValidation`）。
- `HandleCategories`：List 全部 → 每节点 CountContent → `buildCategoryTree` → `respondOK(gin.H{"items": roots, "all": all})`
- `HandleCategoryCreate`：validateCategory + validateParent(0, parentID)；slug 空则 slugify(name)，撞车加 `-2/-3`（循环 GetBySlug 找空闲）
- `HandleCategoryUpdate`：validateCategory + validateParent(id, parentID)
- `HandleCategoryDelete`：`ListChildren(id)` 非空 → `ErrForbidden`"该分类下仍有子分类，请先删除子分类"；`CountContent>0` → 403（现状保留）

- [ ] **步骤 4：运行测试验证通过**

运行：`go test -count=1 ./internal/adminapi/`
预期：PASS

- [ ] **步骤 5：全量验证**

运行：`go test -count=1 ./...`、`go build ./...`、`go vet ./...`、`gofmt -l .`
预期：全绿、干净

- [ ] **步骤 6：更新账本**

---

### 任务 3：前台 —— 任意段路径 + 递归聚合 + 子分类导航 + EntryCategoryURL

**文件：**
- 修改：`internal/server/frontend.go`
- 修改：`internal/theme/theme.go`（Data 加 SubCategories/EntryCategoryURL）
- 修改：`themes/default/templates/list.html`
- 修改：`themes/default/templates/single.html`
- 修改：`internal/server/frontend_test.go`

- [ ] **步骤 1：编写失败的测试**

`internal/server/frontend_test.go` 追加：
```go
// 用 buildTestServer 构造：seed 后建 产品(products)>斩拌机(chopper) 子分类，
// 一篇 article 归 chopper（payload category=chopper id）
// GET /category/products/chopper → 200，页面含 chopper 文章标题
// GET /category/products → 200，聚合显示 chopper 文章（子孙聚合）
// GET /category/products/chopper/x/y → 404（超深路径）
// GET /category/nope → 404（未知顶级）
// 详情页：GET /article/<chopper 文章 slug> → 含 EntryCategory 与 /category/products/chopper 链接
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test -count=1 ./internal/server/ -run TestFrontendSub`
预期：FAIL

- [ ] **步骤 3：实现**

`internal/server/frontend.go`：
- 路由分支改：
```go
case len(segs) >= 2 && segs[0] == "category":
	s.renderCategory(c, th, lang, segs[1:], data)
```
（注意：`len(segs)==2` 的旧 category 分支删除；`len(segs)==2` 非 category 仍是 renderSingle。）
- `renderCategory(c, th, lang, pathSegs []string, data)`：
  - 解析：首段 `GetBySlug`，后续段在 `ListChildren(prevID)` 中按 slug 匹配；任一级 ErrNotFound → 404
  - `Descendants(cat.ID)` + 自身 → `ids []int64`
  - 遍历类型：`s.content.ListPublishedByCategories(ctx, ct.Name, lang, ids, 1, 1000)` 聚合
  - `data.SubCategories = ListChildren(cat.ID)` 的直接子分类（转 theme 需要的形式，见下）
  - `data.TypeName = "category"`（list.html 用 `ne $.TypeName "category"` 隐藏分页）
  - Meta 用 `cat.Name`
- `renderSingle`：分类名解析后，沿父链构造 `EntryCategoryURL`（`/category/父slug/.../子slug`），设 `data.EntryCategoryURL`。父链获取：从分类节点向上（CategoryRepo 需能逐级 GetByID；维护一个 map 或循环 GetByID）。

`internal/theme/theme.go` Data 加：
```go
SubCategories    []CategoryInfo  // 直接子分类
EntryCategoryURL string
```
新增类型：
```go
type CategoryInfo struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	URL  string `json:"url"`
}
```

`themes/default/templates/list.html`：在 `<ul class="entry-list">` 前插入子分类导航：
```html
{{if .SubCategories}}
<div class="sub-categories">
  {{range .SubCategories}}<a class="sub-cat" href="{{.URL}}">{{.Name}}</a>{{end}}
</div>
{{end}}
```

`themes/default/templates/single.html`：
```html
{{if .EntryCategory}}
<div class="entry-category">分类：
  {{if .EntryCategoryURL}}<a href="{{.EntryCategoryURL}}">{{.EntryCategory}}</a>{{else}}{{.EntryCategory}}{{end}}
</div>
{{end}}
```

`themes/default/static/css/main.css`：加 `.sub-categories`/`.sub-cat` 样式。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test -count=1 ./internal/server/ ./internal/theme/ ./themes/default/`
预期：PASS

- [ ] **步骤 5：全量验证**

运行：`go test -count=1 ./...`、`go build ./...`、`go vet ./...`、`gofmt -l .`
预期：全绿、干净

- [ ] **步骤 6：更新账本**

---

### 任务 4：seed —— 子分类演示数据 + 菜单

**文件：**
- 修改：`internal/seed/seed.go`
- 修改：`internal/seed/seed_test.go`

- [ ] **步骤 1：编写失败的测试**

`internal/seed/seed_test.go` `TestSeedDemoData` 追加断言：
```go
// 产品下子分类存在：斩拌机(chopper)、香肠机(sausage-machine)、拌馅机(mixer)，ParentID == 产品id
// chopper 有 1 篇双语文章（zh 已发布 + en 同组翻译），category == chopper id
// main 菜单（zh/en）含子分类项（/category/products/chopper）
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test -count=1 ./internal/seed/`
预期：FAIL

- [ ] **步骤 3：实现**

`internal/seed/seed.go`：
- `seedDemoData` 建分类循环后，追加子分类创建（幂等：GetBySlug 已存在跳过）：
```go
subCategories := map[string][]struct{ Name, Slug string }{
	"products": {{"斩拌机", "chopper"}, {"香肠机", "sausage-machine"}, {"拌馅机", "mixer"}},
}
// 每个：ParentID = catIDs[父slug]
```
- demoContent 增加子分类文章：chopper 1 篇（`chopper-1`，category=chopper id）；sausage-machine 1 篇；mixer 1 篇（zh+en 双语，字段与既有 demoArticle 一致）
- `seedMainMenu`：`/category/products` 项下追加子分类项（`/category/products/chopper` 等），zh/en 各一套标签（斩拌机/Chopper 等）

- [ ] **步骤 4：运行测试验证通过**

运行：`go test -count=1 ./internal/seed/`
预期：PASS

- [ ] **步骤 5：全量验证**

运行：`go test -count=1 ./...`、`go build ./...`、`go vet ./...`、`gofmt -l .`
预期：全绿、干净

- [ ] **步骤 6：更新账本**

---

### 任务 5：管理端 Vue —— 树形表格 + 级联选择器

**文件：**
- 修改：`admin/src/api/category.ts`
- 修改：`admin/src/views/CategoriesView.vue`
- 修改：`admin/src/components/CategoryPicker.vue`
- 修改：`admin/src/views/__tests__/categories-view.test.ts`
- 修改：`admin/src/views/__tests__/category-picker.test.ts`
- 修改：`admin/src/dynamic-form/controls/RelationControl.vue`（如分类选择走 CategoryPicker）

- [ ] **步骤 1：编写失败的测试**

`admin/src/views/__tests__/categories-view.test.ts`：
```ts
// listCategories 返回树（含 children 嵌套）→ 表格渲染出子分类行
// 点击行内"添加子分类"→ 对话框出现，保存时提交 parent_id
// 删除有 children 的分类 → 前端提示"请先删除子分类"不调 API
```
`category-picker.test.ts`：
```ts
// 级联数据由 all（含 path）生成；选中叶子返回 id
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd admin && npx vitest run src/views/__tests__/categories-view.test.ts src/views/__tests__/category-picker.test.ts`
预期：FAIL

- [ ] **步骤 3：实现**

`admin/src/api/category.ts`：
```ts
export interface CategoryItem {
  id: number
  parent_id: number
  name: string
  slug: string
  description: string
  content_count: number
  children: CategoryItem[]
}
export interface CategoryPath { id: number; path: string }
export const listCategories = () => request<{ items: CategoryItem[]; all: CategoryPath[] }>('/categories')
export const createCategory = (body: { name: string; slug?: string; description?: string; parent_id?: number }) => ...
export const updateCategory = (id: number, body: { name: string; slug: string; description?: string; parent_id?: number }) => ...
```

`CategoriesView.vue`：
- `el-table` 加 `row-key="id"` + `:tree-props="{ children: 'children' }"`，`:data="items"`
- 操作列加"添加子分类"按钮：`openCreate(row)` 预填 `form.parent_id = row.id`
- 对话框表单加"上级分类"`el-cascader`（props: `{ value:'id', label:'path', emitPath:false, checkStrictly:true }`，options 由 `all` 组装为树或直接用扁平 options）；编辑时回显父分类；可清空（顶级）
- `form` 加 `parent_id`
- 删除：`row.children?.length` 时警告并 return（后端 403 双保险）
- `remove` 传 `parent_id` 无需变

`CategoryPicker.vue`：改为 `el-cascader`，options 用 `all` 生成（每项 `{ value: id, label: path }`），`checkStrictly: true`，`emitPath: false`，`selected = String(id)`。

- [ ] **步骤 4：运行测试验证通过**

运行：`cd admin && npx vue-tsc --noEmit && npx vitest run`
预期：PASS

- [ ] **步骤 5：构建 + 同步 dist**

运行：`cd admin && npx vite build`；确认 `internal/server/dist/` 同步更新（参考仓库现有构建脚本逻辑，可手动复制 `admin/dist/*` 到 `internal/server/dist/`）。

- [ ] **步骤 6：全量验证**

运行：`go test -count=1 ./...`、`go build ./...`、`go vet ./...`、`gofmt -l .`
预期：全绿、干净

- [ ] **步骤 7：更新账本**

---

### 任务 6：端到端验收 + 最终审查

**文件：**
- 修改：`.superpowers/sdd/2026-08-12-subcategories/progress.md`

- [ ] **步骤 1：冒烟验收**

运行：确认 8080 无残留进程 → `go run ./cmd/dulizhan seed` → 后台分类管理页 → 前台：
- `go run ./cmd/dulizhan --config config.yaml`
- 前台 `/category/products` 显示子分类导航（斩拌机/香肠机/拌馅机）+ 聚合全部产品内容
- `/category/products/chopper` 只显示斩拌机文章
- `/en/category/products/chopper` 英文对应
- 详情页分类 chip 链接到 `/category/products/chopper`
- 后台：树形表格显示层级、行内添加子分类、上级分类级联选择、删除有子分类被拒

- [ ] **步骤 2：派最终审查子代理**

调度一个 general 子代理读规格 + 本计划 + 实现代码，输出 `.superpowers/sdd/2026-08-12-subcategories/final-review.md`（结论：干净 | 需修复 + 严重度清单）。

- [ ] **步骤 3：处理审查意见 + 更新账本**

按严重度修复；`progress.md` 记录处理结果。
