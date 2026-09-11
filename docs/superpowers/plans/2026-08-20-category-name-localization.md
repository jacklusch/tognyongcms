# 分类英文名（name_en）本地化 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标：** 分类增加 `name_en` 字段，英文站分类页标题与分类显示名用英文，中文站保持中文。

**架构：** DB 迁移加列 → store 模型/repo → admin API/UI → seed 回填 → server 前台按语言取显示名 → 数据回填与重建。

**技术栈：** Go + SQLite + Gin + Vue3。

**约定：** 仓库不 git 提交（AGENTS.md）。改 Go 后跑 `go test -count=1 ./...`；改 SPA 后跑 `npx vue-tsc --noEmit && npx vitest run` + `.\build.ps1`。

**规格：** `docs/superpowers/specs/2026-08-20-category-name-localization-design.md`

---

### 任务 1：DB 迁移 + store 模型 + sqlite repo（name_en）

**文件：**
- 修改：`internal/store/sqlite/migrate.go`
- 修改：`internal/store/sqlite/sqlite.go`
- 修改：`internal/store/model.go`
- 测试：`internal/store/sqlite/sqlite_test.go`

- [ ] **步骤 1：写失败测试**

`internal/store/sqlite/sqlite_test.go` 追加：

```go
func TestCategoryNameEn(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	repo := st.CategoryRepo()
	c := &store.Category{Name: "产品", NameEn: "Products", Slug: "products-en"}
	if err := repo.Create(ctx, c); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repo.GetByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.NameEn != "Products" {
		t.Errorf("NameEn = %q, want Products", got.NameEn)
	}
	got.NameEn = "Prod"
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got2, err := repo.GetBySlug(ctx, "products-en")
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if got2.NameEn != "Prod" {
		t.Errorf("update 后 NameEn = %q, want Prod", got2.NameEn)
	}
	// 子分类场景：Descendants/ListChildren 也应带 name_en
	kid := &store.Category{Name: "斩拌机", NameEn: "Chopper", Slug: "chopper-en", ParentID: c.ID}
	if err := repo.Create(ctx, kid); err != nil {
		t.Fatalf("Create kid: %v", err)
	}
	kids, err := repo.ListChildren(ctx, c.ID)
	if err != nil || len(kids) != 1 || kids[0].NameEn != "Chopper" {
		t.Errorf("ListChildren NameEn = %+v, %v", kids, err)
	}
	desc, err := repo.Descendants(ctx, c.ID)
	if err != nil || len(desc) != 1 || desc[0].NameEn != "Chopper" {
		t.Errorf("Descendants NameEn = %+v, %v", desc, err)
	}
	all, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	found := false
	for _, x := range all {
		if x.ID == c.ID && x.NameEn == "Prod" {
			found = true
		}
	}
	if !found {
		t.Errorf("List 未带回 name_en: %+v", all)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test -count=1 ./internal/store/sqlite/ -run TestCategoryNameEn -v`
预期：FAIL（编译错 `store.Category` 无 `NameEn`；或 name_en 列不存在报错）。

- [ ] **步骤 3：实现**

`internal/store/model.go` 的 `Category` 增加字段：

```go
type Category struct {
	ID          int64     `json:"id"`
	ParentID    int64     `json:"parent_id"`
	Name        string    `json:"name"`
	NameEn      string    `json:"name_en"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}
```

`internal/store/sqlite/migrate.go` 新增函数（复用 `migrateCategoriesParent` 模式）：

```go
// migrateCategoriesNameEn 迁移守卫：给 categories 表补 name_en 列（英文名，可空）。
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

`internal/store/sqlite/sqlite.go`：
- `Open` 中在 `migrateCategoriesParent(db)` 之后追加：

```go
	if err := migrateCategoriesNameEn(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
```

- `scanCategory` 改为扫描 name_en（第 2 列 parent_id 之后）：

