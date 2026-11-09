# SEO 审计修复实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标：** 修复 SEO 审计问题：分类页 canonical 404、og:image/twitter:card、按语言 description/品牌名、首页 title 关键词、JSON-LD（Product/Organization/CollectionPage/Breadcrumb）、sitemap 补分类页与首页、favicon、首图 alt、尾斜杠 301、视频懒加载、测试内容清理。

**架构：** 集中改 `internal/seo`（Builder 参数化 + BuildCategory/Entry 重构）、`internal/server/frontend.go`（接线 + 尾斜杠 301 + sitemap）、`internal/config`（Names/Descriptions/HomeTitles/OGImage）、`base.html` 与主题模板（favicon/og/twitter/alt/hero）、主题静态资源（og-default.png/favicon.png）、admin 富文本与主题 JS（懒加载）。

**技术栈：** Go + Gin + html/template + yaml；Vue3 + wangEditor；PowerShell System.Drawing（生成 PNG）。

**约定：** 仓库不 git 提交（AGENTS.md），各任务无 commit 步骤。改 Go 后跑 `go test -count=1 ./...`；改 SPA 后跑 `npx vue-tsc --noEmit && npx vitest run` + `.\build.ps1`。

**规格：** `docs/superpowers/specs/2026-08-20-seo-audit-fixes-design.md`

---

### 任务 1：config 增 Names/Descriptions/HomeTitles/OGImage 与按语言取值

**文件：**
- 修改：`internal/config/config.go`
- 测试：`internal/config/config_test.go`

- [ ] **步骤 1：写失败测试**

在 `internal/config/config_test.go` 末尾追加：

```go
func TestSiteLocalized(t *testing.T) {
	cfg := &Config{}
	cfg.Site.DefaultLang = "zh"
	cfg.Site.Name = "金博威机械"
	cfg.Site.Description = "中文默认描述"
	cfg.Site.Names = map[string]string{"en": "Jinbowei Machinery"}
	cfg.Site.Descriptions = map[string]string{"en": "English description"}
	cfg.Site.HomeTitles = map[string]string{"zh": "金博威机械 - 关键词首页", "en": "Jinbowei - Keywords"}
	cfg.Site.OGImage = "/themes/default/img/og-default.png"

	if got := cfg.Site.SiteName("en"); got != "Jinbowei Machinery" {
		t.Errorf("SiteName(en) = %q", got)
	}
	if got := cfg.Site.SiteName("ja"); got != "金博威机械" {
		t.Errorf("SiteName(ja) 应回退默认 = %q", got)
	}
	if got := cfg.Site.SiteDescription("en"); got != "English description" {
		t.Errorf("SiteDescription(en) = %q", got)
	}
	if got := cfg.Site.SiteDescription("ja"); got != "中文默认描述" {
		t.Errorf("SiteDescription(ja) 应回退默认 = %q", got)
	}
	if got := cfg.Site.HomeTitle("zh"); got != "金博威机械 - 关键词首页" {
		t.Errorf("HomeTitle(zh) = %q", got)
	}
	if got := cfg.Site.HomeTitle("en"); got != "Jinbowei - Keywords" {
		t.Errorf("HomeTitle(en) = %q", got)
	}
	if cfg.Site.OGImage != "/themes/default/img/og-default.png" {
		t.Errorf("OGImage = %q", cfg.Site.OGImage)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test -count=1 ./internal/config/ -run TestSiteLocalized -v`
预期：FAIL，`cfg.Site.SiteName undefined`。

- [ ] **步骤 3：实现**

`internal/config/config.go` 的 `SiteConfig` 结构体增加字段并新增方法：

```go
type SiteConfig struct {
	Name          string   `yaml:"name"`
	URL           string   `yaml:"url"`
	DefaultLang   string   `yaml:"default_lang"`
	Languages     []string `yaml:"languages"`
	Theme         string   `yaml:"theme"`
	ThemesDir     string   `yaml:"themes_dir"`
	PrefixDefault bool     `yaml:"prefix_default_lang"`
	Description   string   `yaml:"description"`
	OGImage       string   `yaml:"og_image"`
	Names         map[string]string `yaml:"names"`
	Descriptions  map[string]string `yaml:"descriptions"`
	HomeTitles    map[string]string `yaml:"home_titles"`

	HomeProductsCategory string `yaml:"home_products_category"`
	HomeNewsCategory     string `yaml:"home_news_category"`
}

// SiteName 按语言取品牌名，缺失回退 Name。
func (s *SiteConfig) SiteName(lang string) string {
	if s.Names != nil {
		if v, ok := s.Names[lang]; ok && v != "" {
			return v
		}
	}
	return s.Name
}

// SiteDescription 按语言取 meta description，缺失回退 Description。
func (s *SiteConfig) SiteDescription(lang string) string {
	if s.Descriptions != nil {
		if v, ok := s.Descriptions[lang]; ok && v != "" {
			return v
		}
	}
	return s.Description
}

// HomeTitle 按语言取首页 title（含关键词），缺失回退 SiteName。
func (s *SiteConfig) HomeTitle(lang string) string {
	if s.HomeTitles != nil {
		if v, ok := s.HomeTitles[lang]; ok && v != "" {
			return v
		}
	}
	return s.SiteName(lang)
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test -count=1 ./internal/config/`
预期：PASS。

---

### 任务 2：seo.Builder 参数化 + Meta.Image + BuildHome/List 按语言

**文件：**
- 修改：`internal/seo/seo.go`
- 测试：`internal/seo/seo_test.go`

- [ ] **步骤 1：写失败测试**

