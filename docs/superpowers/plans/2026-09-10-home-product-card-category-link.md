# 首页产品卡片链接到所属分类归档页 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标：** 首页「产品中心」每张产品卡片的链接改为该产品所属分类的归档页（`/category/products/<slug...>`），无分类时回退详情页。

**架构：** theme 包新增可注入的分类 URL 解析器并注册模板函数 `entryCategoryURL`；server 包实现解析（复用现有分类链逻辑）并在 `New` 时注入；首页模板改用该函数。不改 `content.Entry` / `theme.Data` 契约。

**技术栈：** Go 1.26、Gin、html/template；测试用 `go test`（服务端用 httptest + 内存 SQLite + seed 数据）。

**仓库约定：** 本仓库不执行 `git add/commit`（AGENTS.md），故本计划省略提交步骤；每个任务以测试 + `gofmt`/`vet` 收尾。

**关联规格：** `docs/superpowers/specs/2026-09-10-home-product-card-category-link-design.md`

---

### 任务 1：theme 包 — 注入点 + `entryCategoryURL` 模板函数

**文件：**
- 修改：`internal/theme/theme.go`（`Theme` 结构体 ~88、`Loader` 结构体 ~103、`NewLoader`/`Load` ~111-198）
- 修改：`internal/theme/funcs.go`（`buildFuncs` ~12-26）
- 测试：`internal/theme/theme_test.go`（fixture 模板第 41 行、新增用例）

- [ ] **步骤 1：修改 fixture 模板 + 编写失败测试**

把 `internal/theme/theme_test.go` 中 `writeFixtureTheme` 的 index 模板第 41 行由 `{{url $.Lang .}}` 改为 `{{entryCategoryURL $.Lang .}}`：

```go
		"templates/index.html": `{{define "head"}}{{end}}
{{define "body"}}
<header>{{template "nav" .}}</header>
<main><h1>{{.Site.Name}}</h1>
<ul>{{range .Items}}<li><a href="{{entryCategoryURL $.Lang .}}">{{.Content.Title}}</a></li>{{end}}</ul>
</main>
{{end}}`,
```

在文件末尾追加新用例：

```go
func TestEntryCategoryURLUsesResolver(t *testing.T) {
	reg, err := i18n.New([]string{"zh", "en"}, "zh", false)
	if err != nil {
		t.Fatal(err)
	}
	loader := NewLoader(writeFixtureTheme(t), reg)
	loader.SetEntryCategoryURL(func(lang string, e content.Entry) string {
		return "/category/products/" + e.Content.Slug
	})
	th, err := loader.Load("fixture")
	if err != nil {
		t.Fatal(err)
	}
	buf := &bytes.Buffer{}
	entry := content.Entry{TypeName: "article", Content: store.Content{Slug: "chopper-1", Title: "斩拌机"}}
	if err := th.Render(buf, "index", &Data{
		Site: SiteInfo{Name: "x"}, Lang: "zh", Langs: reg.All(),
		Items: []content.Entry{entry}, Meta: seo.Meta{Title: "x"},
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `href="/category/products/chopper-1"`) {
		t.Errorf("应使用注入的分类 URL: %s", buf.String())
	}
}
```

- [ ] **步骤 2：运行测试验证它失败**

运行：`go test ./internal/theme/ -run TestEntryCategoryURLUsesResolver -v`
预期：FAIL（`SetEntryCategoryURL` 未定义 / 模板解析 `entryCategoryURL` 函数未注册）

- [ ] **步骤 3：实现注入点与模板函数**

`internal/theme/theme.go` — `Theme` 结构体加字段（放在 `loadedAt` 附近）：

```go
	loadedAt  time.Time
	// entryCatURL 由 server 注入，把 entry 解析为其分类归档 URL；nil 时模板回退详情页。
	entryCatURL func(lang string, e content.Entry) string
```

`Loader` 结构体加字段：

```go
type Loader struct {
	ThemesDir   string
	reg         *i18n.Registry
	cache       map[string]*Theme
	mu          sync.Mutex
	debug       bool
	entryCatURL func(lang string, e content.Entry) string
}
```

在 `NewLoader` 之后新增 setter：

```go
// SetEntryCategoryURL 注入分类归档 URL 解析器，之后 Load 的主题共享该解析器。
func (l *Loader) SetEntryCategoryURL(fn func(lang string, e content.Entry) string) {
	l.mu.Lock()
	l.entryCatURL = fn
	l.mu.Unlock()
}
```

`Load` 中，在 `th.reg = l.reg` 之前读取并赋值：

```go
	l.mu.Lock()
	th.entryCatURL = l.entryCatURL
	l.mu.Unlock()
	th.reg = l.reg
	th.funcs = th.buildFuncs()
```

`internal/theme/funcs.go` — `buildFuncs` 的 `template.FuncMap` 中，在 `"asset"` 之后新增：

```go
		"entryCategoryURL": func(lang string, e content.Entry) string {
			if t.entryCatURL != nil {
				if u := t.entryCatURL(lang, e); u != "" {
					return u
				}
			}
			return t.urlPath(lang, "/"+e.TypeName+"/"+e.Content.Slug)
		},
```

- [ ] **步骤 4：运行测试验证它通过**

运行：`go test ./internal/theme/ -v`
预期：PASS（新用例 + 既有 `TestRenderIndex`/`TestRenderIndexEnPrefix` 走回退仍通过）

- [ ] **步骤 5：格式化与静态检查**

运行：`gofmt -l internal/theme/` 与 `go vet ./internal/theme/`
预期：无输出

---

### 任务 2：server 包 — 解析实现 + 注入 + 首页模板改链接

