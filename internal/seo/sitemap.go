// internal/seo/sitemap.go
package seo

import (
	"encoding/xml"
	"strings"
)

type SitemapEntry struct {
	Loc     string
	LastMod string
}

type urlItem struct {
	XMLName xml.Name `xml:"url"`
	Loc     string   `xml:"loc"`
	LastMod string   `xml:"lastmod,omitempty"`
}

type urlset struct {
	XMLName xml.Name  `xml:"urlset"`
	Xmlns   string    `xml:"xmlns,attr"`
	URLs    []urlItem `xml:"url"`
}

func SitemapXML(entries []SitemapEntry) ([]byte, error) {
	items := make([]urlItem, 0, len(entries))
	for _, e := range entries {
		items = append(items, urlItem{Loc: e.Loc, LastMod: e.LastMod})
	}
	body, err := xml.MarshalIndent(urlset{Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: items}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), body...), nil
}

// RobotsTXT 规范化站点 URL 尾斜杠，避免生成 Sitemap: https://x//sitemap.xml。
func RobotsTXT(siteURL string) []byte {
	base := strings.TrimRight(siteURL, "/")
	return []byte("User-agent: *\nAllow: /\n\nSitemap: " + base + "/sitemap.xml\n")
}