把 `internal/seo/seo_test.go` 的 `testBuilder` 改为：

```go
func testBuilder(t *testing.T) *Builder {
	t.Helper()
	reg, err := i18n.New([]string{"zh", "en"}, "zh", false)
	if err != nil {
		t.Fatal(err)
	}
	return NewBuilder(Options{
		SiteURL:         "https://example.com",
		Names:           map[string]string{"zh": "测试站点", "en": "Test Site"},
		Descriptions:    map[string]string{"zh": "站点描述", "en": "Site description"},
		HomeTitles:      map[string]string{"zh": "测试站点 - 关键词首页", "en": "Test Site - Keyword Home"},
		DefaultLang:     "zh",
		OGImage:         "/themes/default/img/og-default.png",
		ProductCategory: "products",
	}, reg)
}
```

追加测试：

```go
func TestBuildHomeLocalized(t *testing.T) {
	b := testBuilder(t)
	zh := b.BuildHome("zh")
	if zh.Title != "测试站点 - 关键词首页" {
		t.Errorf("home zh title = %q", zh.Title)
	}
	if zh.Description != "站点描述" {
		t.Errorf("home zh desc = %q", zh.Description)
	}
	en := b.BuildHome("en")
	if en.Title != "Test Site - Keyword Home" {
		t.Errorf("home en title = %q", en.Title)
	}
	if en.Description != "Site description" {
		t.Errorf("home en desc = %q", en.Description)
	}
	if en.Image != "https://example.com/themes/default/img/og-default.png" {
		t.Errorf("home og image = %q", en.Image)
	}
	js := string(en.JSONLDScript)
	if !strings.Contains(js, "Organization") || !strings.Contains(js, "WebSite") {
		t.Errorf("home JSON-LD 缺 Organization/WebSite: %s", js)
	}
	if !strings.Contains(js, "og-default.png") {
		t.Errorf("Organization.logo 缺失: %s", js)
	}
}

func TestBuildListLocalized(t *testing.T) {
	b := testBuilder(t)
	m := b.BuildList("en", "article", 1)
	if m.Description != "Site description" {
		t.Errorf("list desc = %q", m.Description)
	}
	if !strings.HasSuffix(m.Title, "Test Site") {
		t.Errorf("list title = %q", m.Title)
	}
	if m.Image != "https://example.com/themes/default/img/og-default.png" {
		t.Errorf("list og image = %q", m.Image)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test -count=1 ./internal/seo/ -run 'TestBuildHomeLocalized|TestBuildListLocalized' -v`
预期：FAIL（编译错：`Options undefined` / `NewBuilder` 参数不匹配）。

- [ ] **步骤 3：实现**

整体重写 `internal/seo/seo.go` 的 Builder 声明与 BuildHome/BuildList，替换 `BuildEntry`：

