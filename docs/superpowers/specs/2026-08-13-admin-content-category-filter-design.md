# 内容管理分类展示/筛选 与 分类内容数口径统一 设计文档

- 日期：2026-08-13
- 状态：已确认（用户逐节审阅通过）
- 关联：延续 `2026-08-13-home-category-sections-design.md`（前台首页分类分区，本次为后台管理端）

## 1. 目标

修复后台两个问题：
1. **分类管理"内容数"与内容管理实际数量不一致**——根因：`CountContent` 统计该分类下全部语言（zh+en）、全部状态的内容行；内容管理默认按当前语言过滤显示，导致口径不一致（如"产品"分类显示 4，内容管理 zh 显示 2）。
2. **内容管理增强**——列表展示每条内容所属分类；支持按分类查看对应内容（与关键词搜索可组合）。

## 2. 设计决策（头脑风暴确认）

| 决策点 | 结论 |
|---|---|
| 分类内容数口径 | 按当前语言计数（与内容管理默认视图一致） |
| 分类管理语言来源 | 页面顶部加语言选择器（默认站点默认语言），切换重载计数 |
| 内容管理分类功能 | 表格加"分类"列（显示分类名）+ 筛选区加"分类"下拉 |
| 分类筛选状态范围 | 包含草稿（与内容管理"全部状态"默认视图一致） |
| 搜索与分类组合 | 可同时生效（分类筛选在搜索结果内过滤） |

## 3. 后端改动

### 3.1 store 接口（`internal/store/store.go`）

```go
// CategoryRepo 增加：
CountContentByLang(ctx context.Context, id int64, lang string) (int, error)

// ContentRepo 增加：
ListByTypeLangStatusCategory(ctx, typeName, lang, status string, categoryID int64, offset, limit int) ([]Content, error)
CountByTypeLangStatusCategory(ctx, typeName, lang, status string, categoryID int64) (int, error)
```

### 3.2 sqlite 实现（`internal/store/sqlite/sqlite.go`）

- `CountContentByLang`：在现有 `CountContent` 的 `payload LIKE '%"category":"<id>"%'` 基础上追加 `AND lang=?`。
- `ListByTypeLangStatusCategory` / `CountByTypeLangStatusCategory`：在现有 `ListByTypeLangStatus` / `CountByTypeLangStatus` 基础上追加 `AND c.payload LIKE ?`（category 条件），**不强制 published**（含草稿，status 参数可为空=全部）。

### 3.3 admin API

**`internal/adminapi/categories.go` `HandleCategories`**：
- 读 `?lang=` query；为空用站点默认语言（通过 `Deps` 拿配置或 meta）。
- 计数改用 `CountContentByLang(ctx, id, lang)`。

**`internal/adminapi/content.go` `HandleContentList`**：
- 新增 `category` query 参数（可选，分类 id）。
- 有 category 时用 `ListByTypeLangStatusCategory` / `CountByTypeLangStatusCategory`；否则维持现有 `ListAdmin`。
- 返回每条 entry 附带 `category_name`：解析 `entry.Fields["category"]`（分类 id 字符串）→ `CategoryRepo.GetByID` → 名称；无则留空。
- 新增辅助方法 `categoryName(ctx, entry) string` 复用。

## 4. 前端改动

### 4.1 `admin/src/api/content.ts`
- `ContentEntry` 增加 `category_name?: string`。
- `listContent` 支持 `category` 参数。

### 4.2 `admin/src/api/category.ts`
- `listCategories` 支持 `lang` 参数（`/categories?lang=xx`）。

### 4.3 `admin/src/views/ContentListView.vue`
- 筛选区加"分类"下拉：加载全部分类（含子分类，平铺列表或树），多选（含子分类语义）；与类型/语言/状态/关键词组合。
- 表格加"分类"列，显示 `row.category_name`。
- 分类筛选与关键词搜索同时生效（搜索请求带 category）。

### 4.4 `admin/src/views/CategoriesView.vue`
- 顶部加语言选择器（选项来自 meta.languages，默认 default_lang）。
- 切换语言时重载 `listCategories(lang)`，计数按所选语言显示。

## 5. 数据流与兼容

- 未传 lang / 未选 category → 行为与现状一致（向后兼容）。
- `Fields["category"]` 缺失或分类已删除 → `category_name` 为空，归入"未分类"。
- 分类计数含草稿+已发布，按当前语言（与内容管理默认视图一致）。
- 现有 `CountContent` 保留（删除分类守卫仍用全量计数）；`ListByCategories`/`CountByCategories`（前台 published 聚合）不动。

## 6. 测试

- **store 层**：`CountContentByLang`（按语言过滤）、`ListByTypeLangStatusCategory`/`CountByTypeLangStatusCategory`（含草稿、跨语言、category id 匹配、offset/limit）。
- **adminapi 层**：`HandleCategories` 带 lang 计数；`HandleContentList` 带 category 筛选 + category_name 返回。
- **前端**：ContentListView 分类列/筛选逻辑、CategoriesView 语言选择器（vitest）。

## 7. 验证

- `go test -count=1 ./...` 全绿、`go vet ./...`、`gofmt -l .` 干净。
- 前端 `npx vue-tsc --noEmit && npx vitest run`；改 SPA 后 `npx vite build` + 同步 `internal/server/dist/`。
- 冒烟：分类管理选语言后计数与内容管理（同语言）一致；内容管理选分类看到对应内容（含草稿）；搜索+分类组合正确。
