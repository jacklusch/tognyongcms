package content

import (
	"context"
	"errors"
	"testing"

	"dulizhan/internal/errs"
	"dulizhan/internal/schema"
	"dulizhan/internal/store"
	"dulizhan/internal/store/sqlite"
)

func newTestService(t *testing.T) (*Service, store.Store) {
	t.Helper()
	st, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	reg := schema.NewRegistry()
	svc := New(st, reg, []string{"zh", "en"}) // reservedNames

	article := schema.ContentType{
		Name:  "article",
		Label: "文章",
		Fields: []schema.Field{
			{Name: "title", Label: "标题", Type: schema.TypeText, Required: true, Indexed: true},
			{Name: "slug", Label: "别名", Type: schema.TypeSlug},
			{Name: "content", Label: "正文", Type: schema.TypeRichText},
			{Name: "excerpt", Label: "摘要", Type: schema.TypeTextarea},
		},
	}
	if err := svc.CreateType(context.Background(), &article); err != nil {
		t.Fatal(err)
	}
	return svc, st
}

func TestCreateAndGetPublished(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)

	e, err := svc.Create(ctx, "article", "zh", map[string]any{
		"title":   "你好世界",
		"slug":    "hello",
		"content": "<p>正文</p>",
	}, 1)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// 未发布，前台查不到
	if _, err := svc.GetPublishedBySlugLang(ctx, "article", "hello", "zh"); err != errs.ErrNotFound {
		t.Errorf("draft 应查不到, got %v", err)
	}

	if err := svc.SetStatus(ctx, e.Content.ID, "published"); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	got, err := svc.GetPublishedBySlugLang(ctx, "article", "hello", "zh")
	if err != nil {
		t.Fatalf("GetPublishedBySlugLang: %v", err)
	}
	if got.Content.Title != "你好世界" {
		t.Errorf("Title = %q", got.Content.Title)
	}
	if got.Fields["content"] != "<p>正文</p>" {
		t.Errorf("content field = %v", got.Fields["content"])
	}
	if got.TypeName != "article" {
		t.Errorf("TypeName = %q", got.TypeName)
	}
}

func TestCreateMissingRequired(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	if _, err := svc.Create(ctx, "article", "zh", map[string]any{}, 1); !errors.Is(err, errs.ErrValidation) {
		t.Errorf("缺必填应返回 ErrValidation, got %v", err)
	}
}

func TestCreateUnknownType(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	if _, err := svc.Create(ctx, "nope", "zh", map[string]any{}, 1); err != errs.ErrNotFound {
		t.Errorf("未知类型应返回 ErrNotFound, got %v", err)
	}
}