```go
package seo

import (
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"

	"dulizhan/internal/content"
	"dulizhan/internal/i18n"
)

type HrefLang struct {
	Lang string
	URL  string
}

type Meta struct {
	Title        string
	Description  string
	Canonical    string
	Image        string
	OGTags       map[string]string
	JSONLDScript template.HTML
	HrefLangs    []HrefLang
}

// Breadcrumb 面包屑条目（Name 展示名，URL 绝对地址）。
type Breadcrumb struct {
	Name string
	URL  string
}

type Options struct {
	SiteURL         string
	Names           map[string]string
	Descriptions    map[string]string
	HomeTitles      map[string]string
	DefaultLang     string
	OGImage         string
	ProductCategory string
}

type Builder struct {
	siteURL         string
	names           map[string]string
	descriptions    map[string]string
	homeTitles      map[string]string
	defaultLang     string
	ogImage         string
	productCategory string
	reg             *i18n.Registry
}

func NewBuilder(opts Options, reg *i18n.Registry) *Builder {
	return &Builder{
		siteURL:         strings.TrimRight(opts.SiteURL, "/"),
		names:           opts.Names,
		descriptions:    opts.Descriptions,
		homeTitles:      opts.HomeTitles,
		defaultLang:     opts.DefaultLang,
		ogImage:         opts.OGImage,
		productCategory: opts.ProductCategory,
		reg:             reg,
	}
}

func jsonLDScript(ld []byte) template.HTML {
	return template.HTML(`<script type="application/ld+json">` + string(ld) + `</script>`)
}

func (b *Builder) name(lang string) string {
	if v, ok := b.names[lang]; ok && v != "" {
		return v
	}
	if v, ok := b.names[b.defaultLang]; ok && v != "" {
		return v
	}
	return ""
}

func (b *Builder) description(lang string) string {
	if v, ok := b.descriptions[lang]; ok && v != "" {
		return v
	}
	if v, ok := b.descriptions[b.defaultLang]; ok && v != "" {
		return v
	}
	return ""
}

func (b *Builder) homeTitle(lang string) string {
	if v, ok := b.homeTitles[lang]; ok && v != "" {
		return v
	}
	if v, ok := b.homeTitles[b.defaultLang]; ok && v != "" {
		return v
	}
	return b.name(lang)
}

// defaultImage 返回绝对 og 默认图；未配置返回空串。
func (b *Builder) defaultImage() string {
	if b.ogImage == "" {
		return ""
	}
	if strings.HasPrefix(b.ogImage, "http://") || strings.HasPrefix(b.ogImage, "https://") {
		return b.ogImage
	}
	return b.siteURL + b.ogImage
}

func (b *Builder) pageTitle(prefix, lang string) string {
	n := b.name(lang)
	if n == "" {
		return prefix
	}
	return prefix + " - " + n
}

func (b *Builder) BuildEntry(lang string, e content.Entry, crumbs []Breadcrumb, isProduct bool) Meta {
	title := e.Content.Title
	if title == "" {
		title = b.pageTitle(b.name(lang), lang)
	} else {
		title = b.pageTitle(title, lang)
	}
	desc, _ := e.Fields["excerpt"].(string)
	if desc == "" {
		desc = b.description(lang)
	}
	img := mediaURL(e.Fields["cover"])
	if img == "" {
		img = b.defaultImage()
	}
	path := "/" + e.TypeName + "/" + e.Content.Slug
	canonical := b.siteURL + b.reg.URLPath(lang, path)

	graph := []map[string]any{}
	if isProduct {
		p := map[string]any{
			"@type":            "Product",
			"name":             e.Content.Title,
			"description":      desc,
			"mainEntityOfPage": canonical,
			"brand":            map[string]any{"@type": "Brand", "name": b.name(lang)},
		}
		if img != "" {
			p["image"] = img
		}
		graph = append(graph, p)
	} else {
		graph = append(graph, map[string]any{
			"@type":            "Article",
			"headline":         e.Content.Title,
			"datePublished":    fmtTime(e.Content.PublishedAt),
			"dateModified":     e.Content.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
			"mainEntityOfPage": canonical,
		})
	}
	if len(crumbs) > 0 {
		els := make([]map[string]any, 0, len(crumbs))
		for i, cr := range crumbs {
			els = append(els, map[string]any{"@type": "ListItem", "position": i + 1, "name": cr.Name, "item": cr.URL})
		}
		graph = append(graph, map[string]any{"@type": "BreadcrumbList", "itemListElement": els})
	}
	ld, err := json.Marshal(map[string]any{"@context": "https://schema.org", "@graph": graph})
	if err != nil {
		ld = nil
	}
	og := map[string]string{
		"title": title, "description": desc, "type": "article",
		"url": canonical, "site_name": b.name(lang),
	}
	if img != "" {
		og["image"] = img
	}
	m := Meta{
		Title: title, Description: desc, Canonical: canonical,
		HrefLangs: b.hrefLangs(lang, path), Image: img, OGTags: og,
	}
	if len(ld) > 0 {
		m.JSONLDScript = jsonLDScript(ld)
	}
	return m
}

func (b *Builder) BuildList(lang, typeName string, page int) Meta {
	title := b.pageTitle(typeName, lang)
	desc := b.description(lang)
	img := b.defaultImage()
	path := "/" + typeName
	if page > 1 {
		path = fmt.Sprintf("/%s/page/%d", typeName, page)
	}
	canonical := b.siteURL + b.reg.URLPath(lang, path)
	og := map[string]string{
		"title": title, "description": desc, "type": "website",
		"url": canonical, "site_name": b.name(lang),
	}
	if img != "" {
		og["image"] = img
	}
	return Meta{
		Title: title, Description: desc, Canonical: canonical,
		HrefLangs: b.hrefLangs(lang, "/"+typeName), Image: img, OGTags: og,
	}
}

// BuildCategory 分类归档页 meta：canonical 用 /category/<slug 链>。
func (b *Builder) BuildCategory(lang string, chain []string, catName string, items []content.Entry) Meta {
	title := b.pageTitle(catName, lang)
	desc := b.description(lang)
	img := b.defaultImage()
	path := "/category/" + strings.Join(chain, "/")
	canonical := b.siteURL + b.reg.URLPath(lang, path)

	els := make([]map[string]any, 0)
	n := len(items)
	if n > 10 {
		n = 10
	}
	for i := 0; i < n; i++ {
		it := items[i]
		u := b.siteURL + b.reg.URLPath(lang, "/"+it.TypeName+"/"+it.Content.Slug)
		els = append(els, map[string]any{"@type": "ListItem", "position": i + 1, "name": it.Content.Title, "url": u})
	}
	ld, err := json.Marshal(map[string]any{
		"@context":   "https://schema.org",
		"@type":      "CollectionPage",
		"name":       catName,
		"mainEntity": map[string]any{"@type": "ItemList", "itemListElement": els},
	})
	if err != nil {
		ld = nil
	}
	og := map[string]string{
		"title": title, "description": desc, "type": "website",
		"url": canonical, "site_name": b.name(lang),
	}
	if img != "" {
		og["image"] = img
	}
	m := Meta{
		Title: title, Description: desc, Canonical: canonical,
		HrefLangs: b.hrefLangs(lang, path), Image: img, OGTags: og,
	}
	if len(ld) > 0 {
		m.JSONLDScript = jsonLDScript(ld)
	}
	return m
}

func (b *Builder) BuildHome(lang string) Meta {
	title := b.homeTitle(lang)
	desc := b.description(lang)
	img := b.defaultImage()
	path := "/"
	canonical := b.siteURL + b.reg.URLPath(lang, path)

	graph := []map[string]any{
		{"@type": "WebSite", "name": b.name(lang), "url": canonical},
		{"@type": "Organization", "name": b.name(lang), "url": canonical},
	}
	if img != "" {
		graph[1]["logo"] = img
	}
	ld, err := json.Marshal(map[string]any{"@context": "https://schema.org", "@graph": graph})
	if err != nil {
		ld = nil
	}
	og := map[string]string{
		"title": title, "description": desc, "type": "website",
		"url": canonical, "site_name": b.name(lang),
	}
	if img != "" {
		og["image"] = img
	}
	m := Meta{
		Title: title, Description: desc, Canonical: canonical,
		HrefLangs: b.hrefLangs(lang, "/"), Image: img, OGTags: og,
	}
	if len(ld) > 0 {
		m.JSONLDScript = jsonLDScript(ld)
	}
	return m
}

func (b *Builder) hrefLangs(lang, path string) []HrefLang {
	out := make([]HrefLang, 0, len(b.reg.All())+1)
	out = append(out, HrefLang{Lang: "x-default", URL: b.siteURL + b.reg.URLPath(b.reg.Default(), path)})
	for _, l := range b.reg.All() {
		out = append(out, HrefLang{Lang: l.Code, URL: b.siteURL + b.reg.URLPath(l.Code, path)})
	}
	return out
}

// mediaURL 复用主题 mediaFunc 规则：非空且以 http(s) 或 / 开头的字符串才可用。
func mediaURL(v any) string {
	switch s := v.(type) {
	case string:
		if s != "" && (strings.HasPrefix(s, "http") || strings.HasPrefix(s, "/")) {
			return s
		}
	}
	return ""
}

func fmtTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("2006-01-02T15:04:05Z")
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test -count=1 ./internal/seo/`
预期：PASS（其余既有测试同时更新到新签名后应通过；见任务 2 步骤 5）。

