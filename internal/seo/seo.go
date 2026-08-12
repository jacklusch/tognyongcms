// internal/seo/seo.go
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
	OGTags       map[string]string
	JSONLDScript template.HTML
	HrefLangs    []HrefLang
}

type Builder struct {
	siteName    string
	siteURL     string
	description string
	reg         *i18n.Registry
}

func NewBuilder(siteName, siteURL, description string, reg *i18n.Registry) *Builder {
	return &Builder{siteName: siteName, siteURL: strings.TrimRight(siteURL, "/"), description: description, reg: reg}
}

// jsonLDScript 将 JSON-LD 包成完整 <script> 片段，避免 html/template
// 在 <script> 上下文中把值当 JS 转义。
func jsonLDScript(ld []byte) template.HTML {
	return template.HTML(`<script type="application/ld+json">` + string(ld) + `</script>`)
}

func (b *Builder) BuildEntry(lang string, e content.Entry) Meta {
	title := e.Content.Title
	if title == "" {
		title = b.siteName
	} else {
		title = title + " - " + b.siteName
	}
	desc, _ := e.Fields["excerpt"].(string)
	if desc == "" {
		desc = b.description
	}
	path := "/" + e.TypeName + "/" + e.Content.Slug
	canonical := b.siteURL + b.reg.URLPath(lang, path)
	hrefLangs := b.hrefLangs(lang, path)
	ld, err := json.Marshal(map[string]any{
		"@context":         "https://schema.org",
		"@type":            "Article",
		"headline":         e.Content.Title,
		"datePublished":    fmtTime(e.Content.PublishedAt),
		"dateModified":     e.Content.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"mainEntityOfPage": canonical,
	})
	if err != nil {
		ld = nil
	}
	m := Meta{
		Title:       title,
		Description: desc,
		Canonical:   canonical,
		HrefLangs:   hrefLangs,
		OGTags: map[string]string{
			"title":       title,
			"description": desc,
			"type":        "article",
			"url":         canonical,
			"site_name":   b.siteName,
		},
	}
	if len(ld) > 0 {
		m.JSONLDScript = jsonLDScript(ld)
	}
	return m
}

func (b *Builder) BuildList(lang, typeName string, page int) Meta {
	title := typeName + " - " + b.siteName
	path := "/" + typeName
	if page > 1 {
		path = fmt.Sprintf("/%s/page/%d", typeName, page)
	}
	canonical := b.siteURL + b.reg.URLPath(lang, path)
	return Meta{
		Title:       title,
		Description: b.description,
		Canonical:   canonical,
		HrefLangs:   b.hrefLangs(lang, "/"+typeName),
		OGTags: map[string]string{
			"title": title, "description": b.description, "type": "website",
			"url": canonical, "site_name": b.siteName,
		},
	}
}

func (b *Builder) BuildHome(lang string) Meta {
	title := b.siteName
	path := "/"
	canonical := b.siteURL + b.reg.URLPath(lang, path)
	ld, err := json.Marshal(map[string]any{
		"@context": "https://schema.org",
		"@type":    "WebSite",
		"name":     b.siteName,
		"url":      b.siteURL,
	})
	if err != nil {
		ld = nil
	}
	m := Meta{
		Title:       title,
		Description: b.description,
		Canonical:   canonical,
		HrefLangs:   b.hrefLangs(lang, "/"),
		OGTags: map[string]string{
			"title": title, "description": b.description, "type": "website",
			"url": canonical, "site_name": b.siteName,
		},
	}
	if len(ld) > 0 {
		m.JSONLDScript = jsonLDScript(ld)
	}
	return m
}

// hrefLangs 前置 x-default 条目（指向默认语言 URL），随后是各语言条目。
func (b *Builder) hrefLangs(lang, path string) []HrefLang {
	out := make([]HrefLang, 0, len(b.reg.All())+1)
	out = append(out, HrefLang{Lang: "x-default", URL: b.siteURL + b.reg.URLPath(b.reg.Default(), path)})
	for _, l := range b.reg.All() {
		out = append(out, HrefLang{Lang: l.Code, URL: b.siteURL + b.reg.URLPath(l.Code, path)})
	}
	return out
}

func fmtTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("2006-01-02T15:04:05Z")
}
