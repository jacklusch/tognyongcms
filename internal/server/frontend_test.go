package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"dulizhan/internal/auth"
	"dulizhan/internal/config"
	"dulizhan/internal/content"
	"dulizhan/internal/i18n"
	"dulizhan/internal/media"
	"dulizhan/internal/schema"
	"dulizhan/internal/seed"
	"dulizhan/internal/store"
	"dulizhan/internal/store/sqlite"
	"dulizhan/internal/theme"
)

func buildTestServer(t *testing.T, themesDir string) *Server {
	t.Helper()
	// 测试禁用 seed 封面图网络下载（离线/快速），cover 用 picsum 原 URL 兜底。
	t.Setenv("DULIZHAN_SEED_NO_DOWNLOAD", "1")
	st, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	reg, err := i18n.New([]string{"zh", "en"}, "zh", false)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Server.Addr = ":0"
	cfg.Server.DataDir = t.TempDir()
	cfg.Database.Driver = "sqlite"
	cfg.Site.Name = "测试站点"
	cfg.Site.URL = "https://example.com"
	cfg.Site.Description = "测试描述"
	cfg.Site.DefaultLang = "zh"
	cfg.Site.Languages = []string{"zh", "en"}
	cfg.Site.Theme = "fixture"
	cfg.Site.ThemesDir = themesDir
	cfg.Site.HomeProductsCategory = "products"
	cfg.Media.Driver = "local"

	authSvc := auth.New(st, time.Hour)
	if err := seed.EnsureAuth(context.Background(), st, authSvc); err != nil {
		t.Fatal(err)
	}
	svc := content.New(st, schema.NewRegistry(), []string{"zh", "en"})
	med := media.NewLocalStore(filepath.Join(cfg.Server.DataDir, "media"), "/media")
	if err := seed.Run(context.Background(), st, svc, med); err != nil {
		t.Fatal(err)
	}
	// seed 建了 demo 菜单，测试各自建菜单断言渲染；先清空保证 firstMenu 命中的是测试自身菜单
	clearMenus(t, st)
	srv, err := New(cfg, st, svc, reg, theme.NewLoader(themesDir, reg), authSvc, med)
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

func get(t *testing.T, srv *Server, path string) (int, string) {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	srv.engine.ServeHTTP(w, r)
	resp := w.Result()
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func clearMenus(t *testing.T, st store.Store) {
	t.Helper()
	ctx := context.Background()
	for _, lang := range []string{"zh", "en"} {
		ms, err := st.MenuRepo().ListByLang(ctx, lang)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range ms {
			if err := st.MenuRepo().Delete(ctx, m.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestFrontendRoutes(t *testing.T) {
	// 用仓库内默认主题
	srv := buildTestServer(t, "../../themes")
	cfg := srv.cfg
	cfg.Site.Theme = "default"

	// 首页
	code, body := get(t, srv, "/")
	if code != http.StatusOK {
		t.Fatalf("首页 = %d", code)
	}
	if !strings.Contains(body, "测试站点") {
		t.Errorf("首页缺站点名: %s", body)
	}

	// 文章详情（种子数据 zh）
	code, body = get(t, srv, "/article/hello-zh")
	if code != http.StatusOK {
		t.Fatalf("文章 = %d", code)
	}
	if !strings.Contains(body, "hreflang") {
		t.Errorf("缺 hreflang: %s", body)
	}
	if !strings.Contains(body, `href="https://example.com/en/article/hello-zh"`) {
		t.Errorf("缺 en 前缀链接: %s", body)
	}
	// Product 页（种子 chopper-1 归属 products 顶级分类）渲染 Product JSON-LD
	code, body = get(t, srv, "/article/chopper-1")
	if code != http.StatusOK {
		t.Fatalf("产品文章 = %d", code)
	}
	if !strings.Contains(body, `"@type":"Product"`) {
		t.Errorf("产品页缺 Product schema: %s", body)
	}
	// final-fix：JSON-LD 未转义、x-default hreflang 齐备
	if !strings.Contains(body, `application/ld+json">{"@context"`) {
		t.Errorf("JSON-LD 缺失或被转义: %s", body)
	}
	if !strings.Contains(body, `hreflang="x-default"`) {
		t.Errorf("缺 x-default hreflang: %s", body)
	}

	// 英文前缀
	code, body = get(t, srv, "/en/article/hello-en")
	if code != http.StatusOK {
		t.Fatalf("英文文章 = %d", code)
	}
	if !strings.Contains(body, "Hello World") {
		t.Errorf("英文文章内容缺失: %s", body)
	}

	// 列表
	code, _ = get(t, srv, "/article")
	if code != http.StatusOK {
		t.Fatalf("列表 = %d", code)
	}

	// 404
	code, body = get(t, srv, "/article/not-exist")
	if code != http.StatusNotFound {
		t.Fatalf("404 文章 = %d", code)
	}
	if !strings.Contains(body, "404") {
		t.Errorf("404 页面缺失: %s", body)
	}
}

func TestFrontendMenus(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	srv.cfg.Site.Theme = "default"

	ctx := context.Background()
	// 建 main 菜单（zh）：home + type:article + custom 二级
	items := `[{"label":"首页","type":"home","url":""},{"label":"文章","type":"type:article","url":"","children":[{"label":"关于","type":"custom","url":"/page/about"}]}]`
	if err := srv.store.MenuRepo().Create(ctx, &store.Menu{Name: "main", Lang: "zh", Items: items}); err != nil {
		t.Fatal(err)
	}

	code, body := get(t, srv, "/")
	if code != http.StatusOK {
		t.Fatalf("首页 = %d", code)
	}
	// 渲染出菜单：首页链接、文章列表链接（带语言前缀）、二级下拉
	if !strings.Contains(body, `>首页</a>`) {
		t.Errorf("缺首页导航: %s", body)
	}
	if !strings.Contains(body, `/article`) {
		t.Errorf("缺文章导航: %s", body)
	}
	if !strings.Contains(body, `/page/about`) {
		t.Errorf("缺二级导航: %s", body)
	}
}

func TestFrontendMenuExternalURL(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	srv.cfg.Site.Theme = "default"

	ctx := context.Background()
	// custom 项：外部绝对 URL、协议相对 URL 原样返回；相对路径仍走语言前缀逻辑
	items := `[{"label":"外链","type":"custom","url":"https://github.com/x"},{"label":"CDN","type":"custom","url":"//cdn.example.com/a"},{"label":"本地","type":"custom","url":"/page/about"}]`
	if err := srv.store.MenuRepo().Create(ctx, &store.Menu{Name: "main", Lang: "zh", Items: items}); err != nil {
		t.Fatal(err)
	}

	code, body := get(t, srv, "/")
	if code != http.StatusOK {
		t.Fatalf("首页 = %d", code)
	}
	if !strings.Contains(body, `href="https://github.com/x"`) {
		t.Errorf("外部绝对 URL 未原样返回: %s", body)
	}
	if !strings.Contains(body, `href="//cdn.example.com/a"`) {
		t.Errorf("协议相对 URL 未原样返回: %s", body)
	}
	if !strings.Contains(body, `href="/page/about"`) {
		t.Errorf("相对路径导航缺失: %s", body)
	}
	if strings.Contains(body, `/zh/https://`) {
		t.Errorf("外部 URL 被拼上语言前缀: %s", body)
	}
}

func TestFrontendMenuArbitraryName(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	srv.cfg.Site.Theme = "default"

	ctx := context.Background()
	// 复现用户 bug：菜单名不是 main（如 products），应仍被主导航渲染
	items := `[{"label":"产品中心","type":"type:page","url":""}]`
	if err := srv.store.MenuRepo().Create(ctx, &store.Menu{Name: "products", Lang: "zh", Items: items}); err != nil {
		t.Fatal(err)
	}

	code, body := get(t, srv, "/")
	if code != http.StatusOK {
		t.Fatalf("首页 = %d", code)
	}
	if !strings.Contains(body, `>产品中心</a>`) {
		t.Errorf("任意名菜单未渲染（nav.html 硬编码 main?）: %s", body)
	}
	if !strings.Contains(body, `href="/page"`) {
		t.Errorf("菜单项 /page 链接缺失: %s", body)
	}
}

func TestSEOEndpoints(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	srv.cfg.Site.Theme = "default"

	code, body := get(t, srv, "/sitemap.xml")
	if code != http.StatusOK || !strings.Contains(body, "/article/hello-zh") {
		t.Errorf("sitemap = %d %s", code, body)
	}
	if !strings.Contains(body, "/en/article/hello-en") {
		t.Errorf("sitemap 缺英文: %s", body)
	}

	code, body = get(t, srv, "/robots.txt")
	if code != http.StatusOK || !strings.Contains(body, "Sitemap:") {
		t.Errorf("robots = %d %s", code, body)
	}
}

func TestThemeStatic(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	srv.cfg.Site.Theme = "default"
	code, body := get(t, srv, "/themes/default/css/main.css")
	if code != http.StatusOK || !strings.Contains(body, ":root") {
		t.Errorf("主题静态资源 = %d", code)
	}
	// 路径穿越防护
	code, _ = get(t, srv, "/themes/default/static/../../config.yaml")
	if code != http.StatusNotFound {
		t.Errorf("路径穿越 = %d, want 404", code)
	}
}

func TestListPagination(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	srv.cfg.Site.Theme = "default"
	code, body := get(t, srv, "/article?page=2")
	if code != http.StatusOK {
		t.Errorf("列表 page=2 = %d", code)
	}
	if !strings.Contains(body, "pager") {
		t.Errorf("分页缺失: %s", body)
	}
}

func TestFrontendCategoryPage(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	srv.cfg.Site.Theme = "default"
	ctx := context.Background()
	// 建分类
	cat := &store.Category{Name: "新闻", Slug: "demo-news"}
	if err := srv.store.CategoryRepo().Create(ctx, cat); err != nil {
		t.Fatal(err)
	}
	// 建一篇含分类的已发布文章
	e, err := srv.content.Create(ctx, "article", "zh", map[string]any{
		"title": "分类文章", "slug": "cat-post",
		"content": "<p>正文</p>", "category": strconv.FormatInt(cat.ID, 10), // category 字段存分类 id
		"published_on": "2026-01-15",
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.content.SetStatus(ctx, e.Content.ID, "published"); err != nil {
		t.Fatal(err)
	}
	// 访问 /category/demo-news
	code, body := get(t, srv, "/category/demo-news")
	if code != http.StatusOK {
		t.Fatalf("/category/demo-news = %d", code)
	}
	if !strings.Contains(body, "分类文章") {
		t.Errorf("归档页未渲染该分类文章: %s", body)
	}
	// 不存在的分类 → 404
	code, _ = get(t, srv, "/category/nope")
	if code != http.StatusNotFound {
		t.Errorf("/category/nope = %d, want 404", code)
	}
}

// Task3：任意段路径 + 递归聚合 + 子分类导航 + EntryCategoryURL
func TestFrontendSubcategories(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	srv.cfg.Site.Theme = "default"
	ctx := context.Background()
	// 用 seed 已建的 产品(products) 顶级分类 与 斩拌机(chopper) 子分类
	products, err := srv.store.CategoryRepo().GetBySlug(ctx, "products")
	if err != nil {
		t.Fatal(err)
	}
	chopper, err := srv.store.CategoryRepo().GetBySlug(ctx, "chopper")
	if err != nil {
		t.Fatal(err)
	}
	if chopper.ParentID != products.ID {
		t.Fatalf("seed chopper ParentID = %d, want %d", chopper.ParentID, products.ID)
	}
	// 一篇 article 归 chopper
	e, err := srv.content.Create(ctx, "article", "zh", map[string]any{
		"title": "斩拌机文章", "slug": "chopper-post",
		"content": "<p>正文</p>", "category": strconv.FormatInt(chopper.ID, 10),
		"published_on": "2026-01-15",
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.content.SetStatus(ctx, e.Content.ID, "published"); err != nil {
		t.Fatal(err)
	}
	// 子分类归档页：GET /category/products/chopper → 200，含 chopper 文章标题
	code, body := get(t, srv, "/category/products/chopper")
	if code != http.StatusOK {
		t.Fatalf("/category/products/chopper = %d", code)
	}
	if !strings.Contains(body, "斩拌机文章") {
		t.Errorf("子分类归档页未渲染文章: %s", body)
	}
	// 父分类聚合（子孙）：GET /category/products → 200，聚合显示 chopper 文章
	code, body = get(t, srv, "/category/products")
	if code != http.StatusOK {
		t.Fatalf("/category/products = %d", code)
	}
	if !strings.Contains(body, "斩拌机文章") {
		t.Errorf("父分类聚合未显示子孙分类文章: %s", body)
	}
	if !strings.Contains(body, `class="sub-cat"`) {
		t.Errorf("父分类未渲染子分类导航: %s", body)
	}
	if !strings.Contains(body, `href="/category/products/chopper"`) {
		t.Errorf("子分类导航 URL 错误: %s", body)
	}
	// 超深路径 → 404
	code, _ = get(t, srv, "/category/products/chopper/x/y")
	if code != http.StatusNotFound {
		t.Errorf("/category/products/chopper/x/y = %d, want 404", code)
	}
	// 未知顶级 → 404
	code, _ = get(t, srv, "/category/nope")
	if code != http.StatusNotFound {
		t.Errorf("/category/nope = %d, want 404", code)
	}
	// 详情页：含 EntryCategory 与 /category/products/chopper 链接
	code, body = get(t, srv, "/article/chopper-post")
	if code != http.StatusOK {
		t.Fatalf("/article/chopper-post = %d", code)
	}
	if !strings.Contains(body, "斩拌机") {
		t.Errorf("详情页缺分类名: %s", body)
	}
	if !strings.Contains(body, `href="/category/products/chopper"`) {
		t.Errorf("详情页缺 EntryCategoryURL: %s", body)
	}
}

// I2：归档内容超过 defaultPerPage(10) 时不得渲染分页器（旧实现生成指向 404 的 /category/page/N 死链）。
func TestFrontendCategoryPageNoPager(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	srv.cfg.Site.Theme = "default"
	ctx := context.Background()
	cat := &store.Category{Name: "新闻", Slug: "demo-news"}
	if err := srv.store.CategoryRepo().Create(ctx, cat); err != nil {
		t.Fatal(err)
	}
	// 建 11 篇该分类下的已发布文章
	for i := 1; i <= 11; i++ {
		e, err := srv.content.Create(ctx, "article", "zh", map[string]any{
			"title": fmt.Sprintf("分类文章 %d", i), "slug": fmt.Sprintf("cat-post-%d", i),
			"content": "<p>正文</p>", "category": strconv.FormatInt(cat.ID, 10),
			"published_on": "2026-01-15",
		}, 0)
		if err != nil {
			t.Fatal(err)
		}
		if err := srv.content.SetStatus(ctx, e.Content.ID, "published"); err != nil {
			t.Fatal(err)
		}
	}
	code, body := get(t, srv, "/category/demo-news")
	if code != http.StatusOK {
		t.Fatalf("/category/news = %d", code)
	}
	if strings.Contains(body, "class=\"pager\"") {
		t.Errorf("归档页不应渲染分页器: %s", body)
	}
	if strings.Contains(body, "/category/page/") {
		t.Errorf("归档页出现分页死链: %s", body)
	}
}

func TestAdminAPIE2E(t *testing.T) {
	// 测试禁用 seed 封面图网络下载
	t.Setenv("DULIZHAN_SEED_NO_DOWNLOAD", "1")
	// 独立构造 server：注入 auth/media
	st, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	reg, err := i18n.New([]string{"zh", "en"}, "zh", false)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Server.Addr = ":0"
	cfg.Server.DataDir = t.TempDir()
	cfg.Database.Driver = "sqlite"
	cfg.Site.Name = "测试站点"
	cfg.Site.URL = "https://example.com"
	cfg.Site.Description = "描述"
	cfg.Site.DefaultLang = "zh"
	cfg.Site.Languages = []string{"zh", "en"}
	cfg.Site.Theme = "default"
	cfg.Site.ThemesDir = "../../themes"
	cfg.Media.Driver = "local"

	authSvc := auth.New(st, time.Hour)
	svc := content.New(st, schema.NewRegistry(), []string{"zh", "en"})
	if err := seed.EnsureAuth(context.Background(), st, authSvc); err != nil {
		t.Fatal(err)
	}
	if err := seed.Run(context.Background(), st, svc, media.NewLocalStore(t.TempDir(), "/media")); err != nil {
		t.Fatal(err)
	}
	srv := rebuildServer(t, cfg, st, svc, reg, authSvc)

	// 登录
	r := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"admin","password":"admin123"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.engine.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("login = %d %s", w.Code, w.Body.String())
	}
	var lr struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &lr)
	tok := lr.Data.Token

	// 建类型 + 建内容 + 发布
	authReq := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		srv.engine.ServeHTTP(w, req)
		return w
	}
	// Fix Report ①：title 字段带 indexed:true，Content.Title 才会被填充
	w = authReq(http.MethodPost, "/api/content-types", `{"name":"note","label":"笔记","fields":[{"name":"title","label":"标题","type":"text","required":true,"indexed":true}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("create type = %d %s", w.Code, w.Body.String())
	}
	w = authReq(http.MethodPost, "/api/content", `{"type":"note","lang":"zh","data":{"title":"接口发布的内容"}}`)
	if w.Code != http.StatusOK {
		t.Fatalf("create content = %d %s", w.Code, w.Body.String())
	}
	var cr struct {
		Data struct {
			Content struct {
				Content struct {
					ID int64 `json:"id"`
				} `json:"content"`
			} `json:"content"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &cr)
	id := cr.Data.Content.Content.ID
	w = authReq(http.MethodPost, fmt.Sprintf("/api/content/%d/publish", id), "")
	if w.Code != http.StatusOK {
		t.Fatalf("publish = %d %s", w.Code, w.Body.String())
	}

	// 前台可访问该类型页面（Fix Report ②：同时校验 200 且渲染出内容标题）
	r2 := httptest.NewRequest(http.MethodGet, "/note", nil)
	w2 := httptest.NewRecorder()
	srv.engine.ServeHTTP(w2, r2)
	if w2.Code != http.StatusOK {
		t.Errorf("note list = %d", w2.Code)
	}
	if !strings.Contains(w2.Body.String(), "接口发布的内容") {
		t.Errorf("note 列表未渲染 API 发布的内容: %s", w2.Body.String())
	}
}

func TestHomeCategoryItems(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	ctx := context.Background()

	// products 分类含子分类（chopper/sausage-machine/mixer），聚合应为 5 篇
	items := srv.homeCategoryItems(ctx, "article", "zh", "products", 10)
	got := map[string]bool{}
	for _, it := range items {
		got[it.Content.Slug] = true
	}
	for _, slug := range []string{"chopper-1", "sausage-machine-1", "mixer-1", "products-1", "products-2"} {
		if !got[slug] {
			t.Errorf("products 分类聚合缺 %s", slug)
		}
	}

	// news 分类应为 3 篇
	news := srv.homeCategoryItems(ctx, "article", "zh", "news", 10)
	newsSlugs := map[string]bool{}
	for _, it := range news {
		newsSlugs[it.Content.Slug] = true
	}
	for _, slug := range []string{"news-1", "news-2", "hello-zh"} {
		if !newsSlugs[slug] {
			t.Errorf("news 分类聚合缺 %s", slug)
		}
	}

	// 不存在的分类返回 nil
	if srv.homeCategoryItems(ctx, "article", "zh", "not-exist", 10) != nil {
		t.Errorf("不存在分类应返回 nil")
	}
}

func TestCategoryCanonicalUsesSlugChain(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	srv.cfg.Site.Theme = "default"
	ctx := context.Background()
	chopper, err := srv.store.CategoryRepo().GetBySlug(ctx, "chopper")
	if err != nil {
		t.Fatalf("chopper 分类: %v", err)
	}
	// 给 chopper 挂一篇已发布内容
	catID := strconv.FormatInt(chopper.ID, 10)
	_, err = srv.content.Create(ctx, "article", "zh", map[string]any{
		"title": "斩拌机Z", "slug": "chopper-z", "category": catID, "content": "<p>x</p>",
		"published_on": "2026-01-15",
	}, 1)
	if err != nil {
		t.Fatalf("创建内容: %v", err)
	}
	code, body := get(t, srv, "/category/products/chopper")
	if code != http.StatusOK {
		t.Fatalf("/category/products/chopper = %d", code)
	}
	if !strings.Contains(body, `rel="canonical" href="https://example.com/category/products/chopper"`) {
		t.Errorf("分类页 canonical 应指向 slug 链: %s", body)
	}
	if strings.Contains(body, "%e6%96%a9") {
		t.Errorf("canonical 出现 URL 编码分类名: %s", body)
	}
}

func TestTrailingSlashRedirectsToNoSlash(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	srv.cfg.Site.Theme = "default"
	req := httptest.NewRequest(http.MethodGet, "https://example.com/article/hello-zh/", nil)
	w := httptest.NewRecorder()
	srv.engine.ServeHTTP(w, req)
	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMovedPermanently {
		t.Fatalf("/article/hello-zh/ = %d, want 301", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/article/hello-zh" {
		t.Errorf("Location = %q, want /article/hello-zh", loc)
	}
	// 根路径不受影响
	req2 := httptest.NewRequest(http.MethodGet, "https://example.com/", nil)
	w2 := httptest.NewRecorder()
	srv.engine.ServeHTTP(w2, req2)
	if w2.Result().StatusCode != http.StatusOK {
		t.Errorf("根路径应保持 200: %d", w2.Result().StatusCode)
	}
	// query 保留
	req3 := httptest.NewRequest(http.MethodGet, "https://example.com/article/hello-zh/?a=1", nil)
	w3 := httptest.NewRecorder()
	srv.engine.ServeHTTP(w3, req3)
	resp3 := w3.Result()
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusMovedPermanently {
		t.Fatalf("/article/hello-zh/?a=1 = %d, want 301", resp3.StatusCode)
	}
	if loc := resp3.Header.Get("Location"); loc != "/article/hello-zh?a=1" {
		t.Errorf("Location = %q, want /article/hello-zh?a=1", loc)
	}
}

func TestSitemapIncludesHomeAndCategories(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	srv.cfg.Site.Theme = "default"
	code, body := get(t, srv, "/sitemap.xml")
	if code != http.StatusOK {
		t.Fatalf("sitemap = %d", code)
	}
	for _, want := range []string{
		"https://example.com/",
		"https://example.com/category/products",
		"https://example.com/category/news",
		"https://example.com/article/hello-zh",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("sitemap 缺 %q", want)
		}
	}
}

// Task5：前台分类名按语言本地化（zh 用 Name，en 用 NameEn，空回退 Name）
func TestCategoryTitleLocalized(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	srv.cfg.Site.Theme = "default"
	ctx := context.Background()
	// 给 demo 分类回填英文名（模拟 seed 已生效）
	for _, c := range []struct{ slug, en string }{
		{"products", "Products"}, {"news", "News"}, {"about", "About"},
		{"chopper", "Chopper"}, {"sausage-machine", "Sausage Machine"}, {"mixer", "Mixer"},
	} {
		cat, err := srv.store.CategoryRepo().GetBySlug(ctx, c.slug)
		if err != nil {
			continue
		}
		cat.NameEn = c.en
		if err := srv.store.CategoryRepo().Update(ctx, &cat); err != nil {
			t.Fatalf("回填 %s: %v", c.slug, err)
		}
	}
	// en 分类页 title 英文（buildTestServer 品牌名为「测试站点」，en 回退同名）
	code, body := get(t, srv, "/en/category/products")
	if code != http.StatusOK {
		t.Fatalf("/en/category/products = %d", code)
	}
	if !strings.Contains(body, "<title>Products - 测试站点</title>") {
		t.Errorf("en 分类页 title 应为英文: %s", body)
	}
	if !strings.Contains(body, ">Chopper<") {
		t.Errorf("en 子分类导航应显示英文名: %s", body)
	}
	// zh 分类页 title 中文
	code, body = get(t, srv, "/category/products")
	if code != http.StatusOK {
		t.Fatalf("/category/products = %d", code)
	}
	if !strings.Contains(body, "<title>产品 - 测试站点</title>") {
		t.Errorf("zh 分类页 title 应为中文: %s", body)
	}
}

func TestEntryCategoryNameLocalized(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	srv.cfg.Site.Theme = "default"
	ctx := context.Background()
	cat, err := srv.store.CategoryRepo().GetBySlug(ctx, "chopper")
	if err != nil {
		t.Fatalf("chopper: %v", err)
	}
	cat.NameEn = "Chopper"
	if err := srv.store.CategoryRepo().Update(ctx, &cat); err != nil {
		t.Fatal(err)
	}
	// en 详情页：面包屑/EntryCategory 用英文
	code, body := get(t, srv, "/en/article/chopper-1-en")
	if code != http.StatusOK {
		t.Fatalf("en 详情 = %d", code)
	}
	if !strings.Contains(body, `"Chopper"`) {
		t.Errorf("en 详情页分类名应为英文 Chopper: %s", body)
	}
}

// rebuildServer：把 auth/media 注入 server 并重建引擎
func rebuildServer(t *testing.T, cfg *config.Config, st store.Store, svc *content.Service, reg *i18n.Registry, authSvc *auth.Service) *Server {
	t.Helper()
	med := media.NewLocalStore(filepath.Join(cfg.Server.DataDir, "media"), "/media")
	srv, err := New(cfg, st, svc, reg, theme.NewLoader(cfg.Site.ThemesDir, reg), authSvc, med)
	if err != nil {
		t.Fatal(err)
	}
	return srv
}