- [ ] **步骤 5：更新既有 seo_test 调新签名**

`internal/seo/seo_test.go` 中所有 `b.BuildEntry("zh", e)` / `b.BuildEntry("en", e)` 改为：

```go
m := b.BuildEntry("zh", e, nil, false)
m := b.BuildEntry("en", e, nil, false)
```

新增 Product/Breadcrumb/OG 覆盖测试（追加到文件末尾）：

```go
func TestBuildEntryProductAndBreadcrumb(t *testing.T) {
	b := testBuilder(t)
	now := time.Now().UTC()
	e := content.Entry{
		TypeName: "article",
		Content:  store.Content{Slug: "chopper-1", Title: "斩拌机", Status: "published", PublishedAt: &now},
		Fields:   map[string]any{"cover": "/media/chopper.jpg", "excerpt": "高效斩拌"},
	}
	crumbs := []Breadcrumb{
		{Name: "首页", URL: "https://example.com/"},
		{Name: "产品中心", URL: "https://example.com/category/products"},
		{Name: "斩拌机", URL: "https://example.com/category/products/chopper"},
	}
	m := b.BuildEntry("zh", e, crumbs, true)
	js := string(m.JSONLDScript)
	if !strings.Contains(js, `"@type":"Product"`) {
		t.Errorf("缺 Product schema: %s", js)
	}
	if strings.Contains(js, `"Article"`) {
		t.Errorf("产品页不应是 Article: %s", js)
	}
	if !strings.Contains(js, `"BreadcrumbList"`) {
		t.Errorf("缺 BreadcrumbList: %s", js)
	}
	if !strings.Contains(js, `/category/products/chopper`) {
		t.Errorf("面包屑缺分类 URL: %s", js)
	}
	if m.Image != "https://example.com/media/chopper.jpg" {
		t.Errorf("og image 应取 cover: %q", m.Image)
	}
	if m.OGTags["image"] != "https://example.com/media/chopper.jpg" {
		t.Errorf("og:image tag = %q", m.OGTags["image"])
	}
}

func TestBuildEntryOGImageFallback(t *testing.T) {
	b := testBuilder(t)
	now := time.Now().UTC()
	e := content.Entry{TypeName: "article", Content: store.Content{Slug: "x", Title: "无图", PublishedAt: &now}}
	m := b.BuildEntry("zh", e, nil, false)
	want := "https://example.com/themes/default/img/og-default.png"
	if m.Image != want {
		t.Errorf("无 cover 应回落默认图: %q", m.Image)
	}
	if m.OGTags["image"] != want {
		t.Errorf("og:image 应回落默认: %q", m.OGTags["image"])
	}
}

func TestBuildCategoryCanonical(t *testing.T) {
	b := testBuilder(t)
	m := b.BuildCategory("zh", []string{"products", "chopper"}, "斩拌机", nil)
	if m.Canonical != "https://example.com/category/products/chopper" {
		t.Errorf("category canonical = %q", m.Canonical)
	}
	if strings.Contains(m.Canonical, "%") {
		t.Errorf("canonical 不应含 URL 编码: %q", m.Canonical)
	}
	js := string(m.JSONLDScript)
	if !strings.Contains(js, `"CollectionPage"`) || !strings.Contains(js, `"ItemList"`) {
		t.Errorf("category JSON-LD 缺 CollectionPage/ItemList: %s", js)
	}
	en := b.BuildCategory("en", []string{"products", "chopper"}, "Chopper", nil)
	if en.Canonical != "https://example.com/en/category/products/chopper" {
		t.Errorf("en category canonical = %q", en.Canonical)
	}
}
```

运行：`go test -count=1 ./internal/seo/`
预期：PASS。

---

### 任务 3：frontend.go 接线 + 尾斜杠 301 + sitemap

**文件：**
- 修改：`internal/server/frontend.go`
- 修改：`internal/server/server.go:49-56`
- 测试：`internal/server/frontend_test.go`

- [ ] **步骤 1：写失败测试**

`internal/server/frontend_test.go` 追加：

