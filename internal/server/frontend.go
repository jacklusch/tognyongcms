// internal/server/frontend.go
package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"dulizhan/internal/content"
	"dulizhan/internal/errs"
	"dulizhan/internal/seo"
	"dulizhan/internal/store"
	"dulizhan/internal/theme"
)

const defaultPerPage = 10

func (s *Server) handleFrontend(c *gin.Context) {
	lang, segs := s.i18n.ResolvePath(c.Request.URL.Path)
	th, err := s.themes.Get(s.cfg.Site.Theme)
	if err != nil {
		c.String(http.StatusInternalServerError, "主题加载失败: %v", err)
		return
	}
	c.Status(http.StatusOK) // gin NoRoute 预置 404，成功渲染前显式置 200
	data := &theme.Data{
		Site: theme.SiteInfo{
			Name: s.cfg.Site.Name, URL: s.cfg.Site.URL, Description: s.cfg.Site.Description,
		},
		Lang:    lang,
		Langs:   s.i18n.All(),
		PerPage: defaultPerPage,
		Menus:   s.loadMenus(c.Request.Context(), lang),
	}

	switch {
	case len(segs) == 0:
		s.renderHome(c, th, lang, data)
	case len(segs) == 1:
		s.renderList(c, th, lang, segs[0], data)
	case len(segs) == 2 && segs[0] == "category":
		s.renderCategory(c, th, lang, segs[1], data)
	case len(segs) == 2:
		s.renderSingle(c, th, lang, segs[0], segs[1], data)
	default:
		s.render404(c, th, lang)
	}
}

func (s *Server) renderHome(c *gin.Context, th *theme.Theme, lang string, data *theme.Data) {
	items, total, err := s.content.ListPublished(c.Request.Context(), "article", lang, 1, defaultPerPage)
	if err != nil && !errors.Is(err, errs.ErrNotFound) {
		c.String(http.StatusInternalServerError, "查询失败: %v", err)
		return
	}
	data.Items, data.Total = items, total
	data.Meta = s.seo.BuildHome(lang)
	if err := th.Render(c.Writer, "index", data); err != nil {
		c.String(http.StatusInternalServerError, "渲染失败: %v", err)
	}
}

func (s *Server) renderList(c *gin.Context, th *theme.Theme, lang, typeName string, data *theme.Data) {
	if _, err := s.content.GetType(c.Request.Context(), typeName); err != nil {
		if errors.Is(err, errs.ErrNotFound) {
			s.render404(c, th, lang)
		} else {
			c.String(http.StatusInternalServerError, "查询失败: %v", err)
		}
		return
	}
	page := parsePage(c.Query("page"))
	items, total, err := s.content.ListPublished(c.Request.Context(), typeName, lang, page, defaultPerPage)
	if err != nil {
		c.String(http.StatusInternalServerError, "查询失败: %v", err)
		return
	}
	data.Items, data.Total, data.Page = items, total, page
	data.TypeName = typeName
	data.Meta = s.seo.BuildList(lang, typeName, page)
	key := th.TemplateFor(typeName)
	if key == "single" {
		key = "list"
	}
	if err := th.Render(c.Writer, key, data); err != nil {
		c.String(http.StatusInternalServerError, "渲染失败: %v", err)
	}
}

func (s *Server) renderCategory(c *gin.Context, th *theme.Theme, lang, slug string, data *theme.Data) {
	cat, err := s.store.CategoryRepo().GetBySlug(c.Request.Context(), slug)
	if err != nil {
		if errors.Is(err, errs.ErrNotFound) {
			s.render404(c, th, lang)
		} else {
			c.String(http.StatusInternalServerError, "查询失败: %v", err)
		}
		return
	}
	// 聚合该分类下全部已发布内容（遍历类型）
	var all []content.Entry
	total := 0
	types, err := s.content.AllTypes(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, "查询失败: %v", err)
		return
	}
	for _, ct := range types {
		items, n, err := s.content.ListPublishedByCategories(c.Request.Context(), ct.Name, lang, []int64{cat.ID}, 1, 1000)
		if err != nil {
			c.String(http.StatusInternalServerError, "查询失败: %v", err)
			return
		}
		all = append(all, items...)
		total += n
	}
	data.Items, data.Total, data.Page, data.TypeName = all, total, 1, "category"
	data.Meta = s.seo.BuildList(lang, cat.Name, 1)
	if err := th.Render(c.Writer, "list", data); err != nil {
		c.String(http.StatusInternalServerError, "渲染失败: %v", err)
	}
}

