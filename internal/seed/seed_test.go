package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"dulizhan/internal/auth"
	"dulizhan/internal/content"
	"dulizhan/internal/schema"
	"dulizhan/internal/store"
	"dulizhan/internal/store/sqlite"
)

func TestEnsureAuth(t *testing.T) {
	ctx := context.Background()
	st, _ := sqlite.Open(":memory:")
	authSvc := auth.New(st, time.Hour)

	if err := EnsureAuth(ctx, st, authSvc); err != nil {
		t.Fatal(err)
	}
	// 幂等
	if err := EnsureAuth(ctx, st, authSvc); err != nil {
		t.Fatalf("第二次应幂等: %v", err)
	}

	roles, err := st.RoleRepo().List(ctx)
	if err != nil || len(roles) != 3 {
		t.Fatalf("roles = %v, %v", roles, err)
	}
	// 固定创建顺序：admin < editor < author，admin 必为最小 id
	admin, err := st.RoleRepo().GetByName(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	editor, err := st.RoleRepo().GetByName(ctx, "editor")
	if err != nil {
		t.Fatal(err)
	}
	if admin.ID != 1 {
		t.Errorf("admin id = %d, want 1", admin.ID)
	}
	// 默认管理员可登录
	if _, err := authSvc.Login(ctx, "admin", "admin123"); err != nil {
		t.Errorf("默认管理员登录失败: %v", err)
	}
	// author 角色权限不含发布
	author, err := st.RoleRepo().GetByName(ctx, "author")
	if err != nil {
		t.Fatal(err)
	}
	if !(admin.ID < editor.ID && editor.ID < author.ID) {
		t.Errorf("角色 id 顺序异常: admin=%d editor=%d author=%d", admin.ID, editor.ID, author.ID)
	}
	if !contains(author.Permissions, "content.write") || contains(author.Permissions, "content.publish") {
		t.Errorf("author 权限异常: %s", author.Permissions)
	}
}

func TestEnsureAuthEditorPerms(t *testing.T) {
	ctx := context.Background()
	st, _ := sqlite.Open(":memory:")
	authSvc := auth.New(st, time.Hour)
	if err := EnsureAuth(ctx, st, authSvc); err != nil {
		t.Fatal(err)
	}
	editor, err := st.RoleRepo().GetByName(ctx, "editor")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(editor.Permissions, `"content.*"`) {
		t.Errorf("editor 应含 content.* 通配: %s", editor.Permissions)
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func TestSeedRunTranslationGroup(t *testing.T) {
	t.Setenv("DULIZHAN_SEED_NO_DOWNLOAD", "1") // 测试禁用封面图网络下载
	ctx := context.Background()
	st, _ := sqlite.Open(":memory:")
	reg := schema.NewRegistry()
	svc := content.New(st, reg, []string{"zh", "en"})

	if err := Run(ctx, st, svc, &mockMediaStore{}); err != nil {
		t.Fatal(err)
	}
	// zh/en 应共享翻译组 content_id（多语言切换可用）
	zh, err := svc.GetPublishedBySlugLang(ctx, "article", "hello-zh", "zh")
	if err != nil {
		t.Fatal(err)
	}
	en, err := svc.GetPublishedBySlugLang(ctx, "article", "hello-en", "en")
	if err != nil {
		t.Fatal(err)
	}
	if zh.Content.ContentID == "" || zh.Content.ContentID != en.Content.ContentID {
		t.Errorf("zh/en 应共享 content_id: zh=%q en=%q", zh.Content.ContentID, en.Content.ContentID)
	}
	// 幂等：再跑一次不报错
	if err := Run(ctx, st, svc, &mockMediaStore{}); err != nil {
		t.Errorf("第二次 Run 应幂等: %v", err)
	}
}

func TestSeedDemoData(t *testing.T) {
	t.Setenv("DULIZHAN_SEED_NO_DOWNLOAD", "1") // 测试禁用封面图网络下载
	ctx := context.Background()
	st, _ := sqlite.Open(":memory:")
	reg := schema.NewRegistry()
	svc := content.New(st, reg, []string{"zh", "en"})
	mock := &mockMediaStore{} // 任务 1 定义

	if err := Run(ctx, st, svc, mock); err != nil {
		t.Fatal(err)
	}
	// 分类建了 6 个（3 顶级 + products 下 3 子分类）
	cats, err := st.CategoryRepo().List(ctx)
	if err != nil || len(cats) != 6 {
		t.Errorf("分类数 = %d, %v", len(cats), err)
	}
	// 每分类双语文章存在（zh 已发布 + en 同组翻译）
	for _, slug := range []string{"news-1", "about-1", "products-1"} {
		zh, err := svc.GetPublishedBySlugLang(ctx, "article", slug, "zh")
		if err != nil {
			t.Errorf("zh 文章 %s: %v", slug, err)
			continue
		}
		en, err := svc.GetPublishedBySlugLang(ctx, "article", slug+"-en", "en")
		if err != nil {
			t.Errorf("en 翻译 %s: %v", slug, err)
			continue
		}
		if zh.Content.ContentID != en.Content.ContentID {
			t.Errorf("zh/en 应同翻译组: %q vs %q", zh.Content.ContentID, en.Content.ContentID)
		}
		if zh.Fields["category"] == "" {
			t.Errorf("zh %s 缺 category", slug)
		}
		if en.Fields["category"] == "" {
			t.Errorf("en %s 缺 category", slug)
		}
	}
	// main 菜单（zh）已建
	menus, err := st.MenuRepo().ListByLang(ctx, "zh")
	if err != nil || len(menus) == 0 {
		t.Errorf("zh main 菜单缺失: %v", err)
	}
	// 产品下子分类存在：斩拌机(chopper)、香肠机(sausage-machine)、拌馅机(mixer)，ParentID == 产品id
	prod, err := st.CategoryRepo().GetBySlug(ctx, "products")
	if err != nil {
		t.Fatalf("products 分类缺失: %v", err)
	}
	for _, slug := range []string{"chopper", "sausage-machine", "mixer"} {
		sc, err := st.CategoryRepo().GetBySlug(ctx, slug)
		if err != nil {
			t.Errorf("子分类 %s 缺失: %v", slug, err)
			continue
		}
		if sc.ParentID != prod.ID {
			t.Errorf("子分类 %s ParentID = %d, want %d", slug, sc.ParentID, prod.ID)
		}
	}
	// 每子分类 1 篇双语文章（zh 已发布 + en 同组翻译），category == 子分类 id
	for _, slug := range []string{"chopper-1", "sausage-machine-1", "mixer-1"} {
		zh, err := svc.GetPublishedBySlugLang(ctx, "article", slug, "zh")
		if err != nil {
			t.Errorf("zh 子分类文章 %s: %v", slug, err)
			continue
		}
		en, err := svc.GetPublishedBySlugLang(ctx, "article", slug+"-en", "en")
		if err != nil {
			t.Errorf("en 子分类翻译 %s: %v", slug, err)
			continue
		}
		if zh.Content.ContentID != en.Content.ContentID {
			t.Errorf("子分类 zh/en 应同翻译组: %q vs %q", zh.Content.ContentID, en.Content.ContentID)
		}
		sub, err := st.CategoryRepo().GetBySlug(ctx, strings.TrimSuffix(slug, "-1"))
		if err != nil {
			t.Errorf("子分类 %s 缺失: %v", slug, err)
			continue
		}
		if zh.Fields["category"] != fmt.Sprintf("%d", sub.ID) {
			t.Errorf("zh %s category = %q, want %d", slug, zh.Fields["category"], sub.ID)
		}
	}
	// main 菜单（zh/en）含子分类项 /category/products/chopper
	for _, lang := range []string{"zh", "en"} {
		mMenus, err := st.MenuRepo().ListByLang(ctx, lang)
		if err != nil {
			t.Errorf("main 菜单(%s) 缺失: %v", lang, err)
			continue
		}
		var items []store.MenuItem
		for _, m := range mMenus {
			if m.Name == "main" {
				json.Unmarshal([]byte(m.Items), &items)
			}
		}
		if !menuHasChild(items, "/category/products/chopper") {
			t.Errorf("main 菜单(%s) 缺子分类项 /category/products/chopper", lang)
		}
		if !menuHasChild(items, "/category/products/mixer") {
			t.Errorf("main 菜单(%s) 缺子分类项 /category/products/mixer", lang)
		}
	}
	// hello-zh/en 归入 news 分类
	for _, h := range []struct{ slug, lang string }{{"hello-zh", "zh"}, {"hello-en", "en"}} {
		e, err := svc.GetPublishedBySlugLang(ctx, "article", h.slug, h.lang)
		if err != nil {
			t.Errorf("%s 迁移后应可读: %v", h.slug, err)
			continue
		}
		if e.Fields["category"] == "" {
			t.Errorf("%s 应归入 news 分类", h.slug)
		}
	}
	// 幂等：二次 Run 不报错、数量不变
	if err := Run(ctx, st, svc, mock); err != nil {
		t.Errorf("二次 Run 应幂等: %v", err)
	}
	cats2, _ := st.CategoryRepo().List(ctx)
	if len(cats2) != 6 {
		t.Errorf("幂等后分类数 = %d, want 6", len(cats2))
	}
}

func TestSeedCategoryNameEnBackfill(t *testing.T) {
	t.Setenv("DULIZHAN_SEED_NO_DOWNLOAD", "1")
	ctx := context.Background()
	st, _ := sqlite.Open(":memory:")
	reg := schema.NewRegistry()
	svc := content.New(st, reg, []string{"zh", "en"})
	// 先跑一次 seed 建出分类，再把 news 的 name_en 清空模拟旧库
	if err := Run(ctx, st, svc, &mockMediaStore{}); err != nil {
		t.Fatal(err)
	}
	news, err := st.CategoryRepo().GetBySlug(ctx, "news")
	if err != nil {
		t.Fatal(err)
	}
	news.NameEn = ""
	if err := st.CategoryRepo().Update(ctx, &news); err != nil {
		t.Fatal(err)
	}
	// 再跑 seed，应回填 name_en=News（幂等）
	if err := Run(ctx, st, svc, &mockMediaStore{}); err != nil {
		t.Fatal(err)
	}
	cat, err := st.CategoryRepo().GetBySlug(ctx, "news")
	if err != nil {
		t.Fatalf("GetBySlug news: %v", err)
	}
	if cat.NameEn != "News" {
		t.Errorf("news name_en = %q, want News", cat.NameEn)
	}
}

// menuHasChild 判断菜单项 children 中是否含指定 URL。
func menuHasChild(items []store.MenuItem, url string) bool {
	for _, it := range items {
		for _, ch := range it.Children {
			if ch.URL == url {
				return true
			}
		}
	}
	return false
}