```go
func TestCategoryCanonicalUsesSlugChain(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	srv.cfg.Site.Theme = "default"
	ctx := context.Background()
	products, err := srv.store.CategoryRepo().GetBySlug(ctx, "products")
	if err != nil {
		t.Fatalf("products 分类: %v", err)
	}
	chopper, err := srv.store.CategoryRepo().GetBySlug(ctx, "chopper")
	if err != nil {
		t.Fatalf("chopper 分类: %v", err)
	}
	// 给 chopper 挂一篇已发布内容
	catID := strconv.FormatInt(chopper.ID, 10)
	_, err = srv.content.Create(ctx, "article", "zh", map[string]any{
		"title": "斩拌机Z", "slug": "chopper-z", "category": catID, "content": "<p>x</p>",
	}, true)
	if err != nil {
		t.Fatalf("创建内容: %v", err)
	}
	_ = products
	code, body := get(t, srv, "/category/products/chopper")
	if code != http.StatusOK {
		t.Fatalf("/category/products/chopper = %d", code)
	}
	if !strings.Contains(body, `rel="canonical" href="https://example.com/category/products/chopper"`) {
		t.Errorf("分类页 canonical 应指向 slug 链: %s", body)
	}
	if strings.Contains(body, "%e6%96%a9") {
		t.Errorf("canonical 出现 URL 编码分类名: %s", body)
	}
}

func TestTrailingSlashRedirectsToNoSlash(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	srv.cfg.Site.Theme = "default"
	req := httptest.NewRequest(http.MethodGet, "https://example.com/article/hello-zh/", nil)
	w := httptest.NewRecorder()
	srv.engine.ServeHTTP(w, req)
	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMovedPermanently {
		t.Fatalf("/article/hello-zh/ = %d, want 301", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/article/hello-zh" {
		t.Errorf("Location = %q, want /article/hello-zh", loc)
	}
	// 根路径不受影响
	req2 := httptest.NewRequest(http.MethodGet, "https://example.com/", nil)
	w2 := httptest.NewRecorder()
	srv.engine.ServeHTTP(w2, req2)
	if w2.Result().StatusCode != http.StatusOK {
		t.Errorf("根路径应保持 200: %d", w2.Result().StatusCode)
	}
}

func TestSitemapIncludesHomeAndCategories(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	srv.cfg.Site.Theme = "default"
	code, body := get(t, srv, "/sitemap.xml")
	if code != http.StatusOK {
		t.Fatalf("sitemap = %d", code)
	}
	for _, want := range []string{
		"https://example.com/",
		"https://example.com/category/products",
		"https://example.com/category/news",
		"https://example.com/article/hello-zh",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("sitemap 缺 %q", want)
		}
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test -count=1 ./internal/server/ -run 'TestCategoryCanonicalUsesSlugChain|TestTrailingSlashRedirectsToNoSlash|TestSitemapIncludesHomeAndCategories' -v`
预期：三个测试 FAIL（canonical 为 `/斩拌机` 404、无 301、sitemap 无分类/首页）。

- [ ] **步骤 3：实现**

`internal/server/server.go` 的 `New`（约 51 行）改为：

```go
		seo: seo.NewBuilder(seo.Options{
			SiteURL:      cfg.Site.URL,
			Names:        cfg.Site.Names,
			Descriptions: cfg.Site.Descriptions,
			HomeTitles:   cfg.Site.HomeTitles,
			DefaultLang:  cfg.Site.DefaultLang,
			OGImage:      cfg.Site.OGImage,
		}, reg),
```

`internal/server/frontend.go`：

(a) `handleFrontend` 在解析 lang 前插入尾斜杠 301（`renderList` 调用之前的函数体开头）：

```go
func (s *Server) handleFrontend(c *gin.Context) {
	p := c.Request.URL.Path
	if p != "/" && strings.HasSuffix(p, "/") {
		np := strings.TrimSuffix(p, "/")
		if np == "" {
			np = "/"
		}
		if c.Request.URL.RawQuery != "" {
			np += "?" + c.Request.URL.RawQuery
		}
		c.Redirect(http.StatusMovedPermanently, np)
		return
	}
	lang, segs := s.i18n.ResolvePath(p)
	...
```

(b) `data.Site` 构造改按语言（约 31-34 行）：

```go
	data := &theme.Data{
		Site: theme.SiteInfo{
			Name:        s.cfg.Site.SiteName(lang),
			URL:         s.cfg.Site.URL,
			Description: s.cfg.Site.SiteDescription(lang),
		},
		Lang:    lang,
		Langs:   s.i18n.All(),
		PerPage: defaultPerPage,
		Menus:   s.loadMenus(c.Request.Context(), lang),
	}
```

(c) `renderCategory` 的 meta（约 202 行）改为：

```go
	data.Meta = s.seo.BuildCategory(lang, pathSegs, cat.Name, all)
```

(d) `renderSingle` 重构（替换 219 行 `data.Meta = s.seo.BuildEntry(lang, e)` 及其后分类解析块）：

```go
	var crumbs []seo.Breadcrumb
	isProduct := false
	if catIDStr, ok := e.Fields["category"].(string); ok && catIDStr != "" {
		if id, err := strconv.ParseInt(catIDStr, 10, 64); err == nil {
			if cat, err := s.store.CategoryRepo().GetByID(c.Request.Context(), id); err == nil {
				data.EntryCategory = cat.Name
				chain := s.categoryChain(c.Request.Context(), cat.ID)
				data.EntryCategoryURL = s.categoryURL(lang, chain)
				crumbs = s.entryBreadcrumbs(c.Request.Context(), lang, cat.ID, e.TypeName, e.Content.Slug, e.Content.Title, chain)
				isProduct = len(chain) > 0 && chain[0] == s.cfg.Site.HomeProductsCategory
			}
		}
	}
	data.Meta = s.seo.BuildEntry(lang, e, crumbs, isProduct)
```

