package defaulttheme

import (
	"bytes"
	"os"
	"strings"
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

func TestIndexRendersWithFewItems(t *testing.T) {
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
	items := []content.Entry{
		{TypeName: "article", Content: store.Content{Slug: "p1", Title: "产品一"}, Fields: map[string]any{"excerpt": "摘要"}},
	}
	err = th.Render(buf, "index", &theme.Data{
		Site:  theme.SiteInfo{Name: "Dulizhan CMS", URL: "https://example.com", Description: "描述"},
		Lang:  "zh",
		Langs: reg.All(),
		Items: items,
		Total: 1, PerPage: 10, Page: 1,
		Meta: seo.Meta{Title: "Dulizhan CMS", Canonical: "https://example.com/"},
	})
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	out := buf.String()
	if !contains(out, "产品一") {
		t.Errorf("渲染内容缺失: %s", out)
	}
}

func contains(s, sub string) bool { return bytes.Contains([]byte(s), []byte(sub)) }

// TestMainCSSStickyFooter 守护粘性页脚布局：短内容页页脚也应贴在视口底部。
func TestMainCSSStickyFooter(t *testing.T) {
	b, err := os.ReadFile("static/css/main.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(b)
	if !strings.Contains(css, "min-height:100vh") {
		t.Error("main.css 缺少 body min-height:100vh（页脚贴底所需）")
	}
	if !strings.Contains(css, "main{flex:1 0 auto}") {
		t.Error("main.css 缺少 main{flex:1 0 auto}（撑开剩余空间把页脚压到底部）")
	}
}

// TestFooterSocialLinks 页脚按 social 菜单渲染社交图标链接，未知平台回退文字。
func TestFooterSocialLinks(t *testing.T) {
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
	err = th.Render(buf, "index", &theme.Data{
		Site:  theme.SiteInfo{Name: "Dulizhan CMS", URL: "https://example.com", Description: "描述"},
		Lang:  "zh",
		Langs: reg.All(),
		Items: []content.Entry{{TypeName: "article", Content: store.Content{Slug: "p1", Title: "产品一"}}},
		Menus: []theme.Menu{{Name: "social", Items: []theme.MenuItem{
			{Label: "TikTok", URL: "https://www.tiktok.com/@jinbowei"},
			{Label: "官网", URL: "https://example.com/about"},
		}}},
		Meta: seo.Meta{Title: "Dulizhan CMS", Canonical: "https://example.com/"},
	})
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `href="https://www.tiktok.com/@jinbowei"`) {
		t.Errorf("缺少 TikTok 链接: %s", out)
	}
	if !strings.Contains(out, `class="social-link"`) || !strings.Contains(out, `<svg viewBox="0 0 24 24"`) {
		t.Errorf("缺少社交图标 SVG: %s", out)
	}
	if !strings.Contains(out, "关注我们") {
		t.Errorf("缺少 follow_us 文案: %s", out)
	}
	if !strings.Contains(out, "social-link-text") || !strings.Contains(out, "官网") {
		t.Errorf("未知平台应回退文字标签: %s", out)
	}
	if strings.Contains(out, "BACK TO TOP") {
		t.Error("页脚不应再有 BACK TO TOP")
	}
}

// TestContactTemplateRenders 联系我们模板渲染地址/电话/邮箱/营业时间与地图 iframe。
func TestContactTemplateRenders(t *testing.T) {
	reg, err := i18n.New([]string{"zh", "en"}, "zh", false)
	if err != nil {
		t.Fatal(err)
	}
	loader := theme.NewLoader("../../themes", reg)
	th, err := loader.Get("default")
	if err != nil {
		t.Fatalf("加载默认主题失败: %v", err)
	}
	entry := content.Entry{
		TypeName: "contact",
		Content:  store.Content{Slug: "contact", Title: "联系我们"},
		Fields: map[string]any{
			"address":   "示例市示例路 1 号",
			"phone":     "400 000 0000",
			"email":     "info@example.com",
			"hours":     "周一至周五 9:00 - 18:00",
			"map_embed": `<iframe src="https://www.google.com/maps/embed?pb=PLACEHOLDER" loading="lazy"></iframe>`,
		},
	}
	buf := &bytes.Buffer{}
	err = th.Render(buf, th.TemplateFor("contact"), &theme.Data{
		Site:  theme.SiteInfo{Name: "Dulizhan CMS", URL: "https://example.com"},
		Lang:  "zh",
		Langs: reg.All(),
		Entry: &entry,
		Meta:  seo.Meta{Title: "联系我们", Canonical: "https://example.com/contact/contact"},
	})
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"示例市示例路 1 号", "400 000 0000", "info@example.com", "周一至周五 9:00 - 18:00", "https://www.google.com/maps/embed?pb=PLACEHOLDER", "地址", "营业时间"} {
		if !strings.Contains(out, want) {
			t.Errorf("联系我们页缺少 %q\n%s", want, out)
		}
	}
}
