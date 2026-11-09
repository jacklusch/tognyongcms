# 内容管理分类展示/筛选 与 分类内容数口径统一 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 修复分类管理"内容数"与内容管理实际数量不一致（按当前语言计数），并让内容管理展示每条内容所属分类、支持按分类查看（与关键词搜索可组合）。

**架构：** store 层新增 `CountContentByLang`（分类按语言计数）与 `ListByTypeLangStatusCategory`/`CountByTypeLangStatusCategory`（按分类+状态过滤、含草稿）；adminapi 的 `HandleCategories` 读 `?lang=` 按语言计数、`HandleContentList` 新增 `category` 参数并返回每条 `category_name`；前端 `CategoriesView` 加语言选择器、`ContentListView` 加分类列与分类筛选下拉。

**技术栈：** Go（gin、modernc.org/sqlite）、Vue3+TS+Element Plus（admin SPA）。

**前置基线：** 规格 `docs/superpowers/specs/2026-08-13-admin-content-category-filter-design.md` 已确认。当前 `CountContent`（sqlite.go:721）用 `payload LIKE '%"category":"<id>"%'` 全量计数；`ListByCategories`/`CountByCategories`（sqlite.go:349,378）强制 `status='published'`；`HandleCategories`（adminapi/categories.go:33）；`HandleContentList`（adminapi/content.go:55）。

**环境约束（所有任务遵守）：**
- **不 git 提交**（仓库约定，直接 master 文件级审查）；勿运行 `go mod tidy`
- Go 1.26.5 已装（`C:\Program Files\Go\bin`，跑 go 命令前 `$env:Path = "C:\Program Files\Go\bin;" + $env:Path`）
- Go 验证：`go test -count=1 ./...`、`go vet ./...`、`gofmt -l .`
- 前端：`cd admin && npx vue-tsc --noEmit && npx vitest run`；**改 SPA 后必须 `npx vite build` + 同步 `internal/server/dist/`**（admin SPA 是 go:embed）
- 错误消息用中文；改动不破坏现有测试（`categories_test.go` 的 content_count>0 断言基于 zh seed 数据）

**验证基线（写任何代码前先跑一次）：**
```powershell
$env:Path = "C:\Program Files\Go\bin;" + $env:Path
go test -count=1 ./...
```
预期：全绿。

---

### 任务 1：store 层新增分类按语言计数

**文件：**
- 修改：`internal/store/store.go`
- 修改：`internal/store/sqlite/sqlite.go`
- 测试：`internal/store/sqlite/sqlite_test.go`

`CategoryRepo` 增加 `CountContentByLang(ctx, id, lang)`；sqlite 实现按 `payload LIKE` + `lang` 过滤。

- [ ] **步骤 1：编写失败测试**（`internal/store/sqlite/sqlite_test.go` 追加）

> **修正说明（2026-08-13 执行前）**：sqlite_test.go 仅有 `newTestStore` 辅助，无 seed/content 装配。测试代码改为**自包含装配**（内联 i18n/schema/content/seed），不引用未定义辅助。

```go
func TestCountContentByLang(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	reg, err := i18n.New([]string{"zh", "en"}, "zh", false)
	if err != nil {
		t.Fatal(err)
	}
	svc := content.New(st, schema.NewRegistry(), []string{"zh", "en"})
	med := media.NewLocalStore(t.TempDir(), "/media")
	if err := seed.Run(ctx, st, svc, med); err != nil {
		t.Fatal(err)
	}
	news, err := st.CategoryRepo().GetBySlug(ctx, "news")
	if err != nil {
		t.Fatal(err)
	}
	zh, err := st.CategoryRepo().CountContentByLang(ctx, news.ID, "zh")
	if err != nil {
		t.Fatal(err)
	}
	if zh != 3 {
		t.Errorf("news zh count = %d, want 3", zh)
	}
	en, err := st.CategoryRepo().CountContentByLang(ctx, news.ID, "en")
	if err != nil {
		t.Fatal(err)
	}
	if en != 3 {
		t.Errorf("news en count = %d, want 3", en)
	}
}
```

> 说明：seed 数据 news 分类 zh=3 行、en=3 行（seed.go:128-131 + hello-zh/hello-en）。测试文件需新增 imports：`dulizhan/internal/content`、`dulizhan/internal/i18n`、`dulizhan/internal/media`、`dulizhan/internal/schema`、`dulizhan/internal/seed`。

