# 首页分区按分类取数据 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 首页"产品中心"取配置的产品分类（含子分类）内容、"新闻动态"取配置的新闻分类（含子分类）内容；分类未配置/不存在时回退 `.Items` 全量。样式零改动。

**架构：** config 增加 `site.home_products_category` / `site.home_news_category` 两个可选 slug；`theme.Data` 增加 `Products`/`News` 两个 `[]content.Entry` 字段；`renderHome` 按 slug 查分类→`Descendants` 递归聚合→`ListPublishedByCategories` 填入；`index.html` 产品中心/新闻动态换数据源（空则回退 `.Items`）。其余后台不动。

**技术栈：** Go（gin、html/template）、config YAML、默认主题模板。

**前置基线：** 规格 `docs/superpowers/specs/2026-08-13-home-category-sections-design.md` 已确认。`internal/theme` 的 `Data` 已有 `SubCategories []CategoryInfo` 等字段；`CategoryRepo` 有 `GetBySlug`/`Descendants`；`content.Service` 有 `ListPublishedByCategories(ctx, typeName, lang, ids, page, perPage)`；`renderCategory`（frontend.go:95-175）已演示聚合模式。

**环境约束（所有任务遵守）：**
- **不 git 提交**（仓库约定，直接 master 文件级审查）；勿运行 `go mod tidy`
- Go 1.26.5 已全局安装（`C:\Program Files\Go\bin`）；GOPROXY 已配 goproxy.cn
- 验证：`go test -count=1 ./...`、`go vet ./...`、`gofmt -l .`
- 主题在 `themes/`（运行期从磁盘读取），改模板/静态文件后 `server.debug: true` 热重载或重启；**改后台 Go 代码必须重启服务**
- 冒烟前确认 8080 无残留 dulizhan 进程
- 错误消息用中文

**验证基线（写任何代码前先跑一次）：**
```powershell
$env:Path = "C:\Program Files\Go\bin;" + $env:Path
go test -count=1 ./...
```
预期：全绿。

---

### 任务 1：配置项（config）

**文件：**
- 修改：`internal/config/config.go`
- 测试：`internal/config/config_test.go`
- 修改：`config.example.yaml`

新增 `site.home_products_category` / `site.home_news_category`（可选，默认空）。

- [ ] **步骤 1：编写失败测试**（`internal/config/config_test.go` 追加）

```go
func TestHomeCategoryConfig(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	yaml := `
site:
  url: "https://example.com"
  default_lang: "zh"
  languages: ["zh"]
  home_products_category: "products"
  home_news_category: "news"
`
	if err := os.WriteFile(p, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Site.HomeProductsCategory != "products" {
		t.Errorf("HomeProductsCategory = %q, want products", cfg.Site.HomeProductsCategory)
	}
	if cfg.Site.HomeNewsCategory != "news" {
		t.Errorf("HomeNewsCategory = %q, want news", cfg.Site.HomeNewsCategory)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test -count=1 ./internal/config/ -run TestHomeCategoryConfig -v`
预期：FAIL（编译错误：`cfg.Site.HomeProductsCategory` 未定义）。

- [ ] **步骤 3：实现配置字段**

`internal/config/config.go` `SiteConfig` 增加：
```go
HomeProductsCategory string `yaml:"home_products_category"`
HomeNewsCategory     string `yaml:"home_news_category"`
```
`applyEnv` 中 `set` 块追加：
```go
set("SITE_HOME_PRODUCTS_CATEGORY", &cfg.Site.HomeProductsCategory)
set("SITE_HOME_NEWS_CATEGORY", &cfg.Site.HomeNewsCategory)
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test -count=1 ./internal/config/ -v`
预期：PASS（含新用例）。

- [ ] **步骤 5：更新 config.example.yaml**

`config.example.yaml` 的 `site:` 段追加：
```yaml
  # home_products_category: "products"   # 首页"产品中心"取该分类（含子分类）内容
  # home_news_category: "news"           # 首页"新闻动态"取该分类（含子分类）内容
```

- [ ] **步骤 6：Commit（不执行——仓库约定）**

本仓库不 git 提交，跳过 commit 步骤（下同）。

---

### 任务 2：theme.Data 增加 Products/News 字段

**文件：**
- 修改：`internal/theme/theme.go`

`Data` 增加两个字段，供首页模板访问。

- [ ] **步骤 1：实现字段**