```go
func scanCategory(row interface{ Scan(...any) error }) (store.Category, error) {
	var c store.Category
	var created string
	err := row.Scan(&c.ID, &c.ParentID, &c.Name, &c.NameEn, &c.Slug, &c.Description, &created)
	if err != nil {
		return c, wrapErr(err)
	}
	c.CreatedAt, _ = time.Parse(tsLayout, created)
	return c, nil
}
```

- `List` / `GetByID` / `GetBySlug` / `ListChildren` / `Descendants` 的 SELECT 列清单改为 `id, parent_id, name, name_en, slug, description, created_at`（共 5 处；`Descendants` 的 CTE 内也有 `SELECT c.id, c.parent_id, c.name, c.slug, c.description, c.created_at` 需改为含 name_en）。
- `Create`：

```go
func (r *categoryRepo) Create(ctx context.Context, c *store.Category) error {
	c.CreatedAt = time.Now().UTC()
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO categories (name, name_en, slug, description, created_at, parent_id) VALUES (?,?,?,?,?,?)",
		c.Name, c.NameEn, c.Slug, c.Description, c.CreatedAt.Format(tsLayout), c.ParentID)
	if err != nil {
		return wrapUnique(err, "分类 slug 已存在")
	}
	c.ID, err = res.LastInsertId()
	return err
}
```

- `Update`：

```go
func (r *categoryRepo) Update(ctx context.Context, c *store.Category) error {
	_, err := r.db.ExecContext(ctx, "UPDATE categories SET name=?, name_en=?, slug=?, description=?, parent_id=? WHERE id=?",
		c.Name, c.NameEn, c.Slug, c.Description, c.ParentID, c.ID)
	return wrapUnique(err, "分类 slug 已存在")
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test -count=1 ./internal/store/sqlite/`
预期：PASS（含既有 category 测试）。

---

### 任务 2：admin API（name_en 收发）

**文件：**
- 修改：`internal/adminapi/categories.go`
- 测试：`internal/adminapi/categories_test.go`

- [ ] **步骤 1：写失败测试**

`internal/adminapi/categories_test.go` 追加（用现有测试的建服务器/登录 helper，参照既有用例）：

