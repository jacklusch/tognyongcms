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
	p := c.Request.URL.Path
	if p != "/" && strings.HasSuffix(p, "/") {
		np := strings.TrimRight(p, "/")
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
	th, err := s.themes.Get(s.cfg.Site.Theme)
	if err != nil {
		c.String(http.StatusInternalServerError, "主题加载失败: %v", err)
		return
	}
	c.Status(http.StatusOK) // gin NoRoute 预置 404，成功渲染前显式置 200
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

	switch {
	case len(segs) == 0:
		s.renderHome(c, th, lang, data)
	case len(segs) == 1:
		s.renderList(c, th, lang, segs[0], data)
	case len(segs) >= 2 && segs[0] == "category":
		s.renderCategory(c, th, lang, segs[1:], data)
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
	// 首页产品/新闻分区：按配置的分类 slug 聚合内容（含子分类），未配置/不存在则留空回退
	if slug := s.cfg.Site.HomeProductsCategory; slug != "" {
		if items := s.homeCategoryItems(c.Request.Context(), "article", lang, slug, 6); items != nil {
			data.Products = items
		}
	}
	if slug := s.cfg.Site.HomeNewsCategory; slug != "" {
		if items := s.homeCategoryItems(c.Request.Context(), "article", lang, slug, 3); items != nil {
			data.News = items
		}
	}
	data.Meta = s.seo.BuildHome(lang)
	if err := th.Render(c.Writer, "index", data); err != nil {
		c.String(http.StatusInternalServerError, "渲染失败: %v", err)
	}
}

// homeCategoryItems 返回某分类（含子孙分类）下已发布内容，分类不存在返回 nil。
func (s *Server) homeCategoryItems(ctx context.Context, typeName, lang, slug string, limit int) []content.Entry {
	cat, err := s.store.CategoryRepo().GetBySlug(ctx, slug)
	if err != nil {
		return nil
	}
	ids := []int64{cat.ID}
	if ds, err := s.store.CategoryRepo().Descendants(ctx, cat.ID); err == nil {
		for _, d := range ds {
			ids = append(ids, d.ID)
		}
	}
	items, _, err := s.content.ListPublishedByCategories(ctx, typeName, lang, ids, 1, limit)
	if err != nil {
		return nil
	}
	return items
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
	tplKey := th.TemplateFor(typeName)
	// 单页面类型（映射到自定义模板且仅一个条目，如「联系我们」）：/<type> 直接渲染该详情页，
	// canonical 用短地址 /<type>，避免与 /<type>/<slug> 重复。
	if isSinglePageType(th, typeName) && page == 1 && total == 1 && len(items) == 1 {
		e := items[0]
		data.Entry = &e
		data.TypeName = typeName
		data.Meta = s.seo.BuildEntry(lang, e, nil, false)
		data.Meta.Canonical = s.cfg.Site.URL + s.i18n.URLPath(lang, "/"+typeName)
		if err := th.Render(c.Writer, tplKey, data); err != nil {
			c.String(http.StatusInternalServerError, "渲染失败: %v", err)
		}
		return
	}
	data.Items, data.Total, data.Page = items, total, page
	data.TypeName = typeName
	data.Meta = s.seo.BuildList(lang, typeName, page)
	key := tplKey
	if key == "single" {
		key = "list"
	}
	if err := th.Render(c.Writer, key, data); err != nil {
		c.String(http.StatusInternalServerError, "渲染失败: %v", err)
	}
}

func (s *Server) renderCategory(c *gin.Context, th *theme.Theme, lang string, pathSegs []string, data *theme.Data) {
	if len(pathSegs) == 0 {
		s.render404(c, th, lang)
		return
	}
	ctx := c.Request.Context()
	// 逐级解析路径：首段 GetBySlug，后续段在 ListChildren(prevID) 中按 slug 匹配
	cat, err := s.store.CategoryRepo().GetBySlug(ctx, pathSegs[0])
	if err != nil {
		if errors.Is(err, errs.ErrNotFound) {
			s.render404(c, th, lang)
		} else {
			c.String(http.StatusInternalServerError, "查询失败: %v", err)
		}
		return
	}
	for _, seg := range pathSegs[1:] {
		children, err := s.store.CategoryRepo().ListChildren(ctx, cat.ID)
		if err != nil {
			c.String(http.StatusInternalServerError, "查询失败: %v", err)
			return
		}
		var next *store.Category
		for i := range children {
			if children[i].Slug == seg {
				next = &children[i]
				break
			}
		}
		if next == nil {
			s.render404(c, th, lang)
			return
		}
		cat = *next
	}
	// 递归聚合：自身 + 全部子孙分类
	ids := []int64{cat.ID}
	descendants, err := s.store.CategoryRepo().Descendants(ctx, cat.ID)
	if err != nil {
		c.String(http.StatusInternalServerError, "查询失败: %v", err)
		return
	}
	for _, d := range descendants {
		ids = append(ids, d.ID)
	}
	// 聚合该分类下全部已发布内容（遍历类型）
	var all []content.Entry
	total := 0
	types, err := s.content.AllTypes(ctx)
	if err != nil {
		c.String(http.StatusInternalServerError, "查询失败: %v", err)
		return
	}
	for _, ct := range types {
		items, n, err := s.content.ListPublishedByCategories(ctx, ct.Name, lang, ids, 1, 1000)
		if err != nil {
			c.String(http.StatusInternalServerError, "查询失败: %v", err)
			return
		}
		all = append(all, items...)
		total += n
	}
	// 子分类导航：父链（顶级→当前分类）拼子分类 slug
	parentChain := s.categoryChain(ctx, cat.ID)
	children, err := s.store.CategoryRepo().ListChildren(ctx, cat.ID)
	if err != nil {
		c.String(http.StatusInternalServerError, "查询失败: %v", err)
		return
	}
	for _, ch := range children {
		data.SubCategories = append(data.SubCategories, theme.CategoryInfo{
			ID: ch.ID, Name: s.catName(ch, lang), Slug: ch.Slug,
			URL: s.categoryURL(lang, append(append([]string{}, parentChain...), ch.Slug)),
		})
	}
	data.Items, data.Total, data.Page, data.TypeName = all, total, 1, "category"
	data.EntryCategory = s.catName(cat, lang)
	data.Meta = s.seo.BuildCategory(lang, pathSegs, s.catName(cat, lang), all)
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
	var crumbs []seo.Breadcrumb
	isProduct := false
	if chain := s.entryCategoryChain(c.Request.Context(), e); chain != nil {
		cat := chain[len(chain)-1]
		data.EntryCategory = s.catName(cat, lang)
		slugs := make([]string, len(chain))
		for i, c := range chain {
			slugs[i] = c.Slug
		}
		data.EntryCategoryURL = s.categoryURL(lang, slugs)
		crumbs = s.entryBreadcrumbs(lang, chain, slugs, e.TypeName, e.Content.Slug, e.Content.Title)
		isProduct = len(slugs) > 0 && slugs[0] == s.cfg.Site.HomeProductsCategory // 顶级分类为产品分类即视为产品页
	}
	data.Meta = s.seo.BuildEntry(lang, e, crumbs, isProduct)
	if isSinglePageType(th, typeName) {
		// 单页面类型：规范地址用短地址 /<type>，与 /<type> 渲染保持一致
		data.Meta.Canonical = s.cfg.Site.URL + s.i18n.URLPath(lang, "/"+typeName)
	}
	if err := th.Render(c.Writer, th.TemplateFor(typeName), data); err != nil {
		c.String(http.StatusInternalServerError, "渲染失败: %v", err)
	}
}

// isSinglePageType 判断内容类型是否为「单页面」类型：主题把它映射到自定义模板（非 single/list）。
func isSinglePageType(th *theme.Theme, typeName string) bool {
	tpl := th.TemplateFor(typeName)
	return tpl != "single" && tpl != "list"
}

// entryCategoryChain 解析 entry 的 category 字段为分类链（自顶向下）；无分类/解析失败返回 nil。
func (s *Server) entryCategoryChain(ctx context.Context, e content.Entry) []store.Category {
	catIDStr, ok := e.Fields["category"].(string)
	if !ok || catIDStr == "" {
		return nil
	}
	id, err := strconv.ParseInt(catIDStr, 10, 64)
	if err != nil {
		return nil
	}
	cat, err := s.store.CategoryRepo().GetByID(ctx, id)
	if err != nil {
		return nil
	}
	return s.categoryChainInfo(ctx, cat.ID)
}

// entryCategoryURL 解析 entry 所属分类的归档 URL；无分类/解析失败返回空串。
// 模板函数无请求上下文，此处用 context.Background()（本地 DB 读，无取消需求）。
func (s *Server) entryCategoryURL(lang string, e content.Entry) string {
	chain := s.entryCategoryChain(context.Background(), e)
	if chain == nil {
		return ""
	}
	slugs := make([]string, len(chain))
	for i, c := range chain {
		slugs[i] = c.Slug
	}
	return s.categoryURL(lang, slugs)
}

// categoryChain 返回分类自顶级到自身的 slug 链（自顶向下），任一级查询失败返回 nil。
func (s *Server) categoryChain(ctx context.Context, id int64) []string {
	var slugs []string
	cur := id
	for cur != 0 {
		cat, err := s.store.CategoryRepo().GetByID(ctx, cur)
		if err != nil {
			return nil
		}
		slugs = append(slugs, cat.Slug)
		cur = cat.ParentID
	}
	for i, j := 0, len(slugs)-1; i < j; i, j = i+1, j-1 {
		slugs[i], slugs[j] = slugs[j], slugs[i]
	}
	return slugs
}

// categoryChainInfo 返回分类自顶级到自身的完整分类链（自顶向下），任一级查询失败返回 nil。
func (s *Server) categoryChainInfo(ctx context.Context, id int64) []store.Category {
	var chain []store.Category
	cur := id
	for cur != 0 {
		cat, err := s.store.CategoryRepo().GetByID(ctx, cur)
		if err != nil {
			return nil
		}
		chain = append(chain, cat)
		cur = cat.ParentID
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

// categoryURL 构造带语言前缀的分类归档 URL：/category/<slug1>/<slug2>/...
func (s *Server) categoryURL(lang string, chain []string) string {
	rest := "/category/" + strings.Join(chain, "/")
	return s.i18n.URLPath(lang, rest)
}

// catName 按语言取分类显示名：zh 用 Name；否则 NameEn 非空用 NameEn，空回退 Name。
func (s *Server) catName(cat store.Category, lang string) string {
	if lang == "zh" {
		return cat.Name
	}
	if cat.NameEn != "" {
		return cat.NameEn
	}
	return cat.Name
}

// entryBreadcrumbs 构造详情页面包屑：首页 > 分类链 > 当前页（URL 为绝对地址）。
func (s *Server) entryBreadcrumbs(lang string, chain []store.Category, slugs []string, typeName, slug, currentTitle string) []seo.Breadcrumb {
	homeName := "Home"
	if lang == "zh" {
		homeName = "首页"
	}
	out := []seo.Breadcrumb{{Name: homeName, URL: s.cfg.Site.URL + s.i18n.URLPath(lang, "/")}}
	for i := range chain {
		u := s.categoryURL(lang, slugs[:i+1])
		out = append(out, seo.Breadcrumb{Name: s.catName(chain[i], lang), URL: s.cfg.Site.URL + u})
	}
	out = append(out, seo.Breadcrumb{Name: currentTitle, URL: s.cfg.Site.URL + s.i18n.URLPath(lang, "/"+typeName+"/"+slug)})
	return out
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
	byID := make(map[int64]store.Category, len(cats))
	for _, cat := range cats {
		byID[cat.ID] = cat
	}
	chainOf := func(id int64) []string {
		var slugs []string
		cur := id
		for cur != 0 {
			c, ok := byID[cur]
			if !ok {
				return nil
			}
			slugs = append(slugs, c.Slug)
			cur = c.ParentID
		}
		for i, j := 0, len(slugs)-1; i < j; i, j = i+1, j-1 {
			slugs[i], slugs[j] = slugs[j], slugs[i]
		}
		return slugs
	}
	for _, cat := range cats {
		chain := chainOf(cat.ID)
		if chain == nil {
			continue
		}
		for _, l := range s.i18n.All() {
			entries = append(entries, seo.SitemapEntry{Loc: s.cfg.Site.URL + s.categoryURL(l.Code, chain)})
		}
	}
	th, thErr := s.themes.Get(s.cfg.Site.Theme)
	for _, ct := range types {
		single := thErr == nil && isSinglePageType(th, ct.Name)
		for _, l := range s.i18n.All() {
			items, _, err := s.content.ListPublished(ctx, ct.Name, l.Code, 1, 1000)
			if err != nil && !errors.Is(err, errs.ErrNotFound) {
				c.String(http.StatusInternalServerError, "查询失败: %v", err)
				return
			}
			if single {
				// 单页面类型：收录短地址 /<type>（有内容才收录）
				if len(items) > 0 {
					entries = append(entries, seo.SitemapEntry{Loc: s.cfg.Site.URL + s.i18n.URLPath(l.Code, "/"+ct.Name)})
				}
				continue
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