(e) 新增 `entryBreadcrumbs`（放在 `categoryURL` 之后）：

```go
// entryBreadcrumbs 构造详情页面包屑：首页 > 分类链 > 当前页（URL 为绝对地址）。
func (s *Server) entryBreadcrumbs(ctx context.Context, lang string, catID int64, typeName, slug, currentTitle string, chain []string) []seo.Breadcrumb {
	homeName := "Home"
	if lang == "zh" {
		homeName = "首页"
	}
	out := []seo.Breadcrumb{{Name: homeName, URL: s.cfg.Site.URL + s.i18n.URLPath(lang, "/")}}
	for i, slug := range chain {
		name := slug
		if cat, err := s.store.CategoryRepo().GetBySlug(ctx, slug); err == nil {
			name = cat.Name
		}
		out = append(out, seo.Breadcrumb{Name: name, URL: s.cfg.Site.URL + s.categoryURL(lang, chain[:i+1])})
	}
	out = append(out, seo.Breadcrumb{Name: currentTitle, URL: s.cfg.Site.URL + s.i18n.URLPath(lang, "/"+typeName+"/"+slug)})
	return out
}
```

(f) `handleSitemap` 在内容页循环前加入首页与分类页（在 `var entries` 之后插入）：

```go
	// 首页
	for _, l := range s.i18n.All() {
		entries = append(entries, seo.SitemapEntry{Loc: s.cfg.Site.URL + s.i18n.URLPath(l.Code, "/")})
	}
	// 全部分类页（含父/子，任意层级）
	cats, err := s.store.CategoryRepo().List(ctx)
	if err != nil {
		c.String(http.StatusInternalServerError, "查询分类失败: %v", err)
		return
	}
	for _, cat := range cats {
		chain := s.categoryChain(ctx, cat.ID)
		if chain == nil {
			continue
		}
		for _, l := range s.i18n.All() {
			entries = append(entries, seo.SitemapEntry{Loc: s.cfg.Site.URL + s.categoryURL(l.Code, chain)})
		}
	}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test -count=1 ./internal/server/ -run 'TestCategoryCanonicalUsesSlugChain|TestTrailingSlashRedirectsToNoSlash|TestSitemapIncludesHomeAndCategories' -v`
预期：PASS。

- [ ] **步骤 5：全量回归**

运行：`go test -count=1 ./...`
预期：PASS（若 `internal/server/frontend_test.go` 既有 canonical 断言被改，按新 canonical 值更新断言）。

---

### 任务 4：base.html（favicon/og/twitter）+ single.html alt + zh.yaml hero

**文件：**
- 修改：`internal/theme/base.html`
- 修改：`themes/default/templates/single.html`
- 修改：`themes/default/locales/zh.yaml`

- [ ] **步骤 1：实现**

`internal/theme/base.html` 的 `<head>` 内（`<title>` 之后）替换为：

```html
<title>{{.Meta.Title}}</title>
{{if .Meta.Description}}<meta name="description" content="{{.Meta.Description}}">{{end}}
{{range .Meta.HrefLangs}}<link rel="alternate" hreflang="{{.Lang}}" href="{{.URL}}">{{end}}
<link rel="canonical" href="{{.Meta.Canonical}}">
<link rel="icon" type="image/png" href="{{asset "/img/favicon.png"}}">
{{range $k, $v := .Meta.OGTags}}<meta property="og:{{$k}}" content="{{$v}}">{{end}}
<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:title" content="{{.Meta.Title}}">
<meta name="twitter:description" content="{{.Meta.Description}}">
{{if .Meta.Image}}<meta name="twitter:image" content="{{.Meta.Image}}">{{end}}
{{if .Meta.JSONLDScript}}{{.Meta.JSONLDScript}}{{end}}
```

`themes/default/templates/single.html` 第 8 行改为：

```html
{{if .Entry.Fields.cover}}<img src="{{media .Entry.Fields.cover}}" class="entry-hero" alt="{{.Entry.Content.Title}}">{{end}}
```

`themes/default/locales/zh.yaml` 第 11 行改为：

```yaml
hero_title: 食品加工设备专业制造商
```

- [ ] **步骤 2：验证**

运行：`go test -count=1 ./...`
预期：PASS（`internal/server/frontend_test.go` 有断言 body 含 `hreflang`、`canonical` 等，模板改动不应破坏）。

- [ ] **步骤 3：手动抽查（可选）**

重启后浏览器打开首页，源码应含 `link rel="icon"`、`twitter:card`、`og:image`；文章页 cover 图带 alt。

---

### 任务 5：生成 og-default.png 与 favicon.png

**文件：**
- 创建：`themes/default/static/img/og-default.png`（1200×630）
- 创建：`themes/default/static/img/favicon.png`（32×32）

- [ ] **步骤 1：用 PowerShell System.Drawing 生成**

在仓库根运行（生成含「金博威机械」文字的占位图；字体用 Microsoft YaHei）：

