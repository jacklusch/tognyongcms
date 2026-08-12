package defaulttheme

import (
	"bytes"
	"testing"

	"dulizhan/internal/content"
	"dulizhan/internal/i18n"
	"dulizhan/internal/seo"
	"dulizhan/internal/store"
	"dulizhan/internal/theme"
)

func TestDefaultThemeLoadsAndRenders(t *testing.T) {
	reg, err := i18n.New([]string{"zh", "en"}, "zh", false)
	if err != nil {
		t.Fatal(err)
	}
	loader := theme.NewLoader("../../themes", reg)
	th, err := loader.Get("default")
	if err != nil {
		t.Fatalf("加载默认主题失败: %v", err)
	}
	buf := &bytes.Buffer{}
	entry := content.Entry{
		TypeName: "article",
		Content:  store.Content{Slug: "hello", Title: "你好世界"},
		Fields:   map[string]any{"content": "<p>正文</p>", "excerpt": "摘要"},
	}
	err = th.Render(buf, th.TemplateFor("article"), &theme.Data{
		Site: theme.SiteInfo{Name: "Dulizhan CMS", URL: "https://example.com", Description: "描述"},
		Lang: "zh", Langs: reg.All(),
		Entry: &entry,
		Meta:  seo.Meta{Title: "你好世界 - Dulizhan CMS", Canonical: "https://example.com/article/hello"},
	})
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	out := buf.String()
	if !contains(out, "你好世界") || !contains(out, "<p>正文</p>") {
		t.Errorf("渲染内容缺失: %s", out)
	}
}

func contains(s, sub string) bool { return bytes.Contains([]byte(s), []byte(sub)) }
