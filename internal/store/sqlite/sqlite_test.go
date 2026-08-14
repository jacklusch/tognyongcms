package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dulizhan/internal/content"
	"dulizhan/internal/errs"
	"dulizhan/internal/i18n"
	"dulizhan/internal/media"
	schemareg "dulizhan/internal/schema"
	"dulizhan/internal/seed"
	"dulizhan/internal/store"
)

func newTestStore(t *testing.T) store.Store {
	t.Helper()
	st, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return st
}

func TestContentTypeRepoCRUD(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	repo := st.ContentTypeRepo()

	ct := &store.ContentType{Name: "article", Label: "文章", Fields: `[{"name":"title","type":"text"}]`}
	if err := repo.Create(ctx, ct); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if ct.ID == 0 {
		t.Error("Create 未回填 ID")
	}

	got, err := repo.GetByName(ctx, "article")
	if err != nil {
		t.Fatalf("GetByName: %v", err)
	}
	if got.Label != "文章" {
		t.Errorf("Label = %q", got.Label)
	}

	if _, err := repo.GetByName(ctx, "missing"); err != errs.ErrNotFound {
		t.Errorf("GetByName missing = %v, want errs.ErrNotFound", err)
	}

	if err := repo.Create(ctx, &store.ContentType{Name: "article"}); err == nil {
		t.Error("重复 name 应报错")
	}
}

