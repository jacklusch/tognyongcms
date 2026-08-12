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
	return NewBuilder("测试站点", "https://example.com", "站点描述", reg)
}

func TestBuildEntry(t *testing.T) {
	b := testBuilder(t)
	now := time.Now().UTC()
	e := content.Entry{
		TypeName: "article",
		Content:  store.Content{Slug: "hello", Title: "你好世界", Status: "published", PublishedAt: &now},
		Fields:   map[string]any{"excerpt": "一句话摘要"},
	}
	m := b.BuildEntry("zh", e)
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
	m := b.BuildEntry("en", e)
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