func TestListPublished(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	for i, slug := range []string{"a", "b", "c"} {
		e, err := svc.Create(ctx, "article", "zh", map[string]any{"title": "文章" + slug, "slug": slug}, 1)
		if err != nil {
			t.Fatal(err)
		}
		if i%2 == 0 {
			if err := svc.SetStatus(ctx, e.Content.ID, "published"); err != nil {
				t.Fatal(err)
			}
		}
	}
	items, total, err := svc.ListPublished(ctx, "article", "zh", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(items) != 2 {
		t.Errorf("len = %d, want 2", len(items))
	}
}

func TestListPublishedByCategories(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	// 两篇分类 5、一篇分类 8、一篇分类 6、一篇无分类
	for _, c := range []map[string]any{
		{"title": "甲", "slug": "a", "category": "5"},
		{"title": "乙", "slug": "b", "category": "5"},
		{"title": "丙", "slug": "c", "category": "8"},
		{"title": "丁", "slug": "d", "category": "6"},
		{"title": "戊", "slug": "e"},
	} {
		e, err := svc.Create(ctx, "article", "zh", c, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := svc.SetStatus(ctx, e.Content.ID, "published"); err != nil {
			t.Fatal(err)
		}
	}
	// 多 id 聚合：分类 5 + 8 → 3 篇
	items, total, err := svc.ListPublishedByCategories(ctx, "article", "zh", []int64{5, 8}, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(items) != 3 {
		t.Errorf("len = %d, want 3", len(items))
	}
	// 空 ids → 0 条
	items, total, err = svc.ListPublishedByCategories(ctx, "article", "zh", []int64{}, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(items) != 0 {
		t.Errorf("空 ids: total=%d len=%d, want 0/0", total, len(items))
	}
	// 未知类型 → ErrNotFound
	if _, _, err := svc.ListPublishedByCategories(ctx, "nope", "zh", []int64{5}, 1, 10); err != errs.ErrNotFound {
		t.Errorf("未知类型 = %v, want ErrNotFound", err)
	}
}

func TestUpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	e, err := svc.Create(ctx, "article", "zh", map[string]any{"title": "t1"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	upd, err := svc.Update(ctx, e.Content.ID, map[string]any{"title": "t2", "content": "c2"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if upd.Content.Title != "t2" {
		t.Errorf("Title = %q, want t2", upd.Content.Title)
	}
	if upd.Content.ContentID == "" {
		t.Error("Update 后 ContentID 不应为空")
	}
	if err := svc.Delete(ctx, e.Content.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.GetPublishedBySlugLang(ctx, "article", "t1", "zh"); err != errs.ErrNotFound {
		t.Errorf("删除后应查不到, got %v", err)
	}
}

func TestCreateTypeReservedName(t *testing.T) {
	ctx := context.Background()
	svc, st := newTestService(t)
	ct := &schema.ContentType{Name: "zh", Label: "冲突", Fields: []schema.Field{{Name: "t", Type: schema.TypeText}}}
	if err := svc.CreateType(ctx, ct); !errors.Is(err, errs.ErrValidation) {
		t.Errorf("创建 zh 类型 = %v, want ErrValidation", err)
	}
	// 未落库
	if _, err := st.ContentTypeRepo().GetByName(ctx, "zh"); err != errs.ErrNotFound {
		t.Errorf("冲突类型不应落库, got %v", err)
	}
}

func TestSearch(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	for _, c := range []map[string]any{
		{"title": "你好世界", "slug": "hello"},
		{"title": "另一篇", "slug": "other"},
	} {
		if _, err := svc.Create(ctx, "article", "zh", c, 1); err != nil {
			t.Fatal(err)
		}
	}
	items, total, err := svc.Search(ctx, "article", "zh", "世界", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || items[0].Content.Slug != "hello" {
		t.Errorf("Search = %d items, total=%d, %+v", len(items), total, items)
	}
	// 未知类型 → ErrNotFound
	if _, _, err := svc.Search(ctx, "nope", "zh", "x", 1, 10); err != errs.ErrNotFound {
		t.Errorf("未知类型 = %v, want ErrNotFound", err)
	}
}

type typeStat struct {
	TypeName  string `json:"type_name"`
	Published int    `json:"published"`
	Draft     int    `json:"draft"`
}

type statsResult struct {
	ByType []typeStat `json:"by_type"`
	Recent []Entry    `json:"recent"`
}

func TestStats(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	e1, err := svc.Create(ctx, "article", "zh", map[string]any{"title": "a", "slug": "a"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	svc.Create(ctx, "article", "zh", map[string]any{"title": "b", "slug": "b"}, 1)
	if err := svc.SetStatus(ctx, e1.Content.ID, "published"); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.ByType) != 1 || st.ByType[0].TypeName != "article" {
		t.Fatalf("ByType = %+v", st.ByType)
	}
	if st.ByType[0].Published != 1 || st.ByType[0].Draft != 1 {
		t.Errorf("stats = %+v", st.ByType[0])
	}
	if len(st.Recent) == 0 {
		t.Error("recent 为空")
	}
}

func TestDeleteTypeCascades(t *testing.T) {
	ctx := context.Background()
	svc, st := newTestService(t)
	e, err := svc.Create(ctx, "article", "zh", map[string]any{"title": "a", "slug": "a"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteType(ctx, "article"); err != nil {
		t.Fatal(err)
	}
	// 内容已被删
	if _, err := st.ContentRepo().GetByID(ctx, e.Content.ID); err != errs.ErrNotFound {
		t.Errorf("级联删除后内容应不存在, got %v", err)
	}
	// 类型已被删
	if _, err := st.ContentTypeRepo().GetByName(ctx, "article"); err != errs.ErrNotFound {
		t.Errorf("级联删除后类型应不存在, got %v", err)
	}
	// 删不存在的类型 → ErrNotFound
	if err := svc.DeleteType(ctx, "nope"); err != errs.ErrNotFound {
		t.Errorf("删不存在类型 = %v, want ErrNotFound", err)
	}
}

func TestTranslationOwnership(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	moderator := Actor{UserID: 99, IsModerator: true}

	e, err := svc.Create(ctx, "article", "zh", map[string]any{"title": "原", "slug": "orig"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	// 作者给自己翻译 OK
	tr, err := svc.CreateTranslation(ctx, "article", "en", e.Content.ContentID, map[string]any{"title": "En"}, Actor{UserID: 1})
	if err != nil {
		t.Fatalf("作者自译应成功: %v", err)
	}
	if tr.Content.Lang != "en" {
		t.Errorf("lang = %q", tr.Content.Lang)
	}
	if tr.Content.Title != "En" {
		t.Errorf("翻译行 Title = %q, want En", tr.Content.Title)
	}
	// 作者翻译他人内容 → Forbidden
	if _, err := svc.CreateTranslation(ctx, "article", "fr", e.Content.ContentID, map[string]any{"title": "Fr"}, Actor{UserID: 2}); err != errs.ErrForbidden {
		t.Errorf("作者翻译他人 = %v, want ErrForbidden", err)
	}
	// moderator 翻译他人 OK
	if _, err := svc.CreateTranslation(ctx, "article", "fr", e.Content.ContentID, map[string]any{"title": "Fr"}, moderator); err != nil {
		t.Errorf("moderator 翻译他人 = %v", err)
	}
	// 翻译不存在的组 → NotFound
	if _, err := svc.CreateTranslation(ctx, "article", "de", "nope", map[string]any{"title": "De"}, moderator); err != errs.ErrNotFound {
		t.Errorf("不存在组 = %v, want ErrNotFound", err)
	}
}