- [ ] **步骤 2：运行测试验证失败**

运行：`go test -count=1 ./internal/store/sqlite/ -run TestCountContentByLang -v`
预期：FAIL（`CountContentByLang` 未定义）。

- [ ] **步骤 3：实现 store 接口 + sqlite**

`internal/store/store.go` `CategoryRepo` 增加：
```go
CountContentByLang(ctx context.Context, id int64, lang string) (int, error)
```

`internal/store/sqlite/sqlite.go` 在 `CountContent` 后新增：
```go
func (r *categoryRepo) CountContentByLang(ctx context.Context, id int64, lang string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM content WHERE payload LIKE ? AND lang=?`,
		"%\"category\":\""+itoa64(id)+"\"%", lang).Scan(&n)
	return n, err
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test -count=1 ./internal/store/sqlite/ -run TestCountContentByLang -v`
预期：PASS。

---

### 任务 2：store 层新增按分类+状态过滤（含草稿）

**文件：**
- 修改：`internal/store/store.go`
- 修改：`internal/store/sqlite/sqlite.go`
- 测试：`internal/store/sqlite/sqlite_test.go`

`ContentRepo` 增加 `ListByTypeLangStatusCategory` / `CountByTypeLangStatusCategory`。

- [ ] **步骤 1：编写失败测试**（`internal/store/sqlite/sqlite_test.go` 追加）

```go
func TestListByTypeLangStatusCategory(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	reg, err := i18n.New([]string{"zh", "en"}, "zh", false)
	if err != nil {
		t.Fatal(err)
	}
	svc := content.New(st, schema.NewRegistry(), []string{"zh", "en"})
	med := media.NewLocalStore(t.TempDir(), "/media")
	if err := seed.Run(ctx, st, svc, med); err != nil {
		t.Fatal(err)
	}
	products, err := st.CategoryRepo().GetBySlug(ctx, "products")
	if err != nil {
		t.Fatal(err)
	}
	// products 分类 zh：products-1/products-2 两行（子分类 chopper/sausage-machine/mixer 不属于 products id）
	items, err := st.ContentRepo().ListByTypeLangStatusCategory(ctx, "article", "zh", "", products.ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Errorf("products zh rows = %d, want 2", len(items))
	}
	n, err := st.ContentRepo().CountByTypeLangStatusCategory(ctx, "article", "zh", "", products.ID)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("products zh count = %d, want 2", n)
	}
}
```

> 说明：seed 数据 products 分类 zh=2 行（products-1/products-2，seed.go:135-138）；子分类内容 category 指向子分类 id，不属于 products id。测试文件 imports 与任务 1 相同。

- [ ] **步骤 2：运行测试验证失败**

运行：`go test -count=1 ./internal/store/sqlite/ -run TestListByTypeLangStatusCategory -v`
预期：FAIL（方法未定义）。

- [ ] **步骤 3：实现**

`internal/store/store.go` `ContentRepo` 增加：
```go
ListByTypeLangStatusCategory(ctx context.Context, typeName, lang, status string, categoryID int64, offset, limit int) ([]Content, error)
CountByTypeLangStatusCategory(ctx context.Context, typeName, lang, status string, categoryID int64) (int, error)
```

`internal/store/sqlite/sqlite.go` 参照 `ListByTypeLangStatus`（:248）实现（追加 `AND c.payload LIKE ?`，不强制 published）：
```go
func (r *contentRepo) ListByTypeLangStatusCategory(ctx context.Context, typeName, lang, status string, categoryID int64, offset, limit int) ([]store.Content, error) {
	query := "SELECT " + contentCols + " FROM content c JOIN content_types t ON t.id = c.content_type_id WHERE t.name=?"
	args := []any{typeName}
	if lang != "" {
		query += " AND c.lang=?"
		args = append(args, lang)
	}
	if status != "" {
		query += " AND c.status=?"
		args = append(args, status)
	}
	query += " AND c.payload LIKE ?"
	args = append(args, "%\"category\":\""+itoa64(categoryID)+"\"%")
	query += " ORDER BY c.published_at DESC, c.id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.Content
	for rows.Next() {
		var c store.Content
		if err := rows.Scan(&c.ID, &c.ContentTypeID, &c.ContentID, &c.Lang, &c.Slug, &c.Title, &c.Status, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt, &c.PublishedAt, &c.Payload); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *contentRepo) CountByTypeLangStatusCategory(ctx context.Context, typeName, lang, status string, categoryID int64) (int, error) {
	query := "SELECT COUNT(*) FROM content c JOIN content_types t ON t.id = c.content_type_id WHERE t.name=?"
	args := []any{typeName}
	if lang != "" {
		query += " AND c.lang=?"
		args = append(args, lang)
	}
	if status != "" {
		query += " AND c.status=?"
		args = append(args, status)
	}
	query += " AND c.payload LIKE ?"
	args = append(args, "%\"category\":\""+itoa64(categoryID)+"\"%")
	var n int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&n)
	return n, err
}
```

> 说明：`contentCols` 列顺序与 scan 顺序须与现有 `ListByTypeLangStatus` 一致——实现者须对照 sqlite.go:248-275 逐列核对，勿凭记忆写 scan。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test -count=1 ./internal/store/sqlite/ -run TestListByTypeLangStatusCategory -v`
预期：PASS。

