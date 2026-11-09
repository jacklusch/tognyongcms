package seo

import (
	"strings"
	"testing"
	"time"

	"dulizhan/internal/content"
	"dulizhan/internal/i18n"
	"dulizhan/internal/store"
)

func testBuilder(t *testing.T) *Builder {
	t.Helper()
	reg, err := i18n.New([]string{"zh", "en"}, "zh", false)
	if err != nil {
		t.Fatal(err)
	}
	return NewBuilder(Options{
		SiteURL:      "https://example.com",
		Names:        map[string]string{"zh": "测试站点", "en": "Test Site"},
		Descriptions: map[string]string{"zh": "站点描述", "en": "Site description"},
		HomeTitles:   map[string]string{"zh": "测试站点 - 关键词首页", "en": "Test Site - Keyword Home"},
		DefaultLang:  "zh",
		OGImage:      "/themes/default/img/og-default.png",
	}, reg)
}

func TestBuildEntry(t *testing.T) {
	b := testBuilder(t)
	now := time.Now().UTC()
	e := content.Entry{
		TypeName: "article",
		Content:  store.Content{Slug: "hello", Title: "你好世界", Status: "published", PublishedAt: &now},
		Fields:   map[string]any{"excerpt": "一句话摘要"},
	}
	m := b.BuildEntry("zh", e, nil, false)
	if !strings.Contains(m.Title, "你好世界") {
		t.Errorf("title = %q", m.Title)
	}
	if m.Canonical != "https://example.com/article/hello" {
		t.Errorf("canonical = %q", m.Canonical)
	}
	if len(m.HrefLangs) != 3 {
		t.Fatalf("hreflangs = %+v, want 3 (x-default/zh/en)", m.HrefLangs)
	}
	foundXD := false
	foundEn := false
	for _, h := range m.HrefLangs {
		if h.Lang == "x-default" && h.URL == "https://example.com/article/hello" {
			foundXD = true
		}
		if h.Lang == "en" && h.URL == "https://example.com/en/article/hello" {
			foundEn = true
		}
	}
	if !foundXD {
		t.Errorf("缺少 x-default hreflang: %+v", m.HrefLangs)
	}
	if !foundEn {
		t.Errorf("缺少 en hreflang: %+v", m.HrefLangs)
	}
	if m.OGTags["type"] != "article" {
		t.Errorf("og:type = %q", m.OGTags["type"])
	}
	if m.Description != "一句话摘要" {
		t.Errorf("description = %q", m.Description)
	}
	js := string(m.JSONLDScript)
	if !strings.HasPrefix(js, `<script type="application/ld+json">`) || !strings.Contains(js, `{"@context"`) {
		t.Errorf("JSON-LD 未包成完整 script 片段或对象被转义: %s", js)
	}
	if strings.Contains(js, `\"`) || strings.Contains(js, `\u003c`) {
		t.Errorf("JSON-LD 被转义: %s", js)
	}
}

func TestBuildEntryXDStartsWith(t *testing.T) {
	b := testBuilder(t)
	now := time.Now().UTC()
	e := content.Entry{TypeName: "article", Content: store.Content{Slug: "hello", Title: "t", PublishedAt: &now}}
	m := b.BuildEntry("en", e, nil, false)
	if len(m.HrefLangs) == 0 || m.HrefLangs[0].Lang != "x-default" {
		t.Fatalf("x-default 应为首条: %+v", m.HrefLangs)
	}
	if m.HrefLangs[0].URL != "https://example.com/article/hello" {
		t.Errorf("x-default 应指向默认语言 URL: %+v", m.HrefLangs[0])
	}
}

func TestBuildHome(t *testing.T) {
	b := testBuilder(t)
	m := b.BuildHome("en")
	if m.Canonical != "https://example.com/en/" {
		t.Errorf("canonical = %q", m.Canonical)
	}
	if m.OGTags["type"] != "website" {
		t.Errorf("og:type = %q", m.OGTags["type"])
	}
}

func TestSitemapXML(t *testing.T) {
	xml, err := SitemapXML([]SitemapEntry{
		{Loc: "https://example.com/article/hello", LastMod: "2026-08-05T10:00:00Z"},
		{Loc: "https://example.com/en/article/hello", LastMod: "2026-08-05T10:00:00Z"},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(xml)
	if !strings.Contains(s, "https://example.com/en/article/hello") {
		t.Errorf("sitemap 缺 en 条目: %s", s)
	}
	if !strings.Contains(s, "<urlset") {
		t.Errorf("sitemap 缺少 urlset 根: %s", s)
	}
}

func TestRobotsTXT(t *testing.T) {
	out := RobotsTXT("https://example.com")
	if !strings.Contains(string(out), "Sitemap: https://example.com/sitemap.xml") {
		t.Errorf("robots = %s", out)
	}
}

func TestBuildHomeJSONLD(t *testing.T) {
	b := testBuilder(t)
	m := b.BuildHome("en")
	js := string(m.JSONLDScript)
	if !strings.HasPrefix(js, `<script type="application/ld+json">`) {
		t.Errorf("home JSON-LD 未包 script: %s", js)
	}
}

func TestRobotsTXTNoTrailingSlash(t *testing.T) {
	out := RobotsTXT("https://example.com/")
	if !strings.Contains(string(out), "Sitemap: https://example.com/sitemap.xml") {
		t.Errorf("robots 尾斜杠未规范化: %s", out)
	}
}

func TestSitemapXMLDeclaration(t *testing.T) {
	xml, err := SitemapXML([]SitemapEntry{{Loc: "https://example.com/x", LastMod: ""}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(xml), `<?xml version="1.0"`) {
		t.Errorf("sitemap 缺 XML 声明: %s", xml)
	}
}

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
	if m.OGTags["type"] != "product" {
		t.Errorf("og:type = %q", m.OGTags["type"])
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

func TestBuildHomeLegacyConfigFallback(t *testing.T) {
	reg, err := i18n.New([]string{"zh", "en"}, "zh", false)
	if err != nil {
		t.Fatal(err)
	}
	b := NewBuilder(Options{
		SiteURL:     "https://example.com",
		Name:        "老站点",
		Description: "老描述",
		DefaultLang: "zh",
	}, reg)
	m := b.BuildHome("zh")
	if !strings.Contains(m.Title, "老站点") {
		t.Errorf("home title 应含默认名: %q", m.Title)
	}
	if m.Description != "老描述" {
		t.Errorf("home desc = %q", m.Description)
	}
	js := string(m.JSONLDScript)
	if !strings.Contains(js, `"name":"老站点"`) {
		t.Errorf("JSON-LD name 缺失: %s", js)
	}
	en := b.BuildEntry("en", content.Entry{TypeName: "article", Content: store.Content{Slug: "x", Title: "标题"}}, nil, false)
	if !strings.Contains(en.Title, "老站点") {
		t.Errorf("entry title 应含默认名: %q", en.Title)
	}
}