`internal/theme/theme.go` 的 `Data` 结构（`SubCategories` 附近）增加：
```go
Products []content.Entry `json:"products"` // 首页产品中心（分类聚合）
News     []content.Entry `json:"news"`     // 首页新闻动态（分类聚合）
```

- [ ] **步骤 2：验证编译**

运行：`go build ./...`
预期：编译通过。

---

### 任务 3：renderHome 分类聚合

**文件：**
- 修改：`internal/server/frontend.go`
- 测试：`internal/server/frontend_test.go`

`renderHome` 在现有 `.Items` 填充后，按配置 slug 查分类并聚合填入 `data.Products`/`data.News`。

- [ ] **步骤 1：编写失败测试**（`internal/server/frontend_test.go` 追加）

> **修正说明（2026-08-13 执行中发现）**：原计划测试为渲染级断言（检查 body 含 slug），但任务 4 才改模板数据源，任务 3 单独完成时 `data.Products` 已填充而模板仍渲染 `.Items`，渲染级断言必然失败。改为**单元级**测试：直接调用 `homeCategoryItems` 验证聚合结果，不依赖模板。渲染级验证由任务 5 冒烟覆盖。

```go
func TestHomeCategoryItems(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	ctx := context.Background()

	// products 分类含子分类（chopper/sausage-machine/mixer），聚合应为 5 篇
	items := srv.homeCategoryItems(ctx, "article", "zh", "products", 10)
	got := map[string]bool{}
	for _, it := range items {
		got[it.Content.Slug] = true
	}
	for _, slug := range []string{"chopper-1", "sausage-machine-1", "mixer-1", "products-1", "products-2"} {
		if !got[slug] {
			t.Errorf("products 分类聚合缺 %s", slug)
		}
	}

	// news 分类应为 3 篇
	news := srv.homeCategoryItems(ctx, "article", "zh", "news", 10)
	newsSlugs := map[string]bool{}
	for _, it := range news {
		newsSlugs[it.Content.Slug] = true
	}
	for _, slug := range []string{"news-1", "news-2", "hello-zh"} {
		if !newsSlugs[slug] {
			t.Errorf("news 分类聚合缺 %s", slug)
		}
	}

	// 不存在的分类返回 nil
	if srv.homeCategoryItems(ctx, "article", "zh", "not-exist", 10) != nil {
		t.Errorf("不存在分类应返回 nil")
	}
}
```

> 说明：`homeCategoryItems` 是 `package server` 内未导出方法，frontend_test.go 同包可直接调用。

- [ ] **步骤 2：运行测试验证失败**

运行：`go test -count=1 ./internal/server/ -run TestHomeCategoryItems -v`
预期：FAIL（编译错误：`srv.homeCategoryItems` 未定义）。

- [ ] **步骤 3：实现 renderHome 聚合**

`internal/server/frontend.go` `renderHome` 在 `data.Meta = s.seo.BuildHome(lang)` 之前插入：

```go
	// 首页产品/新闻分区：按配置的分类 slug 聚合内容（含子分类），未配置/不存在则留空回退
	if slug := s.cfg.Site.HomeProductsCategory; slug != "" {
		if items := s.homeCategoryItems(c.Request.Context(), "article", lang, slug, 6); items != nil {
			data.Products = items
		}
	}
	if slug := s.cfg.Site.HomeNewsCategory; slug != "" {
		if items := s.homeCategoryItems(c.Request.Context(), "article", lang, slug, 3); items != nil {
			data.News = items
		}
	}
```

在 `renderHome` 之后新增辅助方法：

```go
// homeCategoryItems 返回某分类（含子孙分类）下已发布内容，分类不存在返回 nil。
func (s *Server) homeCategoryItems(ctx context.Context, typeName, lang, slug string, limit int) []content.Entry {
	cat, err := s.store.CategoryRepo().GetBySlug(ctx, slug)
	if err != nil {
		return nil
	}
	ids := []int64{cat.ID}
	if ds, err := s.store.CategoryRepo().Descendants(ctx, cat.ID); err == nil {
		for _, d := range ds {
			ids = append(ids, d.ID)
		}
	}
	items, _, err := s.content.ListPublishedByCategories(ctx, typeName, lang, ids, 1, limit)
	if err != nil {
		return nil
	}
	return items
}
```