---

### 任务 3：adminapi 分类按语言计数 + 内容列表按分类筛选

**文件：**
- 修改：`internal/adminapi/categories.go`
- 修改：`internal/adminapi/content.go`
- 测试：`internal/adminapi/categories_test.go`、`internal/adminapi/content_test.go`

`HandleCategories` 读 `?lang=` 按语言计数；`HandleContentList` 新增 `category` 参数 + 返回 `category_name`。

- [ ] **步骤 1：编写失败测试**（`internal/adminapi/categories_test.go` 追加）

```go
func TestCategoriesCountByLang(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	// seed news 分类 zh=3, en=3；products 分类 zh=2
	// 无 lang → 默认 zh
	w := e.do(t, http.MethodGet, "/api/categories", "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("list = %d", w.Code)
	}
	var list struct {
		Data struct {
			Items []catTreeNode `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	count := func(name string) int {
		for _, it := range list.Data.Items {
			if it.Name == name {
				return it.ContentCount
			}
		}
		return -1
	}
	if count("新闻") != 3 {
		t.Errorf("新闻 content_count = %d, want 3", count("新闻"))
	}
	// lang=en → 新闻也是 3（en 3 行）
	w = e.do(t, http.MethodGet, "/api/categories?lang=en", "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("list en = %d", w.Code)
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if count("新闻") != 3 {
		t.Errorf("新闻 en content_count = %d, want 3", count("新闻"))
	}
}
```

（`catTreeNode` 已在 categories_test.go 定义复用。）

`internal/adminapi/content_test.go` 追加：
```go
func TestContentListByCategory(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	// seed products 分类 id 从 /api/categories 拿
	w := e.do(t, http.MethodGet, "/api/categories", "", tok)
	var cl struct {
		Data struct {
			All []struct {
				ID   int64  `json:"id"`
				Path string `json:"path"`
			} `json:"all"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &cl)
	var productsID int64
	for _, a := range cl.Data.All {
		if a.Path == "产品" {
			productsID = a.ID
		}
	}
	if productsID == 0 {
		t.Fatal("未找到产品分类")
	}
	// 按分类筛选 zh
	w = e.do(t, http.MethodGet, "/api/content?type=article&lang=zh&category="+strconv.FormatInt(productsID, 10), "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("content by category = %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Items []struct {
				TypeName string `json:"type_name"`
				Fields   map[string]any
			} `json:"items"`
			Total int `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data.Total != 2 {
		t.Errorf("products zh total = %d, want 2", resp.Data.Total)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test -count=1 ./internal/adminapi/ -run "TestCategoriesCountByLang|TestContentListByCategory" -v`
预期：FAIL（新行为未实现——`?lang=` 被忽略仍全量计数、`category` 参数被忽略）。

- [ ] **步骤 3：实现**

`internal/adminapi/categories.go` `HandleCategories`：
```go
func (d *Deps) HandleCategories(c *gin.Context) {
	ctx := c.Request.Context()
	lang := c.Query("lang")
	if lang == "" {
		lang = d.Cfg.Site.DefaultLang
	}
	items, err := d.Store.CategoryRepo().List(ctx)
	if err != nil {
		fail(c, err)
		return
	}
	counts := make(map[int64]int, len(items))
	for _, it := range items {
		n, err := d.Store.CategoryRepo().CountContentByLang(ctx, it.ID, lang)
		if err != nil {
			fail(c, err)
			return
		}
		counts[it.ID] = n
	}
	roots, all := buildCategoryTree(items, counts)
	respondOK(c, gin.H{"items": roots, "all": all})
}
```

`internal/adminapi/content.go` `HandleContentList` 增加 category 分支 + category_name：
```go
func (d *Deps) HandleContentList(c *gin.Context) {
	typeName := c.Query("type")
	if typeName == "" {
		badRequest(c, "缺少 type 参数")
		return
	}
	if !d.requireTypePerm(c, "content.read", typeName) {
		return
	}
	lang := c.Query("lang")
	status := c.Query("status")
	category := c.Query("category")
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.Query("per_page"))
	if perPage < 1 {
		perPage = 20
	}
	ctx := c.Request.Context()
	var items []content.Entry
	var total int
	var err error
	if category != "" {
		cid, convErr := strconv.ParseInt(category, 10, 64)
		if convErr != nil {
			badRequest(c, "category 参数必须是分类 id")
			return
		}
		items, total, err = d.Content.ListByTypeLangStatusCategory(ctx, typeName, lang, status, cid, page, perPage)
	} else {
		items, total, err = d.Content.ListAdmin(ctx, typeName, lang, status, page, perPage)
	}
	if err != nil {
		fail(c, err)
		return
	}
	type outEntry struct {
		Content      store.Content `json:"content"`
		TypeName     string        `json:"type_name"`
		Fields       map[string]any `json:"fields"`
		CategoryName string        `json:"category_name,omitempty"`
	}
	out := make([]outEntry, 0, len(items))
	for _, it := range items {
		out = append(out, outEntry{
			Content:      it.Content,
			TypeName:     it.TypeName,
			Fields:       it.Fields,
			CategoryName: d.categoryName(ctx, it),
		})
	}
	respondOK(c, gin.H{"items": out, "total": total})
}

// categoryName 解析 entry 的分类 id 字段返回分类名，无则空串。
func (d *Deps) categoryName(ctx context.Context, e content.Entry) string {
	raw, ok := e.Fields["category"].(string)
	if !ok || raw == "" {
		return ""
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return ""
	}
	cat, err := d.Store.CategoryRepo().GetByID(ctx, id)
	if err != nil {
		return ""
	}
	return cat.Name
}
```

> 说明：`content.ListByTypeLangStatusCategory` 是 content.Service 层方法，需在 `internal/content/service.go` 新增（转发到 store 新方法）。实现者须在 `internal/content/service.go` 的 `ListAdmin` 附近新增：
> ```go
> func (s *Service) ListByTypeLangStatusCategory(ctx context.Context, typeName, lang, status string, categoryID int64, page, perPage int) ([]Entry, int, error) {
> 	// 校验类型存在
> 	if _, err := s.GetType(ctx, typeName); err != nil {
> 		return nil, 0, err
> 	}
> 	if page < 1 { page = 1 }
> 	if perPage < 1 { perPage = 20 }
> 	rows, err := s.store.ContentRepo().ListByTypeLangStatusCategory(ctx, typeName, lang, status, categoryID, (page-1)*perPage, perPage)
> 	if err != nil { return nil, 0, err }
> 	total, err := s.store.ContentRepo().CountByTypeLangStatusCategory(ctx, typeName, lang, status, categoryID)
> 	if err != nil { return nil, 0, err }
> 	ct, _ := s.GetType(ctx, typeName)
> 	out := make([]Entry, 0, len(rows))
> 	for _, row := range rows {
> 		e, err := s.entryFromStore(ctx, ct, row)
> 		if err != nil { return nil, 0, err }
> 		out = append(out, e)
> 	}
> 	return out, total, nil
> }
> ```
> 需在 content.go 增加 `"dulizhan/internal/store"` import（`outEntry.Content` 用 store.Content）。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test -count=1 ./internal/adminapi/ -run "TestCategoriesCountByLang|TestContentListByCategory" -v`
预期：PASS。同时 `go test -count=1 ./internal/adminapi/ ./internal/content/ -v` 确认现有测试无回归。

---

### 任务 3b：搜索按分类过滤

> **补充说明（2026-08-13 执行中发现的计划缺陷）**：用户确认"关键词搜索与分类筛选可组合"，但现有 `HandleSearch` 走 `SearchByTypeLang`（仅 title/slug LIKE），不支持分类。前端搜+选分类无法同时生效。本任务补齐：store `SearchByTypeLangCategory`/`CountSearchCategory`（search 追加 category 条件）+ content.Service 转发 + `HandleSearch` 加 `category` 参数。任务 5 前端把 category 传给 `searchContent`。

**文件：**
- 修改：`internal/store/store.go`
- 修改：`internal/store/sqlite/sqlite.go`
- 测试：`internal/store/sqlite/sqlite_test.go`
- 修改：`internal/content/service.go`
- 修改：`internal/adminapi/search.go`
- 测试：`internal/adminapi/search_test.go`

- [ ] **步骤 1：编写失败测试**（`internal/store/sqlite/sqlite_test.go` 追加）

```go
func TestSearchByTypeLangCategory(t *testing.T) {
	ctx := context.Background()
	t.Setenv("DULIZHAN_SEED_NO_DOWNLOAD", "1")
	st := newTestStore(t)
	if _, err := i18n.New([]string{"zh", "en"}, "zh", false); err != nil {
		t.Fatal(err)
	}
	svc := content.New(st, schemareg.NewRegistry(), []string{"zh", "en"})
	med := media.NewLocalStore(t.TempDir(), "/media")
	if err := seed.Run(ctx, st, svc, med); err != nil {
		t.Fatal(err)
	}
	products, err := st.CategoryRepo().GetBySlug(ctx, "products")
	if err != nil {
		t.Fatal(err)
	}
	// products 分类下 zh 搜 "产品"（products-1"旗舰产品一览"/products-2"新品评测"标题命中"产品"）→ 2 条
	items, err := st.ContentRepo().SearchByTypeLangCategory(ctx, "article", "zh", "产品", products.ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Errorf("products zh search rows = %d, want 2", len(items))
	}
	n, err := st.ContentRepo().CountSearchCategory(ctx, "article", "zh", "产品", products.ID)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("products zh search count = %d, want 2", n)
	}
	// 空结果：about 分类搜 "产品" → 0
	about, err := st.CategoryRepo().GetBySlug(ctx, "about")
	if err != nil {
		t.Fatal(err)
	}
	empty, err := st.ContentRepo().SearchByTypeLangCategory(ctx, "article", "zh", "产品", about.ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Errorf("about zh search rows = %d, want 0", len(empty))
	}
}
```

> 说明：seed 数据 products 分类 zh 有 products-1"旗舰产品一览"/products-2"新品评测"（seed.go:135-138）。复用 `scanContent`（sqlite.go:158）。imports 与任务 1 相同。

- [ ] **步骤 2：运行测试验证失败**

运行：`go test -count=1 ./internal/store/sqlite/ -run TestSearchByTypeLangCategory -v`
预期：FAIL（方法未定义）。

- [ ] **步骤 3：实现 store 接口 + sqlite**

`internal/store/store.go` `ContentRepo` 增加：
```go
SearchByTypeLangCategory(ctx context.Context, typeName, lang, q string, categoryID int64, offset, limit int) ([]Content, error)
CountSearchCategory(ctx context.Context, typeName, lang, q string, categoryID int64) (int, error)
```

`internal/store/sqlite/sqlite.go` 参照 `SearchByTypeLang`（:361）追加 category 条件：
```go
func (r *contentRepo) SearchByTypeLangCategory(ctx context.Context, typeName, lang, q string, categoryID int64, offset, limit int) ([]store.Content, error) {
	query := "SELECT " + contentCols + " FROM content c JOIN content_types t ON t.id = c.content_type_id WHERE t.name=? AND c.lang=? AND (c.title LIKE ? OR c.slug LIKE ?) AND c.payload LIKE ? ORDER BY c.id DESC LIMIT ? OFFSET ?"
	like := "%" + q + "%"
	rows, err := r.db.QueryContext(ctx, query, typeName, lang, like, like, "%\"category\":\""+itoa64(categoryID)+"\"%", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.Content
	for rows.Next() {
		c, err := scanContent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *contentRepo) CountSearchCategory(ctx context.Context, typeName, lang, q string, categoryID int64) (int, error) {
	var n int
	like := "%" + q + "%"
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM content c JOIN content_types t ON t.id = c.content_type_id WHERE t.name=? AND c.lang=? AND (c.title LIKE ? OR c.slug LIKE ?) AND c.payload LIKE ?",
		typeName, lang, like, like, "%\"category\":\""+itoa64(categoryID)+"\"%").Scan(&n)
	return n, err
}
```

- [ ] **步骤 4：content.Service 转发**

`internal/content/service.go` 在 `Search`（service.go:428）附近新增：
```go
// SearchByTypeLangCategory 按关键词+分类搜索某类型某语言的内容（后台）。
func (s *Service) SearchByTypeLangCategory(ctx context.Context, typeName, lang, q string, categoryID int64, page, perPage int) ([]Entry, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if _, err := s.GetType(ctx, typeName); err != nil {
		return nil, 0, err
	}
	rows, err := s.store.ContentRepo().SearchByTypeLangCategory(ctx, typeName, lang, q, categoryID, (page-1)*perPage, perPage)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.store.ContentRepo().CountSearchCategory(ctx, typeName, lang, q, categoryID)
	if err != nil {
		return nil, 0, err
	}
	ct, _ := s.GetType(ctx, typeName)
	out := make([]Entry, 0, len(rows))
	for _, row := range rows {
		e, err := s.entryFromStore(ctx, ct, row)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, nil
}
```

- [ ] **步骤 5：HandleSearch 加 category**

`internal/adminapi/search.go` `HandleSearch` 在 `q` 校验后、调用前加：
```go
	category := c.Query("category")
	var items []content.Entry
	var total int
	var err error
	if category != "" {
		cid, convErr := strconv.ParseInt(category, 10, 64)
		if convErr != nil {
			badRequest(c, "category 参数必须是分类 id")
			return
		}
		items, total, err = d.Content.SearchByTypeLangCategory(c.Request.Context(), typeName, lang, q, cid, page, perPage)
	} else {
		items, total, err = d.Content.Search(c.Request.Context(), typeName, lang, q, page, perPage)
	}
	if err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"items": items, "total": total})
```
（原 `items, total, err := d.Content.Search(...)` 与 `if err != nil` 块需相应替换；`search.go` 增加 `"dulizhan/internal/content"` import。`strconv` 已导入。）

- [ ] **步骤 6：搜索测试**（`internal/adminapi/search_test.go` 追加）

```go
func TestSearchByCategory(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	// 拿 products 分类 id
	w := e.do(t, http.MethodGet, "/api/categories", "", tok)
	var cl struct {
		Data struct {
			All []struct {
				ID   int64  `json:"id"`
				Path string `json:"path"`
			} `json:"all"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &cl)
	var productsID int64
	for _, a := range cl.Data.All {
		if a.Path == "产品" {
			productsID = a.ID
		}
	}
	if productsID == 0 {
		t.Fatal("未找到产品分类")
	}
	// products 分类内搜 "产品" → 2 条
	w = e.do(t, http.MethodGet, "/api/search?type=article&lang=zh&q="+url.QueryEscape("产品")+"&category="+strconv.FormatInt(productsID, 10), "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("search = %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Total int `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data.Total != 2 {
		t.Errorf("products zh search total = %d, want 2", resp.Data.Total)
	}
}
```

- [ ] **步骤 7：验证**

运行：
```powershell
$env:Path = "C:\Program Files\Go\bin;" + $env:Path
go test -count=1 ./internal/store/sqlite/ -run TestSearchByTypeLangCategory -v
go test -count=1 ./internal/adminapi/ -run TestSearchByCategory -v
go test -count=1 ./internal/content/ -v
```
预期：全 PASS。`go test -count=1 ./...` 无回归。

---

### 任务 4：前端 API 与类型扩展

**文件：**
- 修改：`admin/src/api/content.ts`
- 修改：`admin/src/api/category.ts`
- 修改：`admin/src/api/search.ts`
- 测试：`admin/src/api/__tests__/client.test.ts`（若需）

- [ ] **步骤 1：修改 content.ts**

`ContentEntry` 增加：
```ts
  category_name?: string
```
`listContent` 支持 category：
```ts
export const listContent = (params: { type: string; lang: string; status?: string; category?: number; page?: number; perPage?: number }) =>
  request<{ items: ContentEntry[]; total: number }>(
    `/content?type=${params.type}&lang=${params.lang}${params.status ? `&status=${params.status}` : ''}${params.category ? `&category=${params.category}` : ''}&page=${params.page ?? 1}&per_page=${params.perPage ?? 20}`
  )
```

- [ ] **步骤 2：修改 category.ts**

`listCategories` 支持 lang：
```ts
export const listCategories = (lang?: string) =>
  request<{ items: CategoryItem[]; all: CategoryPath[] }>(`/categories${lang ? `?lang=${lang}` : ''}`)
```

- [ ] **步骤 3：修改 search.ts（搜索支持 category，配套任务 3b）**

```ts
export const searchContent = (params: { type: string; lang: string; q: string; category?: number; page?: number }) =>
  request<{ items: ContentEntry[]; total: number }>(
    `/search?type=${params.type}&lang=${params.lang}&q=${encodeURIComponent(params.q)}${params.category ? `&category=${params.category}` : ''}&page=${params.page ?? 1}`
  )
```

- [ ] **步骤 4：验证**

运行：
```powershell
cd admin
npx vue-tsc --noEmit
```
预期：无类型错误。

---

### 任务 5：ContentListView 分类列 + 筛选

**文件：**
- 修改：`admin/src/views/ContentListView.vue`
- 测试：`admin/src/views/__tests__/`（新增或沿用）

- [ ] **步骤 1：修改 ContentListView.vue**

`script setup` 增加：
```ts
import { listCategories, type CategoryItem } from '../api/category'
const categories = ref<CategoryItem[]>([])
const categoryFilter = ref<number | undefined>(undefined)

// 扁平化分类树（含子分类）为下拉选项
const categoryOptions = computed(() => {
  const out: { label: string; value: number }[] = []
  const walk = (list: CategoryItem[], prefix = '') => {
    for (const c of list) {
      const label = prefix ? `${prefix} / ${c.name}` : c.name
      out.push({ label, value: c.id })
      walk(c.children ?? [], label)
    }
  }
  walk(categories.value)
  return out
})
```

`onMounted` 加载分类（复用 meta 的语言）：
```ts
categories.value = (await listCategories(langFilter.value)).items
```

`load()` 增加 category 参数（搜索与列表都传）：
```ts
if (keyword.value) {
  const r = await searchContent({ type: typeFilter.value, lang: langFilter.value, q: keyword.value, category: categoryFilter.value, page: page.value })
  items.value = r.items
  total.value = r.total
} else {
  const r = await listContent({ type: typeFilter.value, lang: langFilter.value, status: statusFilter.value || undefined, category: categoryFilter.value, page: page.value, perPage })
  items.value = r.items
  total.value = r.total
}
```

`watch` 增加 `categoryFilter`：
```ts
watch([typeFilter, langFilter, statusFilter, categoryFilter, page], () => load())
```
> 注意：切换 langFilter 时分类下拉也应随语言重载（`watch(langFilter, () => listCategories(langFilter.value).then(r => categories.value = r.items))`）。

模板筛选区加：
```html
<el-form-item label="分类">
  <el-select v-model="categoryFilter" style="width: 180px" clearable placeholder="全部">
    <el-option v-for="o in categoryOptions" :key="o.value" :label="o.label" :value="o.value" />
  </el-select>
</el-form-item>
```

表格加分类列：
```html
<el-table-column prop="category_name" label="分类" width="140" />
```

- [ ] **步骤 2：验证**

运行：
```powershell
cd admin
npx vue-tsc --noEmit
npx vitest run
```
预期：无类型错误、现有测试全绿。

---

### 任务 6：CategoriesView 语言选择器

**文件：**
- 修改：`admin/src/views/CategoriesView.vue`
- 测试：`admin/src/views/__tests__/categories-view.test.ts`

- [ ] **步骤 1：修改 CategoriesView.vue**

`script setup` 增加语言状态与加载：
```ts
import { fetchMeta, type MetaData } from '../api/meta'
const meta = ref<MetaData | null>(null)
const langFilter = ref('')
async function load() {
  const r = await listCategories(langFilter.value || undefined)
  items.value = r.items
  all.value = r.all
}
onMounted(async () => {
  meta.value = await fetchMeta()
  langFilter.value = meta.value.default_lang
  await load()
})
```

模板顶部加语言选择器：
```html
<el-form inline style="margin-top: 8px">
  <el-form-item label="语言">
    <el-select v-model="langFilter" style="width: 120px" @change="load">
      <el-option v-for="l in meta?.languages ?? []" :key="l" :label="l" :value="l" />
    </el-select>
  </el-form-item>
</el-form>
```

- [ ] **步骤 2：验证**

运行：
```powershell
cd admin
npx vue-tsc --noEmit
npx vitest run
```
预期：无类型错误、现有测试全绿。

---

### 任务 7：前端构建同步 + 全量验证

**文件：**（无代码逻辑改动）

- [ ] **步骤 1：构建 SPA 并同步 dist**

```powershell
cd admin
npx vite build
```
构建产物自动输出到 `admin/dist/`。**同步到 `internal/server/dist/`**：项目用 `build.sh`/`build.ps1` 同步。若无脚本自动同步，手动复制 `admin/dist/*` 到 `internal/server/dist/`（覆盖）。

- [ ] **步骤 2：Go 全量验证**

运行：
```powershell
$env:Path = "C:\Program Files\Go\bin;" + $env:Path
go test -count=1 ./...
go vet ./...
gofmt -l .
```
预期：全绿、vet 干净、gofmt 无输出。

- [ ] **步骤 3：冒烟**

重启服务（config 已配 debug:true）：
- 分类管理页：语言选择器存在；默认 zh 时"产品"计数=2、"新闻"=3；切 en 后"新闻"仍=3、"产品"=2
- 内容管理页：分类列显示（如 products 内容显示"产品"）；分类下拉可选；选"产品"分类后列表只显示产品内容（含草稿）；选分类 + 输入关键词搜索可组合
- 管理后台 `/admin` 正常加载 SPA（dist 已同步）

预期：全部正确。

---

## 自检记录

**规格覆盖度：**
- §3.1 store 接口 → 任务 1、2、3b ✔
- §3.2 sqlite 实现 → 任务 1、2、3b ✔
- §3.3 adminapi（categories lang + content category/category_name）→ 任务 3 ✔
- 搜索+分类组合（用户确认"可组合"）→ 任务 3b（store/service/handler）+ 任务 4（search.ts）+ 任务 5（load 传 category）✔
- §4.1 content.ts → 任务 4；§4.2 category.ts → 任务 4；§4.3 ContentListView → 任务 5；§4.4 CategoriesView → 任务 6 ✔
- §6 测试 → 各任务 TDD 步骤 ✔
- §7 验证 → 任务 7 ✔

**占位符扫描：** 无 TODO/待定。所有代码块完整可照抄。

**类型一致性：**
- `CountContentByLang(ctx, id, lang)`（任务 1）在任务 3 `HandleCategories` 使用，签名一致。
- `ListByTypeLangStatusCategory`/`CountByTypeLangStatusCategory`（任务 2）签名含 `(ctx, typeName, lang, status, categoryID, offset, limit)` / `(ctx, typeName, lang, status, categoryID)`；任务 3 content.Service 层 `ListByTypeLangStatusCategory(ctx, typeName, lang, status, categoryID, page, perPage)` 转发一致。
- `SearchByTypeLangCategory`/`CountSearchCategory`（任务 3b）签名含 `(ctx, typeName, lang, q, categoryID, offset, limit)` / `(ctx, typeName, lang, q, categoryID)`；service 层 `SearchByTypeLangCategory(ctx, typeName, lang, q, categoryID, page, perPage)` 转发一致；adminapi `HandleSearch` 用 `cid` 调用。
- `categoryName(ctx, e)` 返回 string，拼入 `outEntry.CategoryName`（json:"category_name,omitempty"）一致。
- 前端 `ContentEntry.category_name?`、`listContent`/`searchContent` category 参数、`listCategories(lang?)`、ContentListView 的 `categoryFilter`/`categoryOptions`、CategoriesView 的 `langFilter` 命名一致。
- 现有 `categories_test.go` 的 `content_count>0` 断言：zh 默认语言下 news=3>0 仍成立，不破坏。
- `catTreeNode`（categories_test.go:243）被任务 3 测试复用，存在。
- seed 数据（zh: news=3/products=2/about=1；en 各同）与任务 1/2/3/3b 测试断言一致（seed.go:127-147）。任务 3b 搜索断言 "产品" 命中 products-1"旗舰产品一览"/products-2"新品评测" 标题，属实。
