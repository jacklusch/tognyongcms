package seed

import (
	"context"
	"strings"
	"testing"
	"time"

	"dulizhan/internal/auth"
	"dulizhan/internal/content"
	"dulizhan/internal/schema"
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
	// 分类建了 3 个
	cats, err := st.CategoryRepo().List(ctx)
	if err != nil || len(cats) != 3 {
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
	if len(cats2) != 3 {
		t.Errorf("幂等后分类数 = %d, want 3", len(cats2))
	}
}
