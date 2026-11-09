package theme

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dulizhan/internal/content"
	"dulizhan/internal/i18n"
	"dulizhan/internal/seo"
	"dulizhan/internal/store"
)

// 构造一个最小主题目录。
func writeFixtureTheme(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"theme.yaml": `name: fixture
version: 1.0.0
templates:
  index: templates/index.html
  single: templates/single.html
  list: templates/list.html
partials:
  - templates/partials/nav.html
type_map:
  article: single
  page: page
locales:
  zh: locales/zh.yaml
  en: locales/en.yaml
`,
		"templates/index.html": `{{define "head"}}{{end}}
{{define "body"}}
<header>{{template "nav" .}}</header>
<main><h1>{{.Site.Name}}</h1>
<ul>{{range .Items}}<li><a href="{{entryCategoryURL $.Lang .}}">{{.Content.Title}}</a></li>{{end}}</ul>
</main>
{{end}}`,
		"templates/single.html": `{{define "head"}}{{end}}
{{define "body"}}
<header>{{template "nav" .}}</header>
<main><article><h1>{{.Entry.Content.Title}}</h1><div>{{.Entry.Content.Title}}</div></article></main>
{{end}}`,
		"templates/list.html": `{{define "head"}}{{end}}
{{define "body"}}<h1>列表</h1>{{range .Items}}<p>{{.Content.Title}}</p>{{end}}{{end}}`,
		"templates/partials/nav.html": `{{define "nav"}}<a href="{{home $.Lang}}">{{t $.Lang "home"}}</a>{{end}}`,
		"locales/zh.yaml":             "home: 首页\n",
		"locales/en.yaml":             "home: Home\n",
	}
	for name, body := range files {
		p := filepath.Join(dir, "fixture", name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func testTheme(t *testing.T) (*Theme, *i18n.Registry) {
	t.Helper()
	reg, err := i18n.New([]string{"zh", "en"}, "zh", false)
	if err != nil {
		t.Fatal(err)
	}
	loader := NewLoader(writeFixtureTheme(t), reg)
	th, err := loader.Load("fixture")
	if err != nil {
		t.Fatal(err)
	}
	return th, reg
}

func TestRenderIndex(t *testing.T) {
	th, reg := testTheme(t)
	buf := &bytes.Buffer{}
	entry := content.Entry{TypeName: "article", Content: store.Content{Slug: "hello", Title: "你好世界"}}
	err := th.Render(buf, "index", &Data{
		Site:  SiteInfo{Name: "测试站点", URL: "https://example.com"},
		Lang:  "zh",
		Langs: reg.All(),
		Items: []content.Entry{entry},
		Meta:  seo.Meta{Title: "测试站点", Canonical: "https://example.com/", HrefLangs: []seo.HrefLang{{Lang: "zh", URL: "https://example.com/"}}},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "你好世界") {
		t.Errorf("缺少内容: %s", out)
	}
	if !strings.Contains(out, `rel="canonical" href="https://example.com/"`) {
		t.Errorf("缺少 canonical: %s", out)
	}
	if !strings.Contains(out, "首页") {
		t.Errorf("缺少中文导航: %s", out)
	}
	if !strings.Contains(out, `href="/article/hello"`) {
		t.Errorf("缺少条目链接: %s", out)
	}
}

func TestRenderSingleUsesTypeMap(t *testing.T) {
	th, reg := testTheme(t)
	buf := &bytes.Buffer{}
	entry := content.Entry{TypeName: "article", Content: store.Content{Slug: "hello", Title: "你好世界"}}
	err := th.Render(buf, th.TemplateFor("article"), &Data{
		Site: SiteInfo{Name: "测试站点"}, Lang: "zh", Langs: reg.All(),
		Entry: &entry,
		Meta:  seo.Meta{Title: "你好世界", Canonical: "https://example.com/article/hello"},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(buf.String(), "<article>") {
		t.Errorf("应渲染 single 模板: %s", buf.String())
	}
}

func TestMenuData(t *testing.T) {
	d := &Data{Menus: []Menu{
		{Name: "main", Items: []MenuItem{{Label: "首页", URL: "/"}}},
	}}
	if got := menuData(d, "main"); len(got) != 1 || got[0].Label != "首页" {
		t.Errorf("menu main = %+v", got)
	}
	if got := menuData(d, "nope"); got != nil {
		t.Errorf("缺名菜单应为 nil, got %+v", got)
	}
}

func TestFirstMenu(t *testing.T) {
	// 任意名字的第一个有导航项的菜单
	d := &Data{Menus: []Menu{
		{Name: "products", Items: []MenuItem{{Label: "产品中心", URL: "/page"}}},
		{Name: "footer", Items: []MenuItem{{Label: "关于", URL: "/page/about"}}},
	}}
	if got := firstMenu(d); len(got) != 1 || got[0].Label != "产品中心" {
		t.Errorf("firstMenu = %+v", got)
	}
	// 空菜单集 → nil
	if got := firstMenu(&Data{}); got != nil {
		t.Errorf("空菜单集应为 nil, got %+v", got)
	}
	// 全是空 items → nil
	empty := &Data{Menus: []Menu{{Name: "x", Items: []MenuItem{}}, {Name: "y", Items: nil}}}
	if got := firstMenu(empty); got != nil {
		t.Errorf("全空菜单应为 nil, got %+v", got)
	}
}

func TestLookupLocale(t *testing.T) {
	th, _ := testTheme(t)
	if got := th.LookupLocale("zh", "home"); got != "首页" {
		t.Errorf("zh home = %q", got)
	}
	if got := th.LookupLocale("en", "home"); got != "Home" {
		t.Errorf("en home = %q", got)
	}
	if got := th.LookupLocale("en", "missing_key"); got != "missing_key" {
		t.Errorf("缺失回退 = %q", got)
	}
}

func TestRenderNotFound(t *testing.T) {
	th, _ := testTheme(t)
	buf := &bytes.Buffer{}
	if err := th.RenderNotFound(buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "404") {
		t.Errorf("内置404缺失: %s", buf.String())
	}
}

func TestRenderIndexEnPrefix(t *testing.T) {
	th, reg := testTheme(t)
	buf := &bytes.Buffer{}
	entry := content.Entry{TypeName: "article", Content: store.Content{Slug: "hello", Title: "你好世界"}}
	err := th.Render(buf, "index", &Data{
		Site:  SiteInfo{Name: "测试站点", URL: "https://example.com"},
		Lang:  "en",
		Langs: reg.All(),
		Items: []content.Entry{entry},
		Meta:  seo.Meta{Title: "x", Canonical: "https://example.com/en/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `href="/en/article/hello"`) {
		t.Errorf("en 前缀 URL 缺失: %s", buf.String())
	}
}

func TestLoaderDebugReload(t *testing.T) {
	dir := writeFixtureTheme(t) // 已存在 helper，写入 dir/fixture
	reg, _ := i18n.New([]string{"zh", "en"}, "zh", false)
	loader := NewLoader(dir, reg, true) // debug 模式

	th, err := loader.Get("fixture")
	if err != nil {
		t.Fatal(err)
	}
	// 首次加载内容
	// 修改 theme.yaml 的 mtime
	oldPath := filepath.Join(dir, "fixture", "theme.yaml")
	// 改一个模板文件 mtime，验证 Get 重新加载
	singlePath := filepath.Join(dir, "fixture", "templates", "single.html")
	if err := os.Chtimes(singlePath, time.Now(), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	_ = oldPath
	th2, err := loader.Get("fixture")
	if err != nil {
		t.Fatal(err)
	}
	if th2 == th {
		t.Error("debug 模式 mtime 变更后应重新加载新 Theme 实例")
	}
}

func TestLoaderNonDebugCache(t *testing.T) {
	dir := writeFixtureTheme(t)
	reg, _ := i18n.New([]string{"zh", "en"}, "zh", false)
	loader := NewLoader(dir, reg, false) // 非 debug
	th1, _ := loader.Get("fixture")
	th2, _ := loader.Get("fixture")
	if th1 != th2 {
		t.Error("非 debug 模式应返回缓存同一实例")
	}
}

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
