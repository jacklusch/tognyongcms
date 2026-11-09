# 分类英文名（name_en）本地化 设计文档

- 日期：2026-08-20
- 状态：已确认（用户审阅通过）
- 背景：用户要求英文站点（/en/category/...）分类页标题及分类展示名为英文。现有分类只有中文 `name`。

## 1. 目标

给分类增加英文名 `name_en`，英文模式下分类页 title、分类页 h1、子分类导航、详情页分类名、面包屑均显示英文；中文模式或英文名未填时回退中文名。

## 2. 设计决策（头脑风暴确认）

| 决策点 | 结论 |
|---|---|
| 数据模型 | `categories` 表新增 `name_en` 列（`TEXT NOT NULL DEFAULT ''`） |
| 回退规则 | `lang=="zh"` → `Name`；否则 `NameEn` 非空 → `NameEn`，空 → `Name` |
| 现有数据 | 迁移加列（PRAGMA 探测，兼容已有库）；`dulizhan seed` 幂等回填演示分类英文名 |
| 后台 | 分类管理新增「英文名称」输入框（可空） |
| 菜单 | 不涉及（菜单已是每语言独立实体） |
| 英文名 | 新闻→News、关于→About、产品→Products、斩拌机→Chopper、香肠机→Sausage Machine、拌馅机→Mixer |

## 3. 数据库迁移（`internal/store/sqlite/migrate.go`）

新增 `migrateCategoriesNameEn`，复用 `migrateCategoriesParent` 的 PRAGMA 探测模式：

```go
func migrateCategoriesNameEn(db *sql.DB) error {
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('categories') WHERE name = 'name_en'").Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err := db.Exec("ALTER TABLE categories ADD COLUMN name_en TEXT NOT NULL DEFAULT '';")
	return err
}
```

在 `migrate.go` 的迁移调用链中追加（参照现有 `migrateCategoriesParent` 的注册位置）。

## 4. store 层（`internal/store/model.go` + `sqlite.go`）

- `store.Category` 增加 `NameEn string`（`json:"name_en"`）。
- `sqlite.go`：
  - `scanCategory` 增加 name_en 扫描。
  - `List` / `GetByID` / `GetBySlug` / `ListChildren` / `Descendants` 的 SELECT 列清单加 `name_en`。
  - `Create` / `Update` 的 SQL 加 name_en 列与参数。

## 5. admin API（`internal/adminapi/categories.go`）

- `categoryReq` 增加 `NameEn string json:"name_en"`。
- `HandleCategoryCreate` / `HandleCategoryUpdate` 写入 `cat.NameEn`。
- `buildCategoryTree`：node 增加 nameEn，响应 gin.H 增加 `"name_en"`。
- `validateCategory`：name_en 可空，不校验。

## 6. admin UI（`admin/src/views/CategoriesView.vue` + `api/category.ts`）

- `CategoryItem` 增加 `name_en: string`；`createCategory`/`updateCategory` body 增加 `name_en`。
- 表单加「英文名称」`el-input`（`v-model="form.name_en"`），打开编辑时回填。

## 7. seed（`internal/seed/seed.go`）

- `demoCategories` 与 `subCategories` 结构加 `NameEn` 字段并赋值。
- 已存在分类分支：若 `existing.NameEn == ""` 则 `Update` 回填英文名（幂等，覆盖已有库）。

## 8. 前台本地化（`internal/server/frontend.go` + theme）

新增 server 方法（放 `frontend.go`）：

```go
// catName 按语言取分类显示名：zh 用 Name；否则 NameEn 非空用 NameEn，空回退 Name。
func (s *Server) catName(cat store.Category, lang string) string {
	if lang == "zh" {
		return cat.Name
	}
	if cat.NameEn != "" {
		return cat.NameEn
	}
	return cat.Name
}
```

应用点（全部在 server 层本地化后传给 theme/seo）：
- `renderCategory`：`BuildCategory(lang, pathSegs, s.catName(cat, lang), all)`；`data.EntryCategory = s.catName(cat, lang)`；`SubCategories` 的 `Name` 用 `catName(ch, lang)`。
- `renderSingle`：`data.EntryCategory = s.catName(cat, lang)`；`entryBreadcrumbs` 中分类名用 `catName(chain[i], lang)`。
- `list.html`/`single.html` 无需改（用的就是 `.EntryCategory`/`.SubCategories`）。

## 9. 数据回填

实施完成、`build.ps1` 重建后运行 `.\dulizhan.exe seed`（幂等），为现有分类回填 name_en。

## 10. 测试

- `internal/store/sqlite/sqlite_test.go`：分类 Create/Update/Get 带 name_en 往返。
- `internal/adminapi/categories_test.go`：create/update 带 name_en；列表返回 name_en。
- `internal/server/frontend_test.go`：
  - `buildTestServer` 后 `GET /en/category/products`，断言 `<title>Products - Test Site</title>`（或按 seed 回填后的英文名）。
  - `GET /category/products`（zh）断言 title 用中文名。
  - 子分类导航 en 模式显示英文子分类名。
- 回退用例：`name_en` 为空时 en 模式回退中文名。

## 11. 验证

- `go test -count=1 ./...`、`go vet ./...`、`gofmt -l .`
- 前端 `npx vue-tsc --noEmit && npx vitest run`
- `.\build.ps1` 重建 + `.\dulizhan.exe seed` 回填 + 重启
- 浏览器抽查 `/en/category/products`、`/en/category/products/chopper` 标题为英文，中文站仍中文。