`context` 已在 `frontend.go` 导入（第 4 行），`content` 已导入（第 14 行）。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test -count=1 ./internal/server/ -run TestHomeCategoryItems -v`
预期：PASS。同时跑 `go test -count=1 ./internal/server/ -run TestFrontendRoutes -v` 确认现有首页测试仍绿（未配置 slug 时行为不变）。

---

### 任务 4：主题模板 index.html 换数据源

**文件：**
- 修改：`themes/default/templates/index.html`

产品中心/新闻动态数据源改为 `.Products`/`.News`，空则回退 `.Items`。样式与结构零改动。

- [ ] **步骤 1：修改 index.html**

`themes/default/templates/index.html` 产品中心段（现为 `{{$n := 6}}{{if lt (len .Items) $n}}{{$n = len .Items}}{{end}}{{range slice .Items 0 $n}}`）改为：

```html
{{$prod := .Products}}{{if not $prod}}{{$prod = .Items}}{{end}}
{{$n := 6}}{{if lt (len $prod) $n}}{{$n = len $prod}}{{end}}{{range slice $prod 0 $n}}
```

新闻动态段（现为 `{{$n := 3}}{{if lt (len .Items) $n}}{{$n = len .Items}}{{end}}{{range slice .Items 0 $n}}`）改为：

```html
{{$news := .News}}{{if not $news}}{{$news = .Items}}{{end}}
{{$n := 3}}{{if lt (len $news) $n}}{{$n = len $news}}{{end}}{{range slice $news 0 $n}}
```

> 说明：`{{if not $prod}}{{$prod = .Items}}{{end}}` 在 `$prod` 为 nil/空切片时回退到 `.Items`。`slice $prod 0 $n` 的 `$n` 钳制沿用既有逻辑。

- [ ] **步骤 2：验证模板可解析**

运行：
```powershell
go test -count=1 ./internal/theme/... -v
go test -count=1 ./themes/... -v
```
预期：PASS（模板解析失败会 FAIL）。

---

### 任务 5：全量验证与冒烟

**文件：**（无代码改动）

- [ ] **步骤 1：Go 全量验证**

运行：
```powershell
$env:Path = "C:\Program Files\Go\bin;" + $env:Path
go test -count=1 ./...
go vet ./...
gofmt -l .
```
预期：全绿、vet 干净、gofmt 无输出。

- [ ] **步骤 2：更新 config.yaml 并重启冒烟**

`config.yaml` 的 `site:` 段追加（保留现有 `debug: true`）：
```yaml
  home_products_category: "products"
  home_news_category: "news"
```
确认 8080 无残留进程后重启（`go run ./cmd/dulizhan` 或构建二进制后台运行）。

- [ ] **步骤 3：首页分区冒烟**

- `/` 首页：产品中心只含产品分类内容（products-1/products-2/chopper-1/sausage-machine-1/mixer-1，最多 6 条），**不含** about-1/news-1
- 新闻动态只含新闻分类内容（news-1/news-2/hello-zh，最多 3 条）
- `/en/` 英文版同样正确
- 样式无变化（CSS 文件未动）

预期：全部正确。

- [ ] **步骤 4：回退验证**

临时把 `config.yaml` 两个 slug 注释掉，重启，确认首页回退为 article 全量（行为与改造前一致）。恢复配置。

---

## 自检记录

**规格覆盖度：**
- §3.1 config → 任务 1；theme.Data → 任务 2；renderHome → 任务 3 ✔
- §3.2 index.html → 任务 4 ✔
- §3.3 config.example.yaml → 任务 1 步骤 5 ✔
- §5 验证 → 任务 5 ✔

**占位符扫描：** 无 TODO/待定。所有代码块完整可照抄。

**类型一致性：**
- `cfg.Site.HomeProductsCategory`/`HomeNewsCategory`（任务 1 定义）在任务 3 的 `renderHome` 使用，签名一致。
- `homeCategoryItems(ctx, typeName, lang, slug, limit)` 返回 `[]content.Entry`，赋值给 `data.Products`/`data.News`（任务 2 定义，`[]content.Entry`）一致。
- `CategoryRepo.GetBySlug(ctx, slug) (store.Category, error)`、`Descendants(ctx, id) ([]store.Category, error)`、`ListPublishedByCategories(ctx, typeName, lang, ids, page, perPage) ([]content.Entry, int, error)` 与 `internal/store/store.go:92,98`、`internal/content/service.go:245` 一致。
- 模板 `$prod`/`$news` 变量名在任务 4 内各自独立声明，不冲突。
- 测试断言的产品分类 slug（products-1/products-2/chopper-1/sausage-machine-1/mixer-1）与 `internal/seed/seed.go:135-147` 演示数据一致；news 分类（news-1/news-2/hello-zh）与 seed.go:128-131、182 一致。