func (s *Server) renderSingle(c *gin.Context, th *theme.Theme, lang, typeName, slug string, data *theme.Data) {
	e, err := s.content.GetPublishedBySlugLang(c.Request.Context(), typeName, slug, lang)
	if err != nil {
		if errors.Is(err, errs.ErrNotFound) {
			s.render404(c, th, lang)
			return
		}
		c.String(http.StatusInternalServerError, "查询失败: %v", err)
		return
	}
	data.Entry = &e
	data.Meta = s.seo.BuildEntry(lang, e)
	// 解析分类：entry.Fields["category"] 是分类 id，查名称
	if catIDStr, ok := e.Fields["category"].(string); ok && catIDStr != "" {
		if id, err := strconv.ParseInt(catIDStr, 10, 64); err == nil {
			if cat, err := s.store.CategoryRepo().GetByID(c.Request.Context(), id); err == nil {
				data.EntryCategory = cat.Name
			}
		}
	}
	if err := th.Render(c.Writer, th.TemplateFor(typeName), data); err != nil {
		c.String(http.StatusInternalServerError, "渲染失败: %v", err)
	}
}

func (s *Server) render404(c *gin.Context, th *theme.Theme, lang string) {
	c.Writer.WriteHeader(http.StatusNotFound)
	_ = th.RenderNotFound(c.Writer)
}

func (s *Server) handleRobots(c *gin.Context) {
	c.Data(http.StatusOK, "text/plain; charset=utf-8", seo.RobotsTXT(s.cfg.Site.URL))
}

func (s *Server) handleSitemap(c *gin.Context) {
	ctx := c.Request.Context()
	types, err := s.content.AllTypes(ctx)
	if err != nil {
		c.String(http.StatusInternalServerError, "查询失败: %v", err)
		return
	}
	var entries []seo.SitemapEntry
	for _, ct := range types {
		for _, l := range s.i18n.All() {
			items, _, err := s.content.ListPublished(ctx, ct.Name, l.Code, 1, 1000)
			if err != nil && !errors.Is(err, errs.ErrNotFound) {
				c.String(http.StatusInternalServerError, "查询失败: %v", err)
				return
			}
			for _, it := range items {
				path := "/" + ct.Name + "/" + it.Content.Slug
				entries = append(entries, seo.SitemapEntry{
					Loc:     s.cfg.Site.URL + s.i18n.URLPath(l.Code, path),
					LastMod: formatLastMod(it),
				})
			}
		}
	}
	body, err := seo.SitemapXML(entries)
	if err != nil {
		c.String(http.StatusInternalServerError, "生成 sitemap 失败: %v", err)
		return
	}
	c.Data(http.StatusOK, "application/xml", body)
}

func formatLastMod(it content.Entry) string {
	if it.Content.PublishedAt != nil {
		return it.Content.PublishedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	return ""
}

func parsePage(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// loadMenus 查当前语言菜单，回退默认语言；解析为渲染就绪。
func (s *Server) loadMenus(ctx context.Context, lang string) []theme.Menu {
	menus := s.fetchMenus(ctx, lang)
	if len(menus) == 0 && lang != s.i18n.Default() {
		menus = s.fetchMenus(ctx, s.i18n.Default())
	}
	if len(menus) == 0 {
		return nil
	}
	out := make([]theme.Menu, 0, len(menus))
	for _, m := range menus {
		out = append(out, theme.Menu{Name: m.Name, Items: s.resolveMenuItems(m.Items, lang)})
	}
	return out
}

func (s *Server) fetchMenus(ctx context.Context, lang string) []store.Menu {
	items, err := s.store.MenuRepo().ListByLang(ctx, lang)
	if err != nil {
		return nil
	}
	return items
}

// resolveMenuItems 把存储 Items JSON 解析为渲染就绪（含语言前缀 URL）。
func (s *Server) resolveMenuItems(raw string, lang string) []theme.MenuItem {
	var items []store.MenuItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	return s.mapMenuItems(items, lang)
}

func (s *Server) mapMenuItems(items []store.MenuItem, lang string) []theme.MenuItem {
	out := make([]theme.MenuItem, 0, len(items))
	for _, it := range items {
		mi := theme.MenuItem{Label: it.Label, URL: s.menuURL(it, lang)}
		if len(it.Children) > 0 {
			mi.Children = s.mapMenuItems(it.Children, lang)
		}
		out = append(out, mi)
	}
	return out
}

// menuURL 根据 type 生成带语言前缀的 URL。
func (s *Server) menuURL(it store.MenuItem, lang string) string {
	switch {
	case it.Type == "home":
		return s.i18n.URLPath(lang, "/")
	case strings.HasPrefix(it.Type, "type:"):
		typeName := strings.TrimPrefix(it.Type, "type:")
		return s.i18n.URLPath(lang, "/"+typeName)
	default: // custom 或旧格式 {label,url}
		if it.URL == "" || it.URL == "/" {
			return s.i18n.URLPath(lang, "/")
		}
		if strings.HasPrefix(it.URL, "http://") || strings.HasPrefix(it.URL, "https://") || strings.HasPrefix(it.URL, "//") {
			return it.URL
		}
		return s.i18n.URLPath(lang, it.URL)
	}
}
