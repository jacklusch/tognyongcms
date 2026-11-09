package content

import (
	"context"
	"errors"
	"testing"

	"dulizhan/internal/config"
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
			{Name: "title", Label: "标题", Type: schema.TypeText, Required: true, Indexed: true, Translatable: true},
			{Name: "slug", Label: "别名", Type: schema.TypeSlug},
			{Name: "content", Label: "正文", Type: schema.TypeRichText, Translatable: true},
			{Name: "excerpt", Label: "摘要", Type: schema.TypeTextarea, Translatable: true},
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

// 纯中文标题且未填 slug 时，自动生成的 slug 不应为空（需兜底，否则前台链接失效）。
func TestCreateChineseTitleAutoSlug(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)

	e, err := svc.Create(ctx, "article", "zh", map[string]any{
		"title":   "新建文章",
		"content": "<p>正文</p>",
	}, 1)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if e.Content.Slug == "" {
		t.Error("纯中文标题自动生成 slug 为空，应兜底为合法 slug")
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

// mockTranslator 固定返回翻译文本；TranslateRichText 模拟 html.Render 输出的 <html>/<head>/<body> 外壳，
// 用于验证 EnsureTranslation 落库前 stripHTMLShell 剥离外壳。
type mockTranslator struct{ translated string }

func (m *mockTranslator) TranslateText(ctx context.Context, text, source, target string) (string, error) {
	if m.translated == "" {
		return "", errors.New("翻译失败")
	}
	return m.translated, nil
}

func (m *mockTranslator) TranslateRichText(ctx context.Context, htmlStr, source, target string) (string, error) {
	if m.translated == "" {
		return "", errors.New("翻译失败")
	}
	return "<html><head><title>x</title></head><body><p>" + m.translated + "</p></body></html>", nil
}

func TestEnsureTranslationCreatesEn(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	// 注入翻译器（enabled）
	cfg := &config.TranslateConfig{Enabled: true, SourceLang: "zh", TargetLang: "en"}
	svc.SetTranslator(&mockTranslator{translated: "EN-TRANSLATED"}, cfg)

	e, err := svc.Create(ctx, "article", "zh", map[string]any{
		"title":   "你好世界",
		"slug":    "hello",
		"content": "<p>正文内容</p>",
	}, 1)
	if err != nil {
		t.Fatal(err)
	}
	// 发布 zh，验证 en 状态跟随 zh（published），前台可查
	if err := svc.SetStatus(ctx, e.Content.ID, "published"); err != nil {
		t.Fatal(err)
	}
	st, err := svc.EnsureTranslation(ctx, "article", "zh", e.Content.ContentID, e.Fields, Actor{UserID: 1, IsModerator: true})
	if err != nil {
		t.Fatalf("EnsureTranslation: %v", err)
	}
	if !st.Triggered || !st.Created || st.Status != "translated" {
		t.Errorf("status = %+v", st)
	}
	// en 已生成
	en, err := svc.GetPublishedBySlugLang(ctx, "article", "hello", "en")
	if err != nil {
		t.Fatalf("en 未生成: %v", err)
	}
	if en.Content.Title != "EN-TRANSLATED" {
		t.Errorf("en title = %q", en.Content.Title)
	}
	// 富文本翻译结果已剥离 html.Render 外壳，只剩 body 内内容
	if en.Fields["content"] != "<p>EN-TRANSLATED</p>" {
		t.Errorf("en content = %v", en.Fields["content"])
	}
	// 非可翻译字段（slug）原样复制
	if en.Fields["slug"] != "hello" {
		t.Errorf("en slug = %v", en.Fields["slug"])
	}
	// 正常翻译路径不得写内部 fallback 标记
	if _, ok := en.Fields["_auto_translate_fallback"]; ok {
		t.Errorf("正常翻译 en 不应有 _auto_translate_fallback 标记, fields = %v", en.Fields)
	}
}

func TestEnsureTranslationDisabled(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	// 未注入翻译器 → Triggered:false
	e, _ := svc.Create(ctx, "article", "zh", map[string]any{"title": "x", "slug": "a", "content": "<p>x</p>"}, 1)
	st, err := svc.EnsureTranslation(ctx, "article", "zh", e.Content.ContentID, e.Fields, Actor{UserID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if st.Triggered {
		t.Error("未注入翻译器应 Triggered:false")
	}
}

func TestEnsureTranslationFallback(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	// 注入翻译器（enabled）但 translated 为空 → TranslateText/TranslateRichText 返回错误
	cfg := &config.TranslateConfig{Enabled: true, SourceLang: "zh", TargetLang: "en"}
	svc.SetTranslator(&mockTranslator{translated: ""}, cfg)

	e, err := svc.Create(ctx, "article", "zh", map[string]any{
		"title":   "你好世界",
		"slug":    "hello",
		"content": "<p>正文内容</p>",
	}, 1)
	if err != nil {
		t.Fatal(err)
	}
	// 发布 zh
	if err := svc.SetStatus(ctx, e.Content.ID, "published"); err != nil {
		t.Fatal(err)
	}
	st, err := svc.EnsureTranslation(ctx, "article", "zh", e.Content.ContentID, e.Fields, Actor{UserID: 1, IsModerator: true})
	if err != nil {
		t.Fatalf("EnsureTranslation: %v", err)
	}
	if !st.Triggered || !st.Created || st.Status != "fallback" {
		t.Errorf("status = %+v", st)
	}
	// en 已生成（降级复制 zh 字段），但强制 draft → 前台查不到
	if _, err := svc.GetPublishedBySlugLang(ctx, "article", "hello", "en"); err != errs.ErrNotFound {
		t.Errorf("fallback en 应强制 draft（前台查不到）, got %v", err)
	}
	entries, err := svc.ListByContentID(ctx, e.Content.ContentID)
	if err != nil {
		t.Fatal(err)
	}
	var en *Entry
	for i := range entries {
		if entries[i].Content.Lang == "en" {
			en = &entries[i]
			break
		}
	}
	if en == nil {
		t.Fatal("en 未生成")
	}
	if en.Content.Status != "draft" {
		t.Errorf("en status = %q, want draft", en.Content.Status)
	}
	// en 字段复制了 zh 内容
	if en.Content.Title != "你好世界" {
		t.Errorf("en title = %q", en.Content.Title)
	}
	if en.Fields["content"] != "<p>正文内容</p>" {
		t.Errorf("en content = %v", en.Fields["content"])
	}
	if en.Fields["slug"] != "hello" {
		t.Errorf("en slug = %v", en.Fields["slug"])
	}
	// fallback 降级草稿带内部标记，syncTranslationStatus 据此跳过状态同步
	fb, ok := en.Fields["_auto_translate_fallback"].(bool)
	if !ok || !fb {
		t.Errorf("fallback en 应带 _auto_translate_fallback 标记, fields = %v", en.Fields)
	}
}

// 发现 6：已存在 en 时再次 EnsureTranslation 走覆盖路径（Created=false、内容更新）。
func TestEnsureTranslationOverwritesExistingEn(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	cfg := &config.TranslateConfig{Enabled: true, SourceLang: "zh", TargetLang: "en"}
	actor := Actor{UserID: 1, IsModerator: true}

	svc.SetTranslator(&mockTranslator{translated: "EN-V1"}, cfg)
	e, err := svc.Create(ctx, "article", "zh", map[string]any{
		"title":   "第一版",
		"slug":    "hello",
		"content": "<p>正文一</p>",
	}, 1)
	if err != nil {
		t.Fatal(err)
	}
	st, err := svc.EnsureTranslation(ctx, "article", "zh", e.Content.ContentID, e.Fields, actor)
	if err != nil || !st.Triggered || !st.Created {
		t.Fatalf("首次 EnsureTranslation: %+v err=%v", st, err)
	}

	// 改 zh 后再翻译（覆盖）
	upd, err := svc.Update(ctx, e.Content.ID, map[string]any{"title": "第二版", "slug": "hello", "content": "<p>正文二</p>"})
	if err != nil {
		t.Fatal(err)
	}
	svc.SetTranslator(&mockTranslator{translated: "EN-V2"}, cfg)
	st2, err := svc.EnsureTranslation(ctx, "article", "zh", e.Content.ContentID, upd.Fields, actor)
	if err != nil {
		t.Fatalf("二次 EnsureTranslation: %v", err)
	}
	if !st2.Triggered || st2.Created {
		t.Errorf("二次 status = %+v, want Triggered=true/Created=false", st2)
	}
	entries, err := svc.ListByContentID(ctx, e.Content.ContentID)
	if err != nil {
		t.Fatal(err)
	}
	var en *Entry
	for i := range entries {
		if entries[i].Content.Lang == "en" {
			en = &entries[i]
			break
		}
	}
	if en == nil {
		t.Fatal("en 未生成")
	}
	if en.Content.Title != "EN-V2" {
		t.Errorf("en title = %q, want EN-V2（覆盖更新）", en.Content.Title)
	}
	if en.Fields["content"] != "<p>EN-V2</p>" {
		t.Errorf("en content = %v, want <p>EN-V2</p>", en.Fields["content"])
	}
}

// 发现 6：非源语言调用 EnsureTranslation 不触发。
func TestEnsureTranslationNotSourceLang(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	cfg := &config.TranslateConfig{Enabled: true, SourceLang: "zh", TargetLang: "en"}
	svc.SetTranslator(&mockTranslator{translated: "X"}, cfg)

	e, err := svc.Create(ctx, "article", "en", map[string]any{"title": "Hello", "slug": "hi", "content": "<p>hi</p>"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	st, err := svc.EnsureTranslation(ctx, "article", "en", e.Content.ContentID, e.Fields, Actor{UserID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if st.Triggered {
		t.Error("非源语言应 Triggered:false")
	}
}

// 发现 6：zh 发布后再保存（EnsureTranslation 重跑），en 状态跟随 published、前台可查。
func TestEnsureTranslationPublishFollows(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	cfg := &config.TranslateConfig{Enabled: true, SourceLang: "zh", TargetLang: "en"}
	svc.SetTranslator(&mockTranslator{translated: "EN"}, cfg)
	actor := Actor{UserID: 1, IsModerator: true}

	e, err := svc.Create(ctx, "article", "zh", map[string]any{
		"title":   "标题",
		"slug":    "hello",
		"content": "<p>正文</p>",
	}, 1)
	if err != nil {
		t.Fatal(err)
	}
	// 首次保存：en 创建并跟随 zh（draft），前台不可查
	if _, err := svc.EnsureTranslation(ctx, "article", "zh", e.Content.ContentID, e.Fields, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetPublishedBySlugLang(ctx, "article", "hello", "en"); err != errs.ErrNotFound {
		t.Errorf("首次保存 en 应 draft（前台查不到）, got %v", err)
	}
	// 发布 zh
	if err := svc.SetStatus(ctx, e.Content.ID, "published"); err != nil {
		t.Fatal(err)
	}
	// 发布后再次保存 zh → EnsureTranslation 重跑，en 状态跟随 published
	if _, err := svc.EnsureTranslation(ctx, "article", "zh", e.Content.ContentID, e.Fields, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetPublishedBySlugLang(ctx, "article", "hello", "en"); err != nil {
		t.Errorf("zh 发布后 en 应 published 可查: %v", err)
	}
}