```go
func TestCategoryNameEnAPI(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	// 创建带 name_en
	var created struct {
		Data struct {
			Category struct {
				ID     int64  `json:"id"`
				Name   string `json:"name"`
				NameEn string `json:"name_en"`
			} `json:"category"`
		} `json:"data"`
	}
	w := e.do(t, http.MethodPost, "/api/categories", `{"name":"产品","name_en":"Products","slug":"prod-en"}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("create = %d %s", w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if created.Data.Category.NameEn != "Products" {
		t.Errorf("create name_en = %q", created.Data.Category.NameEn)
	}
	// 更新 name_en
	w = e.do(t, http.MethodPut, "/api/categories/"+strconv.FormatInt(created.Data.Category.ID, 10),
		`{"name":"产品","name_en":"Product","slug":"prod-en"}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("update = %d %s", w.Code, w.Body.String())
	}
	// 列表返回 name_en
	var list struct {
		Data struct {
			Items []struct {
				NameEn string `json:"name_en"`
			} `json:"items"`
		} `json:"data"`
	}
	w = e.do(t, http.MethodGet, "/api/categories", "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("list = %d %s", w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	found := false
	for _, it := range list.Data.Items {
		if it.NameEn == "Product" {
			found = true
		}
	}
	if !found {
		t.Errorf("列表未返回 name_en=Product: %+v", list.Data.Items)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test -count=1 ./internal/adminapi/ -run TestCategoryNameEnAPI -v`
预期：FAIL（`name_en` 未解析/未返回）。

- [ ] **步骤 3：实现**

`internal/adminapi/categories.go`：

- `categoryReq` 增加：

```go
type categoryReq struct {
	Name        string `json:"name"`
	NameEn      string `json:"name_en"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	ParentID    int64  `json:"parent_id"`
}
```

- `buildCategoryTree` 的 node 增加 nameEn：

```go
	type node struct {
		id         int64
		parentID   int64
		name       string
		nameEn     string
		slug       string
		desc       string
		count      int
		totalCount int
		children   []*node
	}
```
并把 `byID[c.ID] = &node{..., nameEn: c.NameEn, ...}`；`toH` 的 gin.H 增加 `"name_en": n.nameEn`。

- `HandleCategoryCreate`：

```go
	cat := &store.Category{Name: req.Name, NameEn: req.NameEn, Slug: req.Slug, Description: req.Description, ParentID: req.ParentID}
```

- `HandleCategoryUpdate`：

```go
	cat := &store.Category{ID: id, Name: req.Name, NameEn: req.NameEn, Slug: req.Slug, Description: req.Description, ParentID: req.ParentID}
```

- `validateCategory` 不校验 name_en（保持可空）。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test -count=1 ./internal/adminapi/`
预期：PASS。

---

### 任务 3：admin UI（英文名称输入框）

**文件：**
- 修改：`admin/src/api/category.ts`
- 修改：`admin/src/views/CategoriesView.vue`

- [ ] **步骤 1：实现**

`admin/src/api/category.ts`：
- `CategoryItem` 增加 `name_en: string`。
- `createCategory`/`updateCategory` body 类型增加 `name_en?: string`：

```ts
export const createCategory = (body: { name: string; name_en?: string; slug?: string; description?: string; parent_id?: number }) =>
  request<{ category: CategoryItem }>('/categories', { method: 'POST', body })
export const updateCategory = (id: number, body: { name: string; name_en?: string; slug: string; description?: string; parent_id?: number }) =>
  request<{ category: CategoryItem }>(`/categories/${id}`, { method: 'PUT', body })
```

`admin/src/views/CategoriesView.vue`：
- 表单初始化处（`form` 定义/`openCreate`/`openEdit`）确保含 `name_en`（空字符串），编辑时回填 `row.name_en`。
- 表单对话框在「名称」后新增一项：

```html
        <el-form-item label="英文名称"><el-input v-model="form.name_en" placeholder="英文模式显示的分类名（可空）" /></el-form-item>
```

- `save()` 的请求体包含 `name_en: form.name_en`。
- 表格列可加显示英文名（可选，不改也行）。

- [ ] **步骤 2：前端校验**

运行：`cd admin && npx vue-tsc --noEmit && npx vitest run`
预期：typecheck 无输出、37 个测试通过。

---

### 任务 4：seed 回填英文名

**文件：**
- 修改：`internal/seed/seed.go`
- 测试：`internal/seed/seed_test.go`

- [ ] **步骤 1：写失败测试**

`internal/seed/seed_test.go` 追加（复用现有测试环境）：

```go
func TestSeedCategoryNameEnBackfill(t *testing.T) {
	t.Setenv("DULIZHAN_SEED_NO_DOWNLOAD", "1")
	ctx := context.Background()
	st, _ := sqlite.Open(":memory:")
	reg := schema.NewRegistry()
	svc := content.New(st, reg, []string{"zh", "en"})
	// 先跑一次 seed 建出分类，再把 news 的 name_en 清空模拟旧库
	if err := Run(ctx, st, svc, &mockMediaStore{}); err != nil {
		t.Fatal(err)
	}
	news, err := st.CategoryRepo().GetBySlug(ctx, "news")
	if err != nil {
		t.Fatal(err)
	}
	news.NameEn = ""
	if err := st.CategoryRepo().Update(ctx, &news); err != nil {
		t.Fatal(err)
	}
	// 再跑 seed，应回填 name_en=News（幂等）
	if err := Run(ctx, st, svc, &mockMediaStore{}); err != nil {
		t.Fatal(err)
	}
	cat, err := st.CategoryRepo().GetBySlug(ctx, "news")
	if err != nil {
		t.Fatalf("GetBySlug news: %v", err)
	}
	if cat.NameEn != "News" {
		t.Errorf("news name_en = %q, want News", cat.NameEn)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test -count=1 ./internal/seed/ -run TestSeedCategoryNameEnBackfill -v`
预期：FAIL（`news name_en = "", want News`）。

- [ ] **步骤 3：实现**

`internal/seed/seed.go`：

- 结构加 `NameEn`：

```go
	demoCategories := []struct{ Name, NameEn, Slug string }{
		{"新闻", "News", "news"}, {"关于", "About", "about"}, {"产品", "Products", "products"},
	}
```

- 顶级分类循环，已存在分支回填 name_en：

```go
	for _, c := range demoCategories {
		if existing, err := st.CategoryRepo().GetBySlug(ctx, c.Slug); err == nil {
			if existing.NameEn == "" && c.NameEn != "" {
				existing.NameEn = c.NameEn
				if err := st.CategoryRepo().Update(ctx, &existing); err != nil {
					return fmt.Errorf("回填分类 %s 英文名: %w", c.Slug, err)
				}
			}
			catIDs[c.Slug] = existing.ID
			continue
		}
		cat := &store.Category{Name: c.Name, NameEn: c.NameEn, Slug: c.Slug}
		if err := st.CategoryRepo().Create(ctx, cat); err != nil {
			return fmt.Errorf("创建分类 %s: %w", c.Slug, err)
		}
		catIDs[c.Slug] = cat.ID
	}
```

- 子分类：

```go
	subCategories := map[string][]struct{ Name, NameEn, Slug string }{
		"products": {{"斩拌机", "Chopper", "chopper"}, {"香肠机", "Sausage Machine", "sausage-machine"}, {"拌馅机", "Mixer", "mixer"}},
	}
```

子分类循环同样处理已存在分支回填（逻辑同顶级）：

```go
	for parentSlug, subs := range subCategories {
		for _, sc := range subs {
			if existing, err := st.CategoryRepo().GetBySlug(ctx, sc.Slug); err == nil {
				if existing.NameEn == "" && sc.NameEn != "" {
					existing.NameEn = sc.NameEn
					if err := st.CategoryRepo().Update(ctx, &existing); err != nil {
						return fmt.Errorf("回填子分类 %s 英文名: %w", sc.Slug, err)
					}
				}
				catIDs[sc.Slug] = existing.ID
				continue
			}
			cat := &store.Category{Name: sc.Name, NameEn: sc.NameEn, Slug: sc.Slug, ParentID: catIDs[parentSlug]}
			if err := st.CategoryRepo().Create(ctx, cat); err != nil {
				return fmt.Errorf("创建子分类 %s: %w", sc.Slug, err)
			}
			catIDs[sc.Slug] = cat.ID
		}
	}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test -count=1 ./internal/seed/`
预期：PASS。

---

### 任务 5：前台本地化（catName + 应用点）

**文件：**
- 修改：`internal/server/frontend.go`
- 测试：`internal/server/frontend_test.go`

- [ ] **步骤 1：写失败测试**

`internal/server/frontend_test.go` 追加：

```go
func TestCategoryTitleLocalized(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	srv.cfg.Site.Theme = "default"
	ctx := context.Background()
	// 给 demo 分类回填英文名（模拟 seed 已生效）
	for _, c := range []struct{ slug, en string }{
		{"products", "Products"}, {"news", "News"}, {"about", "About"},
		{"chopper", "Chopper"}, {"sausage-machine", "Sausage Machine"}, {"mixer", "Mixer"},
	} {
		cat, err := srv.store.CategoryRepo().GetBySlug(ctx, c.slug)
		if err != nil {
			continue
		}
		cat.NameEn = c.en
		if err := srv.store.CategoryRepo().Update(ctx, &cat); err != nil {
			t.Fatalf("回填 %s: %v", c.slug, err)
		}
	}
	// en 分类页 title 英文（buildTestServer 品牌名为「测试站点」，en 回退同名）
	code, body := get(t, srv, "/en/category/products")
	if code != http.StatusOK {
		t.Fatalf("/en/category/products = %d", code)
	}
	if !strings.Contains(body, "<title>Products - 测试站点</title>") {
		t.Errorf("en 分类页 title 应为英文: %s", body)
	}
	if !strings.Contains(body, ">Chopper<") {
		t.Errorf("en 子分类导航应显示英文名: %s", body)
	}
	// zh 分类页 title 中文
	code, body = get(t, srv, "/category/products")
	if code != http.StatusOK {
		t.Fatalf("/category/products = %d", code)
	}
	if !strings.Contains(body, "<title>产品 - 测试站点</title>") {
		t.Errorf("zh 分类页 title 应为中文: %s", body)
	}
}

func TestEntryCategoryNameLocalized(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	srv.cfg.Site.Theme = "default"
	ctx := context.Background()
	cat, err := srv.store.CategoryRepo().GetBySlug(ctx, "chopper")
	if err != nil {
		t.Fatalf("chopper: %v", err)
	}
	cat.NameEn = "Chopper"
	if err := srv.store.CategoryRepo().Update(ctx, &cat); err != nil {
		t.Fatal(err)
	}
	// en 详情页：面包屑/EntryCategory 用英文
	code, body := get(t, srv, "/en/article/chopper-1-en")
	if code != http.StatusOK {
		t.Fatalf("en 详情 = %d", code)
	}
	if !strings.Contains(body, `"Chopper"`) {
		t.Errorf("en 详情页分类名应为英文 Chopper: %s", body)
	}
}
```

> 注意：`buildTestServer` 的 `cfg.Site.Name` 是「测试站点」且未配 `Names` map，故 en 品牌名回退「测试站点」，断言用 `Products - 测试站点`。

- [ ] **步骤 2：运行测试验证失败**

运行：`go test -count=1 ./internal/server/ -run 'TestCategoryTitleLocalized|TestEntryCategoryNameLocalized' -v`
预期：FAIL（en 分类页 title 仍是中文）。

- [ ] **步骤 3：实现**

`internal/server/frontend.go` 新增：

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

应用点：
- `renderCategory`：
  - `data.EntryCategory = cat.Name` → `data.EntryCategory = s.catName(cat, lang)`
  - `data.Meta = s.seo.BuildCategory(lang, pathSegs, cat.Name, all)` → 用 `s.catName(cat, lang)`
  - 子分类导航循环内 `Name: ch.Name` → `Name: s.catName(ch, lang)`
- `renderSingle`：`data.EntryCategory = cat.Name` → `data.EntryCategory = s.catName(cat, lang)`
- `entryBreadcrumbs`：`out = append(out, seo.Breadcrumb{Name: chain[i].Name, ...})` → `Name: s.catName(chain[i], lang)`

- [ ] **步骤 4：运行测试验证通过**

运行：`go test -count=1 ./internal/server/ -run 'TestCategoryTitleLocalized|TestEntryCategoryNameLocalized' -v`
预期：PASS。

- [ ] **步骤 5：全量回归**

运行：`go test -count=1 ./...`
预期：PASS。

---

### 任务 6：数据回填 + 全量验证 + 重建 + 重启

**文件：** 无（操作）

- [ ] **步骤 1：Go 全量校验**

运行：`go test -count=1 ./...; if ($?) { go vet ./...; gofmt -l . }`
预期：全部通过、gofmt 无输出。

- [ ] **步骤 2：前端校验（任务 3 改过）**

运行：`cd admin && npx vue-tsc --noEmit && npx vitest run`
预期：通过。

- [ ] **步骤 3：重建 + seed 回填 + 重启**

运行：`.\build.ps1`
然后：`Stop-Process -Name dulizhan -Force -ErrorAction SilentlyContinue; .\dulizhan.exe seed`（回填 name_en，幂等）
再启动：`Start-Process -FilePath .\dulizhan.exe -WorkingDirectory $PWD -RedirectStandardOutput out.log -RedirectStandardError err.log -WindowStyle Hidden`
预期：`out.log` 启动成功、8080 监听。

- [ ] **步骤 4：浏览器抽查**

- `http://localhost:8080/en/category/products`：`<title>Products - Jinbowei Machinery</title>`、h1 英文、子分类 Chopper/Sausage Machine/Mixer 英文。
- `http://localhost:8080/category/products`：title/h1 仍中文「产品」。
- `http://localhost:8080/en/category/products/chopper`：title「Chopper - Jinbowei Machinery」。
- en 产品详情页面包屑分类名英文。