```powershell
Add-Type -AssemblyName System.Drawing
$img = New-Object System.Drawing.Bitmap 1200, 630
$g = [System.Drawing.Graphics]::FromImage($img)
$g.SmoothingMode = 'AntiAlias'
$g.TextRenderingHint = 'AntiAlias'
$g.Clear([System.Drawing.Color]::FromArgb(20, 40, 80))
$font = New-Object System.Drawing.Font('Microsoft YaHei', 72, [System.Drawing.FontStyle]::Bold)
$brush = [System.Drawing.Brushes]::White
$format = New-Object System.Drawing.StringFormat
$format.Alignment = 'Center'
$format.LineAlignment = 'Center'
$rect = New-Object System.Drawing.RectangleF 0, 180, 1200, 120
$g.DrawString('金博威机械', $font, $brush, $rect, $format)
$font2 = New-Object System.Drawing.Font('Microsoft YaHei', 28)
$g.DrawString('斩拌机 · 香肠机 · 拌馅机 食品机械', $font2, $brush, (New-Object System.Drawing.RectangleF 0, 330, 1200, 60), $format)
$g.Save()
$img.Save('themes\default\static\img\og-default.png', [System.Drawing.Imaging.ImageFormat]::Png)

$ico = New-Object System.Drawing.Bitmap 32, 32
$ig = [System.Drawing.Graphics]::FromImage($ico)
$ig.Clear([System.Drawing.Color]::FromArgb(20, 40, 80))
$if = New-Object System.Drawing.Font('Microsoft YaHei', 18, [System.Drawing.FontStyle]::Bold)
$ig.DrawString('博', $if, [System.Drawing.Brushes]::White, (New-Object System.Drawing.RectangleF 0, 2, 32, 32), $format)
$ig.Save()
$ico.Save('themes\default\static\img\favicon.png', [System.Drawing.Imaging.ImageFormat]::Png)
```

预期：两个 PNG 生成成功。

- [ ] **步骤 2：验证文件存在**

运行：`Get-ChildItem themes\default\static\img` 
预期：含 `og-default.png`、`favicon.png`、`hero.svg`。

- [ ] **步骤 3：服务可访问**

重启服务后：`curl -s -o NUL -w "%{http_code}" http://localhost:8080/themes/default/img/og-default.png`
预期：`200`。

---

### 任务 6：视频嵌入懒加载（admin + 主题 JS）

**文件：**
- 修改：`admin/src/dynamic-form/controls/RichTextControl.vue`
- 修改：`themes/default/static/js/main.js`

- [ ] **步骤 1：实现**

`RichTextControl.vue` 的 `insertEmbed` 中 iframe 字符串加 `loading="lazy"`：

```ts
    src: `<iframe width="315" height="560" src="${src}" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" allowfullscreen loading="lazy"></iframe>`,
```

`themes/default/static/js/main.js` 末尾（`})();` 之前）追加：

```js
  // SEO/性能：正文内嵌视频 iframe 懒加载（覆盖已存内容）
  var frames = document.querySelectorAll('.entry-content iframe');
  Array.prototype.forEach.call(frames, function (f) {
    if (!f.hasAttribute('loading')) f.setAttribute('loading', 'lazy');
  });
```

- [ ] **步骤 2：前端校验**

运行：`cd admin && npx vue-tsc --noEmit && npx vitest run`
预期：typecheck 无输出、37 个测试通过。

- [ ] **步骤 3：重建**

运行：`.\build.ps1`
预期：SPA 构建成功、dist 同步、`dulizhan.exe` 更新。

---

### 任务 7：config.yaml / config.example.yaml / docs 更新

**文件：**
- 修改：`config.yaml`
- 修改：`config.example.yaml`
- 修改：`docs/deployment.md`

- [ ] **步骤 1：config.yaml**

`config.yaml` 的 `site:` 段替换为：

```yaml
site:
  name: "金博威机械"
  names:
    zh: "金博威机械"
    en: "Jinbowei Machinery"
  url: "http://localhost:8080"
  description: "金博威机械专注食品加工设备研发制造，主营斩拌机、香肠机、拌馅机等肉类深加工机械，为食品企业提供高效、卫生、稳定的整体解决方案。"
  descriptions:
    zh: "金博威机械专注食品加工设备研发制造，主营斩拌机、香肠机、拌馅机等肉类深加工机械，为食品企业提供高效、卫生、稳定的整体解决方案。"
    en: "Jinbowei Machinery designs and builds food processing equipment — meat choppers, sausage making machines and mixers — for efficient, hygienic and reliable production."
  home_titles:
    zh: "金博威机械 - 斩拌机/香肠机/拌馅机食品机械厂家"
    en: "Jinbowei Machinery - Meat Chopper, Sausage Machine & Mixer Manufacturer"
  og_image: "/themes/default/img/og-default.png"
  default_lang: "zh"
  languages: ["zh", "en"]
  theme: "default"
  themes_dir: "./themes"
  home_products_category: "products"
  home_news_category: "news"
```

- [ ] **步骤 2：config.example.yaml**

`config.example.yaml` 的 `site:` 段替换为：

```yaml
site:
  name: "金博威机械"                  # 站点名（默认品牌名，zh）
  names:                              # 可选：按语言的品牌名（title 后缀 / og:site_name / JSON-LD）
    zh: "金博威机械"
    en: "Jinbowei Machinery"
  url: "http://localhost:8080"        # 上线后改为正式域名
  description: "站点默认描述（zh）"
  descriptions:                       # 可选：按语言的 meta description
    zh: "中文描述"
    en: "English description"
  home_titles:                        # 可选：按语言的首页 title（含关键词）
    zh: "金博威机械 - 关键词"
    en: "Jinbowei Machinery - Keywords"
  og_image: "/themes/default/img/og-default.png"   # 分享默认图（相对路径，输出时拼 site.url）
  default_lang: "zh"
  languages: ["zh", "en"]
  theme: "default"
  themes_dir: "./themes"
  # home_products_category: "products"   # 首页"产品中心"取该分类（含子分类）内容
  # home_news_category: "news"           # 首页"新闻动态"取该分类（含子分类）内容
