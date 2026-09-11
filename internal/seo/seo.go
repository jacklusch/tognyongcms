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
	SiteURL      string
	Name         string // 默认品牌名（旧格式 site.name），map 缺失时回退
	Description  string // 默认描述（旧格式 site.description），map 缺失时回退
	Names        map[string]string
	Descriptions map[string]string
	HomeTitles   map[string]string
	DefaultLang  string
	OGImage      string
}

type Builder struct {
	siteURL            string
	defaultName        string
	defaultDescription string
	names              map[string]string
	descriptions       map[string]string
	homeTitles         map[string]string
	defaultLang        string
	ogImage            string
	reg                *i18n.Registry
}

func NewBuilder(opts Options, reg *i18n.Registry) *Builder {
	return &Builder{
		siteURL:            strings.TrimRight(opts.SiteURL, "/"),
		defaultName:        opts.Name,
		defaultDescription: opts.Description,
		names:              opts.Names,
		descriptions:       opts.Descriptions,
		homeTitles:         opts.HomeTitles,
		defaultLang:        opts.DefaultLang,
		ogImage:            opts.OGImage,
		reg:                reg,
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
	return b.defaultName
}

func (b *Builder) description(lang string) string {
	if v, ok := b.descriptions[lang]; ok && v != "" {
		return v
	}
	if v, ok := b.descriptions[b.defaultLang]; ok && v != "" {
		return v
	}
	return b.defaultDescription
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

// ogTags 组装 OG 标签；img 为空时不写入 image。
func (b *Builder) ogTags(title, desc, typ, canonical, img, lang string) map[string]string {
	og := map[string]string{
		"title": title, "description": desc, "type": typ,
		"url": canonical, "site_name": b.name(lang),
	}
	if img != "" {
		og["image"] = img
	}
	return og
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
	img := b.mediaURL(e.Fields["cover"])
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
	ogType := "article"
	if isProduct {
		ogType = "product"
	}
	og := b.ogTags(title, desc, ogType, canonical, img, lang)
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
	og := b.ogTags(title, desc, "website", canonical, img, lang)
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
	og := b.ogTags(title, desc, "website", canonical, img, lang)
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
	og := b.ogTags(title, desc, "website", canonical, img, lang)
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

// mediaURL 复用主题 mediaFunc 规则：非空且以 http(s) 或 / 开头的字符串才可用；
// 相对路径补全为绝对 URL。
func (b *Builder) mediaURL(v any) string {
	switch s := v.(type) {
	case string:
		if s != "" && (strings.HasPrefix(s, "http") || strings.HasPrefix(s, "/")) {
			if strings.HasPrefix(s, "/") {
				return b.siteURL + s
			}
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