**文件：**
- 修改：`internal/server/frontend.go`（`renderSingle` ~222-256，新增两个方法）
- 修改：`internal/server/server.go`（`New` ~66 前）
- 修改：`internal/server/frontend_test.go`（新增用例）
- 修改：`themes/default/templates/index.html`（第 22 行）

- [ ] **步骤 1：编写失败测试**

在 `internal/server/frontend_test.go` 末尾追加：

```go
func TestHomeProductCardsLinkToCategory(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	srv.cfg.Site.Theme = "default"
	code, body := get(t, srv, "/")
	if code != http.StatusOK {
		t.Fatalf("首页 = %d", code)
	}
	if !strings.Contains(body, `href="/category/products/chopper"`) {
		t.Errorf("首页产品卡片应链接到分类归档 /category/products/chopper: %s", body)
	}
}
```

- [ ] **步骤 2：运行测试验证它失败**

运行：`go test ./internal/server/ -run TestHomeProductCardsLinkToCategory -v`
预期：FAIL（首页卡片当前为 `href="/article/chopper-1"`）

- [ ] **步骤 3：实现解析、注入与模板改动**

`internal/server/frontend.go` — 在 `renderSingle` 之后新增两个方法：

```go
// entryCategoryChain 解析 entry 的 category 字段为分类链（自顶向下）；无分类/解析失败返回 nil。
func (s *Server) entryCategoryChain(ctx context.Context, e content.Entry) []store.Category {
	catIDStr, ok := e.Fields["category"].(string)
	if !ok || catIDStr == "" {
		return nil
	}
	id, err := strconv.ParseInt(catIDStr, 10, 64)
	if err != nil {
		return nil
	}
	cat, err := s.store.CategoryRepo().GetByID(ctx, id)
	if err != nil {
		return nil
	}
	return s.categoryChainInfo(ctx, cat.ID)
}

// entryCategoryURL 解析 entry 所属分类的归档 URL；无分类/解析失败返回空串。
// 模板函数无请求上下文，此处用 context.Background()（本地 DB 读，无取消需求）。
func (s *Server) entryCategoryURL(lang string, e content.Entry) string {
	chain := s.entryCategoryChain(context.Background(), e)
	if chain == nil {
		return ""
	}
	slugs := make([]string, len(chain))
	for i, c := range chain {
		slugs[i] = c.Slug
	}
	return s.categoryURL(lang, slugs)
}
```

把 `renderSingle` 中原有的分类解析块（从 `var crumbs []seo.Breadcrumb` 到 `}` 结束的 `if catIDStr ...` 整块）替换为：

```go
	var crumbs []seo.Breadcrumb
	isProduct := false
	if chain := s.entryCategoryChain(c.Request.Context(), e); chain != nil {
		cat := chain[len(chain)-1]
		data.EntryCategory = s.catName(cat, lang)
		slugs := make([]string, len(chain))
		for i, c := range chain {
			slugs[i] = c.Slug
		}
		data.EntryCategoryURL = s.categoryURL(lang, slugs)
		crumbs = s.entryBreadcrumbs(lang, chain, slugs, e.TypeName, e.Content.Slug, e.Content.Title)
		isProduct = len(slugs) > 0 && slugs[0] == s.cfg.Site.HomeProductsCategory // 顶级分类为产品分类即视为产品页
	}
```

`internal/server/server.go` — `New` 中，在 `s := &Server{...}` 之后、`eng := gin.New()` 之前注入：

```go
	loader.SetEntryCategoryURL(s.entryCategoryURL)
```

`themes/default/templates/index.html` 第 22 行：

```html
      <a class="card" href="{{entryCategoryURL $.Lang .}}">
```

（新闻卡片第 39 行保持 `{{url $.Lang .}}` 不动。）

- [ ] **步骤 4：运行测试验证它通过**

运行：`go test ./internal/server/ -v`
预期：PASS（新用例 + 既有分类/详情/首页用例全绿）

- [ ] **步骤 5：格式化与静态检查**

运行：`gofmt -l internal/server/` 与 `go vet ./internal/server/`
预期：无输出

---

### 任务 3：全量验证与冒烟

**文件：** 无（仅验证）

- [ ] **步骤 1：全量测试与构建**

运行：`go test -count=1 ./...`、`go build ./...`、`go vet ./...`、`gofmt -l .`
预期：测试全绿；build/vet 无输出；gofmt 无输出

- [ ] **步骤 2：重编译并重启服务**

运行：
```
Get-Process dulizhan -ErrorAction SilentlyContinue | Stop-Process -Force
go build -o dulizhan.exe ./cmd/dulizhan
Start-Process -FilePath ".\dulizhan.exe" -WorkingDirectory (Get-Location) -RedirectStandardOutput "server.out.log" -RedirectStandardError "server.err.log" -WindowStyle Hidden
```
预期：进程启动、8080 监听

- [ ] **步骤 3：冒烟验证中英首页链接**

运行：
```
(Invoke-WebRequest "http://localhost:8080/en/" -UseBasicParsing).Content -split "`n" | Select-String 'class="card"'
(Invoke-WebRequest "http://localhost:8080/zh/" -UseBasicParsing).Content -split "`n" | Select-String 'class="card"'
```
预期：产品卡片 `href` 指向 `/en/category/products/chopper`、`/en/category/products/sausage-machine`、`/en/category/products/mixer`、`/en/category/products`（zh 同理无前缀或 `/zh/` 前缀）

- [ ] **步骤 4：验证跳转目标可达**

运行：
```
(Invoke-WebRequest "http://localhost:8080/en/category/products/chopper" -UseBasicParsing).StatusCode
```
预期：200