```

- [ ] **步骤 3：docs/deployment.md**

在 `site` 配置说明处补充：`names`/`descriptions`/`home_titles` 为按语言覆盖（缺失回退 `name`/`description`），`og_image` 为分享默认图路径（可换 logo）。若文件已有 site 配置表格，按现有格式追加三行。

- [ ] **步骤 4：验证**

运行：`go test -count=1 ./internal/config/`
预期：PASS。

---

### 任务 8：清理测试内容

**文件：**
- 创建：`scripts/seo-content-cleanup/main.go`（临时脚本）
- 修改：运行后确认 `sitemap.xml` 不再含测试 slug

- [ ] **步骤 1：写清理脚本**

创建 `scripts/seo-content-cleanup/main.go`（`-dry-run` 时只打印不删除）：

```go
// 一次性脚本：删除测试内容（按 slug 匹配），供 SEO 审计后清理。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"dulizhan/internal/store/sqlite"
)

// 待删除的 slug（含 en 翻译组一并删）
var testSlugs = map[string]bool{
	"tiktok": true, "youtube": true, "news003": true, "news": true,
	"news-004": true, "news-005": true, "news-006": true, "news-007": true,
	"news-008": true, "news-009": true, "news-010": true, "run-test-1": true,
	"post-21": true, "chopper-mixer-test-data": true,
}

func main() {
	dry := flag.Bool("dry-run", false, "只打印命中，不删除")
	flag.Parse()
	if len(flag.Args()) < 1 {
		fmt.Fprintln(os.Stderr, "用法: go run ./scripts/seo-content-cleanup [-dry-run] <sqlite-dsn>")
		os.Exit(2)
	}
	ctx := context.Background()
	st, err := sqlite.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "打开库:", err)
		os.Exit(1)
	}
	types, err := st.ContentTypeRepo().List(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "列出类型:", err)
		os.Exit(1)
	}
	deleted := 0
	for _, ct := range types {
		for _, lang := range []string{"zh", "en"} {
			rows, err := st.ContentRepo().ListByTypeLangStatus(ctx, ct.Name, lang, "", 0, 100000)
			if err != nil {
				fmt.Fprintln(os.Stderr, "列出内容:", err)
				continue
			}
			for _, row := range rows {
				if !testSlugs[row.Slug] {
					continue
				}
				if *dry {
					fmt.Printf("命中待删: %s/%s lang=%s id=%d\n", ct.Name, row.Slug, row.Lang, row.ID)
					continue
				}
				group, err := st.ContentRepo().ListByContentID(ctx, row.ContentID)
				if err != nil {
					fmt.Fprintln(os.Stderr, "查内容组:", err)
					continue
				}
				for _, g := range group {
					if err := st.ContentRepo().Delete(ctx, g.ID); err != nil {
						fmt.Fprintf(os.Stderr, "删除 %s/%s (id=%d): %v\n", ct.Name, g.Slug, g.ID, err)
						continue
					}
					fmt.Printf("已删除 %s/%s (lang=%s, id=%d)\n", ct.Name, g.Slug, g.Lang, g.ID)
					deleted++
				}
			}
		}
	}
	fmt.Printf("共删除 %d 条\n", deleted)
}
```

- [ ] **步骤 2：dry-run 核对命中清单**

运行：`go run ./scripts/seo-content-cleanup -dry-run ./dulizhan.db`
预期：列出各测试 slug 的命中（zh/en）。**人工确认**清单与审计一致（尤其 `news`、`news003` 是否确为测试页；`tiktok`/`youtube` 若已作为演示保留，则从 `testSlugs` 删除后再执行）。

- [ ] **步骤 3：正式删除**

确认清单无误后：`go run ./scripts/seo-content-cleanup ./dulizhan.db`
预期：输出各测试 slug 删除记录，`共删除 N 条`。

- [ ] **步骤 4：验证 sitemap**

重启服务后：`curl -s http://localhost:8080/sitemap.xml | Select-String -Pattern 'tiktok|youtube|news003|run-test-1|post-21|chopper-mixer-test-data'`
预期：无匹配输出。

---

### 任务 9：全量验证 + 重启

**文件：** 无（验证为主）

- [ ] **步骤 1：Go 全量校验**

运行：`go test -count=1 ./...; if ($?) { go build ./...; go vet ./...; gofmt -l . }`
预期：全部通过，gofmt 无输出。

- [ ] **步骤 2：前端校验（若任务 6 改动过）**

运行：`cd admin && npx vue-tsc --noEmit && npx vitest run`
预期：通过。

- [ ] **步骤 3：重建并重启**

运行：`.\build.ps1`；停旧进程、启新进程（`Stop-Process -Name dulizhan -Force` 后 `Start-Process .\dulizhan.exe ...`）。
预期：`out.log` 显示启动成功，`8080` 监听。

- [ ] **步骤 4：浏览器抽查**

- 首页：`og:image`=默认图、`twitter:card`、JSON-LD 含 Organization/WebSite、title=「金博威机械 - 斩拌机/香肠机/拌馅机食品机械厂家」、favicon 生效、hero H1 中文。
- 分类页 `/category/products/chopper`：canonical=`https://localhost:8080/category/products/chopper`（非 404 死链）、JSON-LD 含 CollectionPage/ItemList。
- 产品详情页：canonical 正确、JSON-LD=Product+BreadcrumbList、og:image=cover。
- `/article/hello-zh/` → 301 到无尾斜杠。
- `/sitemap.xml`：含首页、分类页，无测试 slug。