func TestWithTxRollbackAndAssert(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	repo := st.ContentTypeRepo()
	ct := &store.ContentType{Name: "article", Label: "文章", Fields: `[]`}
	if err := repo.Create(ctx, ct); err != nil {
		t.Fatal(err)
	}
	// I3：fn 返回错误 → 事务回滚，数据未变
	err := st.WithTx(ctx, func(tx store.Store) error {
		if err := tx.ContentTypeRepo().Delete(ctx, ct.ID); err != nil {
			return err
		}
		return errors.New("模拟失败")
	})
	if err == nil {
		t.Fatal("WithTx 应返回 fn 的错误")
	}
	if _, err := repo.GetByName(ctx, "article"); err != nil {
		t.Errorf("回滚后类型应仍存在: %v", err)
	}
	// 成功路径提交
	if err := st.WithTx(ctx, func(tx store.Store) error {
		return tx.ContentTypeRepo().Delete(ctx, ct.ID)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByName(ctx, "article"); err != errs.ErrNotFound {
		t.Errorf("提交后类型应被删除, got %v", err)
	}
	// I3：断言安全——事务 Store 上再开 WithTx 应返回错误而非 panic
	err = st.WithTx(ctx, func(tx store.Store) error {
		return tx.WithTx(ctx, func(tx2 store.Store) error { return nil })
	})
	if err == nil {
		t.Error("tx Store 上调用 WithTx 应报错")
	}
}

func TestContentTypeDuplicateName(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	repo := st.ContentTypeRepo()
	if err := repo.Create(ctx, &store.ContentType{Name: "article"}); err != nil {
		t.Fatal(err)
	}
	// 重复 name（UNIQUE 冲突）→ ErrValidation
	if err := repo.Create(ctx, &store.ContentType{Name: "article"}); !errors.Is(err, errs.ErrValidation) {
		t.Errorf("重复类型名 = %v, want ErrValidation", err)
	}
}

func TestContentRepoCRUD(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	types := st.ContentTypeRepo()
	ct := &store.ContentType{Name: "article", Label: "文章", Fields: `[]`}
	if err := types.Create(ctx, ct); err != nil {
		t.Fatal(err)
	}
	repo := st.ContentRepo()

	publishedAt := time.Now().Add(-time.Hour).UTC()
	c := &store.Content{
		ContentTypeID: ct.ID,
		ContentID:     "grp-1",
		Lang:          "zh",
		Slug:          "hello",
		Title:         "你好",
		Status:        "published",
		CreatedBy:     1,
		PublishedAt:   &publishedAt,
		Payload:       `{"title":"你好"}`,
	}
	if err := repo.Create(ctx, c); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.ID == 0 {
		t.Error("Create 未回填 ID")
	}

	got, err := repo.GetBySlugLangStatus(ctx, "article", "hello", "zh", "published")
	if err != nil {
		t.Fatalf("GetBySlugLangStatus: %v", err)
	}
	if got.Title != "你好" {
		t.Errorf("Title = %q", got.Title)
	}

	// 未发布的不应被查出来
	if err := repo.Create(ctx, &store.Content{ContentTypeID: ct.ID, ContentID: "g2", Lang: "zh", Slug: "draft-one", Status: "draft"}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetBySlugLangStatus(ctx, "article", "draft-one", "zh", "published"); err != errs.ErrNotFound {
		t.Errorf("draft 应查不到, got %v", err)
	}

	// 分页
	if err := repo.Create(ctx, &store.Content{ContentTypeID: ct.ID, ContentID: "g3", Lang: "zh", Slug: "b", Status: "published"}); err != nil {
		t.Fatal(err)
	}
	list, err := repo.ListByTypeLangStatus(ctx, "article", "zh", "published", 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Errorf("len = %d, want 1", len(list))
	}
	n, err := repo.CountByTypeLangStatus(ctx, "article", "zh", "published")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("count = %d, want 2", n)
	}
}

func TestListByTypeLangStatusEmptyFilter(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	types := st.ContentTypeRepo()
	ct := &store.ContentType{Name: "article", Label: "文章", Fields: `[]`}
	if err := types.Create(ctx, ct); err != nil {
		t.Fatal(err)
	}
	repo := st.ContentRepo()
	for _, c := range []*store.Content{
		{ContentTypeID: ct.ID, ContentID: "g1", Lang: "zh", Slug: "zh-pub", Status: "published"},
		{ContentTypeID: ct.ID, ContentID: "g2", Lang: "en", Slug: "en-draft", Status: "draft"},
	} {
		if err := repo.Create(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	// lang/status 空 → 不过滤，返回全部
	list, err := repo.ListByTypeLangStatus(ctx, "article", "", "", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Errorf("空过滤 list = %d, want 2", len(list))
	}
	// lang 空、status 限定 → 跨语言取发布
	list, err = repo.ListByTypeLangStatus(ctx, "article", "", "published", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Slug != "zh-pub" {
		t.Errorf("空 lang published list = %d, %+v", len(list), list)
	}
	n, err := repo.CountByTypeLangStatus(ctx, "article", "", "published")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("空 lang CountByTypeLangStatus = %d, want 1", n)
	}
	// CountByTypeStatus（不按 lang）
	n, err = repo.CountByTypeStatus(ctx, "article", "draft")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("CountByTypeStatus draft = %d, want 1", n)
	}
}

func TestContentSearch(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	types := st.ContentTypeRepo()
	ct := &store.ContentType{Name: "article", Label: "文章", Fields: `[]`}
	if err := types.Create(ctx, ct); err != nil {
		t.Fatal(err)
	}
	repo := st.ContentRepo()
	for _, c := range []*store.Content{
		{ContentTypeID: ct.ID, ContentID: "g1", Lang: "zh", Slug: "hello-world", Title: "你好世界", Status: "published"},
		{ContentTypeID: ct.ID, ContentID: "g2", Lang: "zh", Slug: "other", Title: "另一篇", Status: "draft"},
		{ContentTypeID: ct.ID, ContentID: "g3", Lang: "en", Slug: "en-hello", Title: "Hello", Status: "published"},
	} {
		if err := repo.Create(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	// 按 title LIKE 命中
	list, err := repo.SearchByTypeLang(ctx, "article", "zh", "世界", 0, 10)
	if err != nil || len(list) != 1 || list[0].Slug != "hello-world" {
		t.Errorf("Search title = %d items, %v", len(list), err)
	}
	n, err := repo.CountSearch(ctx, "article", "zh", "世界")
	if err != nil || n != 1 {
		t.Errorf("CountSearch = %d, %v", n, err)
	}
	// 按 slug LIKE 命中
	list, _ = repo.SearchByTypeLang(ctx, "article", "zh", "hello", 0, 10)
	if len(list) != 1 || list[0].Slug != "hello-world" {
		t.Errorf("Search slug = %d items", len(list))
	}
	// 无结果
	list, _ = repo.SearchByTypeLang(ctx, "article", "zh", "zzz", 0, 10)
	if len(list) != 0 {
		t.Errorf("Search 无结果应为空, got %d", len(list))
	}
}

func TestSettingRepo(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	repo := st.SettingRepo()
	if err := repo.Set(ctx, "site_name", "abc"); err != nil {
		t.Fatal(err)
	}
	v, err := repo.Get(ctx, "site_name")
	if err != nil || v != "abc" {
		t.Errorf("Get = %q, %v", v, err)
	}
	if _, err := repo.Get(ctx, "missing"); err != errs.ErrNotFound {
		t.Errorf("missing = %v, want errs.ErrNotFound", err)
	}
	all, err := repo.All(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if all["site_name"] != "abc" {
		t.Errorf("All = %v", all)
	}
}

func TestContentDuplicateSlug(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	types := st.ContentTypeRepo()
	ct := &store.ContentType{Name: "article", Label: "文章", Fields: `[]`}
	if err := types.Create(ctx, ct); err != nil {
		t.Fatal(err)
	}
	repo := st.ContentRepo()
	c := &store.Content{ContentTypeID: ct.ID, ContentID: "grp-1", Lang: "zh", Slug: "hello", Status: "published"}
	if err := repo.Create(ctx, c); err != nil {
		t.Fatal(err)
	}
	// 同 (lang, content_type_id, slug) 二次 Create → ErrValidation
	if err := repo.Create(ctx, &store.Content{ContentTypeID: ct.ID, ContentID: "grp-2", Lang: "zh", Slug: "hello", Status: "draft"}); !errors.Is(err, errs.ErrValidation) {
		t.Errorf("重复 slug = %v, want ErrValidation", err)
	}
	// 同组不同 slug 仍可创建
	if err := repo.Create(ctx, &store.Content{ContentTypeID: ct.ID, ContentID: "grp-2", Lang: "zh", Slug: "other", Status: "draft"}); err != nil {
		t.Errorf("同组不同 slug 应成功: %v", err)
	}
}

func TestContentUpdateAndGetByID(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	types := st.ContentTypeRepo()
	ct := &store.ContentType{Name: "article", Label: "文章", Fields: `[]`}
	types.Create(ctx, ct)
	repo := st.ContentRepo()
	c := &store.Content{ContentTypeID: ct.ID, ContentID: "g", Lang: "zh", Slug: "a", Title: "旧", Status: "draft"}
	if err := repo.Create(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(ctx, c.ID)
	if err != nil || got.Title != "旧" {
		t.Fatalf("GetByID = %+v, %v", got, err)
	}
	// Update 更新 content_id/content_type_id
	c.Title = "新"
	c.ContentID = "g2"
	if err := repo.Update(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.GetByID(ctx, c.ID)
	if got.Title != "新" || got.ContentID != "g2" {
		t.Errorf("Update 后 = %+v", got)
	}
	if err := repo.Delete(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByID(ctx, c.ID); err != errs.ErrNotFound {
		t.Errorf("删除后 = %v, want ErrNotFound", err)
	}
}

func TestSettingUpsert(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	repo := st.SettingRepo()
	if err := repo.Set(ctx, "k", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Set(ctx, "k", "v2"); err != nil { // upsert 覆盖
		t.Fatal(err)
	}
	v, err := repo.Get(ctx, "k")
	if err != nil || v != "v2" {
		t.Errorf("Get = %q, %v", v, err)
	}
}

func TestUserRoleRepo(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	roles := st.RoleRepo()
	admin, err := roles.Create(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	got, err := roles.GetByID(ctx, admin.ID)
	if err != nil || got.Name != "admin" {
		t.Errorf("GetByID = %+v, %v", got, err)
	}
	if err := roles.Update(ctx, admin.ID, "管理员", `["content.read"]`); err != nil {
		t.Fatal(err)
	}
	got, _ = roles.GetByID(ctx, admin.ID)
	if got.Permissions != `["content.read"]` {
		t.Errorf("permissions = %q", got.Permissions)
	}
	if _, err := roles.GetByName(ctx, "missing"); err != errs.ErrNotFound {
		t.Errorf("missing role = %v", err)
	}

	users := st.UserRepo()
	u := &store.User{Username: "jack", PasswordHash: "h", RoleID: admin.ID}
	if err := users.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	if u.ID == 0 {
		t.Error("用户 ID 未回填")
	}
	gotU, err := users.GetByUsername(ctx, "jack")
	if err != nil || gotU.PasswordHash != "h" {
		t.Errorf("GetByUsername = %+v, %v", gotU, err)
	}
	if _, err := users.GetByUsername(ctx, "nobody"); err != errs.ErrNotFound {
		t.Errorf("missing user = %v", err)
	}
	// 唯一约束
	if err := users.Create(ctx, &store.User{Username: "jack"}); err == nil {
		t.Error("重复用户名应报错")
	}
	list, err := users.List(ctx)
	if err != nil || len(list) != 1 {
		t.Errorf("List = %v, %v", list, err)
	}
}

func TestSessionRepo(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	repo := st.SessionRepo()
	exp := time.Now().Add(time.Hour).UTC()
	if err := repo.Create(ctx, &store.Session{Token: "tok-1", UserID: 5, ExpiresAt: exp}); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, "tok-1")
	if err != nil || got.UserID != 5 {
		t.Errorf("Get = %+v, %v", got, err)
	}
	if _, err := repo.Get(ctx, "tok-x"); err != errs.ErrNotFound {
		t.Errorf("missing session = %v", err)
	}
	if err := repo.Delete(ctx, "tok-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, "tok-1"); err != errs.ErrNotFound {
		t.Errorf("删除后应查不到, %v", err)
	}
}

func TestMenuMediaRepo(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	menus := st.MenuRepo()
	if err := menus.Create(ctx, &store.Menu{Name: "主导航", Lang: "zh", Items: `[]`}); err != nil {
		t.Fatal(err)
	}
	ms, err := menus.ListByLang(ctx, "zh")
	if err != nil || len(ms) != 1 {
		t.Errorf("menus = %v, %v", ms, err)
	}

	med := st.MediaRepo()
	if err := med.Create(ctx, &store.Media{Filename: "a.png", URL: "/media/1.png", Mime: "image/png", Size: 100}); err != nil {
		t.Fatal(err)
	}
	list, err := med.List(ctx, 0, 10)
	if err != nil || len(list) != 1 {
		t.Errorf("media list = %v, %v", list, err)
	}
	if err := med.Delete(ctx, list[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := med.GetByID(ctx, list[0].ID); err != errs.ErrNotFound {
		t.Errorf("删除后应查不到, %v", err)
	}
}

func TestSessionExpiryRoundTrip(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	repo := st.SessionRepo()
	exp := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	if err := repo.Create(ctx, &store.Session{Token: "tok-r", UserID: 1, ExpiresAt: exp}); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, "tok-r")
	if err != nil {
		t.Fatal(err)
	}
	if !got.ExpiresAt.Equal(exp) {
		t.Errorf("ExpiresAt 往返 = %v, want %v", got.ExpiresAt, exp)
	}
	if err := repo.DeleteByUser(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, "tok-r"); err != errs.ErrNotFound {
		t.Errorf("DeleteByUser 后 = %v", err)
	}
}

func TestContentDeleteByType(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	types := st.ContentTypeRepo()
	ct := &store.ContentType{Name: "article", Label: "文章", Fields: `[]`}
	if err := types.Create(ctx, ct); err != nil {
		t.Fatal(err)
	}
	repo := st.ContentRepo()
	for _, c := range []*store.Content{
		{ContentTypeID: ct.ID, ContentID: "g1", Lang: "zh", Slug: "a", Status: "published"},
		{ContentTypeID: ct.ID, ContentID: "g2", Lang: "zh", Slug: "b", Status: "draft"},
	} {
		if err := repo.Create(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.DeleteByType(ctx, "article"); err != nil {
		t.Fatal(err)
	}
	n, _ := repo.CountByTypeLangStatus(ctx, "article", "zh", "")
	if n != 0 {
		t.Errorf("删类型后内容应清零, count=%d", n)
	}
}

func TestUserRoleMenuMediaRepoDetail(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	roles := st.RoleRepo()
	admin, _ := roles.Create(ctx, "admin")
	// RoleRepo.List
	list, err := roles.List(ctx)
	if err != nil || len(list) != 1 {
		t.Errorf("RoleRepo.List = %v, %v", list, err)
	}
	users := st.UserRepo()
	u := &store.User{Username: "u1", PasswordHash: "h", RoleID: admin.ID}
	if err := users.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	// UserRepo.GetByID
	got, err := users.GetByID(ctx, u.ID)
	if err != nil || got.Username != "u1" {
		t.Errorf("GetByID = %+v, %v", got, err)
	}
	menus := st.MenuRepo()
	m := &store.Menu{Name: "主导航", Lang: "zh", Items: `[{"label":"首页"}]`}
	if err := menus.Create(ctx, m); err != nil {
		t.Fatal(err)
	}
	// Menu GetByID/Update/Delete
	gotM, err := menus.GetByID(ctx, m.ID)
	if err != nil || gotM.Items == "" {
		t.Errorf("Menu GetByID = %+v, %v", gotM, err)
	}
	m.Items = `[{"label":"关于"}]`
	if err := menus.Update(ctx, m); err != nil {
		t.Fatal(err)
	}
	gotM, _ = menus.GetByID(ctx, m.ID)
	if !strings.Contains(gotM.Items, "关于") {
		t.Errorf("Menu Update 后 = %+v", gotM)
	}
	if err := menus.Delete(ctx, m.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := menus.GetByID(ctx, m.ID); err != errs.ErrNotFound {
		t.Errorf("Menu Delete 后 = %v", err)
	}
}

func TestMediaListEmptyIsNotNil(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	list, err := st.MediaRepo().List(ctx, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if list == nil {
		t.Error("空媒体列表应返回空切片而非 nil（JSON 输出 [] 而非 null）")
	}
	if len(list) != 0 {
		t.Errorf("len = %d, want 0", len(list))
	}
}

func TestContentListByCategories(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	types := st.ContentTypeRepo()
	ct := &store.ContentType{Name: "article", Label: "文章", Fields: `[]`}
	if err := types.Create(ctx, ct); err != nil {
		t.Fatal(err)
	}
	repo := st.ContentRepo()
	// 两篇已发布文章分别归属分类 5 和 8（payload 含 category:"5"/"8"）
	for _, c := range []*store.Content{
		{ContentTypeID: ct.ID, ContentID: "g1", Lang: "zh", Slug: "a", Title: "A", Status: "published", Payload: `{"category":"5","title":"A"}`},
		{ContentTypeID: ct.ID, ContentID: "g2", Lang: "zh", Slug: "b", Title: "B", Status: "published", Payload: `{"category":"8","title":"B"}`},
		{ContentTypeID: ct.ID, ContentID: "g3", Lang: "zh", Slug: "c", Title: "C", Status: "published", Payload: `{"category":"6","title":"C"}`},
	} {
		if err := repo.Create(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	// 多分类 IN 查询命中 5 和 8 两篇
	list, err := repo.ListByCategories(ctx, "article", "zh", []int64{5, 8}, 0, 10)
	if err != nil {
		t.Fatalf("ListByCategories: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("ListByCategories = %d items, want 2", len(list))
	}
	n, err := repo.CountByCategories(ctx, "article", "zh", []int64{5, 8})
	if err != nil {
		t.Fatalf("CountByCategories: %v", err)
	}
	if n != 2 {
		t.Errorf("CountByCategories = %d, want 2", n)
	}
	// 空 ids → 返回空（不报错）
	list, err = repo.ListByCategories(ctx, "article", "zh", []int64{}, 0, 10)
	if err != nil {
		t.Fatalf("ListByCategories empty ids: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("ListByCategories empty ids = %d items, want 0", len(list))
	}
	n, err = repo.CountByCategories(ctx, "article", "zh", []int64{})
	if err != nil {
		t.Fatalf("CountByCategories empty ids: %v", err)
	}
	if n != 0 {
		t.Errorf("CountByCategories empty ids = %d, want 0", n)
	}
	// 无命中分类
	list, err = repo.ListByCategories(ctx, "article", "zh", []int64{99}, 0, 10)
	if err != nil {
		t.Fatalf("ListByCategories no hit: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("ListByCategories no hit = %d items, want 0", len(list))
	}
}

func TestCategoryParentChildren(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	repo := st.CategoryRepo()

	mk := func(name, slug string, parent int64) *store.Category {
		c := &store.Category{Name: name, Slug: slug, ParentID: parent}
		if err := repo.Create(ctx, c); err != nil {
			t.Fatalf("Create %s: %v", name, err)
		}
		return c
	}
	a := mk("A", "a", 0)
	b := mk("B", "b", 0)
	a1 := mk("A1", "a1", a.ID)
	a1a := mk("A1a", "a1a", a1.ID)

	// 无 parent 的分类读回 ParentID==0
	got, err := repo.GetByID(ctx, a.ID)
	if err != nil || got.ParentID != 0 {
		t.Errorf("GetByID A ParentID = %d, %v, want 0", got.ParentID, err)
	}

	ids := func(cats []store.Category) []int64 {
		out := make([]int64, len(cats))
		for i, c := range cats {
			out[i] = c.ID
		}
		return out
	}

	// 直接子分类
	kids, err := repo.ListChildren(ctx, a.ID)
	if err != nil || len(kids) != 1 || kids[0].ID != a1.ID {
		t.Errorf("ListChildren(A) = %v, %v", ids(kids), err)
	}
	kids, err = repo.ListChildren(ctx, a1.ID)
	if err != nil || len(kids) != 1 || kids[0].ID != a1a.ID {
		t.Errorf("ListChildren(A1) = %v, %v", ids(kids), err)
	}
	kids, err = repo.ListChildren(ctx, b.ID)
	if err != nil || len(kids) != 0 {
		t.Errorf("ListChildren(B) = %v, %v, want empty", ids(kids), err)
	}

	// 递归子孙
	desc, err := repo.Descendants(ctx, a.ID)
	if err != nil || len(desc) != 2 {
		t.Fatalf("Descendants(A) = %v, %v, want [A1 A1a]", ids(desc), err)
	}
	want := []int64{a1.ID, a1a.ID} // ORDER BY id
	for i, id := range want {
		if desc[i].ID != id {
			t.Errorf("Descendants(A)[%d].ID = %d, want %d", i, desc[i].ID, id)
		}
	}
	desc, err = repo.Descendants(ctx, a1.ID)
	if err != nil || len(desc) != 1 || desc[0].ID != a1a.ID {
		t.Errorf("Descendants(A1) = %v, %v", ids(desc), err)
	}
	// 子分类读回 ParentID
	got, err = repo.GetByID(ctx, a1.ID)
	if err != nil || got.ParentID != a.ID {
		t.Errorf("GetByID A1 ParentID = %d, %v, want %d", got.ParentID, err, a.ID)
	}
	got, err = repo.GetByID(ctx, a1a.ID)
	if err != nil || got.ParentID != a1.ID {
		t.Errorf("GetByID A1a ParentID = %d, %v, want %d", got.ParentID, err, a1.ID)
	}
	// Update 保留 parent_id
	a1.Name = "A1 改"
	if err := repo.Update(ctx, a1); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.GetByID(ctx, a1.ID)
	if got.ParentID != a.ID {
		t.Errorf("Update 后 A1 ParentID = %d, want %d", got.ParentID, a.ID)
	}
}

func TestSchemaReopenAndParentColumn(t *testing.T) {
	// 首次 Open 建 schema，重复 Open（重新执行 schema + ALTER 守卫）不报错
	dir := t.TempDir()
	dsn := "file:" + filepath.Join(dir, "test.db")
	st1, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open#1: %v", err)
	}
	st2, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open#2（重复 schema）: %v", err)
	}
	st1.Close()
	st2.Close()
	// 验证 parent_id 列存在（直接查 PRAGMA）
	st3, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open#3: %v", err)
	}
	defer st3.Close()
	var cols int
	if err := st3.db.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM pragma_table_info('categories') WHERE name = 'parent_id'").Scan(&cols); err != nil {
		t.Fatalf("查询 parent_id 列: %v", err)
	}
	if cols != 1 {
		t.Errorf("categories 表缺 parent_id 列, COUNT=%d", cols)
	}
}

func TestCategoryRepo(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	repo := st.CategoryRepo()

	c := &store.Category{Name: "新闻", Slug: "news", Description: "新闻栏目"}
	if err := repo.Create(ctx, c); err != nil {
		t.Fatal(err)
	}
	if c.ID == 0 {
		t.Error("Create 未回填 ID")
	}
	got, err := repo.GetBySlug(ctx, "news")
	if err != nil || got.Name != "新闻" {
		t.Errorf("GetBySlug = %+v, %v", got, err)
	}
	// 更新
	c.Name = "要闻"
	if err := repo.Update(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.GetByID(ctx, c.ID)
	if got.Name != "要闻" {
		t.Errorf("Update 后 Name = %q", got.Name)
	}
	// 列表
	list, err := repo.List(ctx)
	if err != nil || len(list) != 1 {
		t.Errorf("List = %d, %v", len(list), err)
	}
	// 内容数（当前 0）
	n, err := repo.CountContent(ctx, c.ID)
	if err != nil || n != 0 {
		t.Errorf("CountContent = %d, %v", n, err)
	}
	// 删除
	if err := repo.Delete(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByID(ctx, c.ID); err != errs.ErrNotFound {
		t.Errorf("删除后 = %v", err)
	}
}

func TestCountContentByLang(t *testing.T) {
	ctx := context.Background()
	t.Setenv("DULIZHAN_SEED_NO_DOWNLOAD", "1")
	st := newTestStore(t)
	if _, err := i18n.New([]string{"zh", "en"}, "zh", false); err != nil {
		t.Fatal(err)
	}
	svc := content.New(st, schemareg.NewRegistry(), []string{"zh", "en"})
	med := media.NewLocalStore(t.TempDir(), "/media")
	if err := seed.Run(ctx, st, svc, med); err != nil {
		t.Fatal(err)
	}
	news, err := st.CategoryRepo().GetBySlug(ctx, "news")
	if err != nil {
		t.Fatal(err)
	}
	zh, err := st.CategoryRepo().CountContentByLang(ctx, news.ID, "zh")
	if err != nil {
		t.Fatal(err)
	}
	if zh != 3 {
		t.Errorf("news zh count = %d, want 3", zh)
	}
	en, err := st.CategoryRepo().CountContentByLang(ctx, news.ID, "en")
	if err != nil {
		t.Fatal(err)
	}
	if en != 3 {
		t.Errorf("news en count = %d, want 3", en)
	}
}

func TestListByTypeLangStatusCategory(t *testing.T) {
	ctx := context.Background()
	t.Setenv("DULIZHAN_SEED_NO_DOWNLOAD", "1")
	st := newTestStore(t)
	if _, err := i18n.New([]string{"zh", "en"}, "zh", false); err != nil {
		t.Fatal(err)
	}
	svc := content.New(st, schemareg.NewRegistry(), []string{"zh", "en"})
	med := media.NewLocalStore(t.TempDir(), "/media")
	if err := seed.Run(ctx, st, svc, med); err != nil {
		t.Fatal(err)
	}
	products, err := st.CategoryRepo().GetBySlug(ctx, "products")
	if err != nil {
		t.Fatal(err)
	}
	// products 分类 zh：products-1/products-2 两行（子分类 chopper/sausage-machine/mixer 不属于 products id）
	items, err := st.ContentRepo().ListByTypeLangStatusCategory(ctx, "article", "zh", "", products.ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Errorf("products zh rows = %d, want 2", len(items))
	}
	n, err := st.ContentRepo().CountByTypeLangStatusCategory(ctx, "article", "zh", "", products.ID)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("products zh count = %d, want 2", n)
	}
}

func TestSearchByTypeLangCategory(t *testing.T) {
	ctx := context.Background()
	t.Setenv("DULIZHAN_SEED_NO_DOWNLOAD", "1")
	st := newTestStore(t)
	if _, err := i18n.New([]string{"zh", "en"}, "zh", false); err != nil {
		t.Fatal(err)
	}
	svc := content.New(st, schemareg.NewRegistry(), []string{"zh", "en"})
	med := media.NewLocalStore(t.TempDir(), "/media")
	if err := seed.Run(ctx, st, svc, med); err != nil {
		t.Fatal(err)
	}
	products, err := st.CategoryRepo().GetBySlug(ctx, "products")
	if err != nil {
		t.Fatal(err)
	}
	// products 分类下 zh 搜 "品"（products-1"旗舰产品一览"/products-2"新品评测"标题命中"品"）→ 2 条
	items, err := st.ContentRepo().SearchByTypeLangCategory(ctx, "article", "zh", "品", products.ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Errorf("products zh search rows = %d, want 2", len(items))
	}
	n, err := st.ContentRepo().CountSearchCategory(ctx, "article", "zh", "品", products.ID)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("products zh search count = %d, want 2", n)
	}
	// 空结果：about 分类搜 "品" → 0
	about, err := st.CategoryRepo().GetBySlug(ctx, "about")
	if err != nil {
		t.Fatal(err)
	}
	empty, err := st.ContentRepo().SearchByTypeLangCategory(ctx, "article", "zh", "品", about.ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Errorf("about zh search rows = %d, want 0", len(empty))
	}
}
