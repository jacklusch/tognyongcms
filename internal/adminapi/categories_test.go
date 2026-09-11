package adminapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestCategoriesCRUD(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)

	// 建分类（seed 已预建 news/about/products，用独立 slug 避免冲突）
	w := e.do(t, http.MethodPost, "/api/categories", `{"name":"栏目","slug":"demo-cat","description":"x"}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("create = %d %s", w.Code, w.Body.String())
	}
	// 列表
	w = e.do(t, http.MethodGet, "/api/categories", "", tok)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "demo-cat") {
		t.Errorf("list = %d %s", w.Code, w.Body.String())
	}
	var list struct {
		Data struct {
			Items []struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
			} `json:"items"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &list)
	var found struct {
		hit bool
		id  int64
	}
	for _, it := range list.Data.Items {
		if it.Name == "栏目" {
			found.hit = true
			found.id = it.ID
		}
	}
	if !found.hit {
		t.Fatalf("列表未含新建分类: %+v", list.Data.Items)
	}
	cid := found.id
	// 更新
	w = e.do(t, http.MethodPut, "/api/categories/"+fmt.Sprintf("%d", cid), `{"name":"要闻","slug":"demo-cat","description":"x"}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("update = %d %s", w.Code, w.Body.String())
	}
	// 删除
	w = e.do(t, http.MethodDelete, "/api/categories/"+fmt.Sprintf("%d", cid), "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("delete = %d %s", w.Code, w.Body.String())
	}
}

// C1：author 角色编辑含 category 字段的文章需读分类，GET /categories 应 200（content.read），
// 写操作仍保持 content_types.manage → 403。
func TestAuthorCategoryReadWrite(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	authorRole, err := e.st.RoleRepo().GetByName(ctx, "author")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.st.RoleRepo().Update(ctx, authorRole.ID, "author", `["content.read","content.write","media.upload"]`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.auth.CreateUser(ctx, "author-cat", "secret123", authorRole.ID); err != nil {
		t.Fatal(err)
	}
	authorTok, _ := e.auth.Login(ctx, "author-cat", "secret123")

	w := e.do(t, http.MethodGet, "/api/categories", "", authorTok)
	if w.Code != http.StatusOK {
		t.Errorf("author GET /categories = %d, want 200: %s", w.Code, w.Body.String())
	}
	w = e.do(t, http.MethodPost, "/api/categories", `{"name":"新类","slug":"new-cat"}`, authorTok)
	if w.Code != http.StatusForbidden {
		t.Errorf("author POST /categories = %d, want 403: %s", w.Code, w.Body.String())
	}
}

// 树形返回 + parent_id CRUD + 防环 + slug 自动生成/撞车后缀。
func TestCategoriesTree(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)

	type catResp struct {
		Data struct {
			Category struct {
				ID   int64  `json:"id"`
				Slug string `json:"slug"`
			} `json:"category"`
		} `json:"data"`
	}
	mk := func(body string) catResp {
		t.Helper()
		w := e.do(t, http.MethodPost, "/api/categories", body, tok)
		if w.Code != http.StatusOK {
			t.Fatalf("create %s = %d %s", body, w.Code, w.Body.String())
		}
		var r catResp
		if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	parent := mk(`{"name":"产品中心","slug":"product"}`)
	child := mk(`{"name":"斩拌机","slug":"emulsifier","parent_id":` + strconv.FormatInt(parent.Data.Category.ID, 10) + `}`)
	grand := mk(`{"name":"刀片","slug":"blade","parent_id":` + strconv.FormatInt(child.Data.Category.ID, 10) + `}`)

	// 树形返回
	w := e.do(t, http.MethodGet, "/api/categories", "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("list = %d %s", w.Code, w.Body.String())
	}
	var list struct {
		Data struct {
			Items []catTreeNode `json:"items"`
			All   []struct {
				ID   int64  `json:"id"`
				Path string `json:"path"`
			} `json:"all"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	var root *catTreeNode
	for i := range list.Data.Items {
		if list.Data.Items[i].ID == parent.Data.Category.ID {
			root = &list.Data.Items[i]
			break
		}
	}
	if root == nil {
		t.Fatalf("树根未含 id %d: %+v", parent.Data.Category.ID, list.Data.Items)
	}
	if root.ID != parent.Data.Category.ID || root.ParentID != 0 {
		t.Errorf("root = id %d parent %d, want id %d parent 0", root.ID, root.ParentID, parent.Data.Category.ID)
	}
	if len(root.Children) != 1 || root.Children[0].Name != "斩拌机" || root.Children[0].ID != child.Data.Category.ID || root.Children[0].ParentID != root.ID {
		t.Errorf("child = %+v, want 斩拌机 id %d parent %d", root.Children, child.Data.Category.ID, root.ID)
	}
	g := root.Children[0].Children
	if len(g) != 1 || g[0].Name != "刀片" || g[0].ID != grand.Data.Category.ID || g[0].ParentID != child.Data.Category.ID {
		t.Errorf("grand = %+v, want 刀片 id %d parent %d", g, grand.Data.Category.ID, child.Data.Category.ID)
	}

	// content_count：seed 新闻节点有文章 → >0
	for i := range list.Data.Items {
		if list.Data.Items[i].Name == "新闻" && list.Data.Items[i].ContentCount <= 0 {
			t.Errorf("新闻 content_count = %d, want > 0", list.Data.Items[i].ContentCount)
		}
	}

	// all 带 path
	idByPath := map[string]int64{}
	for _, a := range list.Data.All {
		idByPath[a.Path] = a.ID
	}
	if idByPath["产品中心"] != parent.Data.Category.ID {
		t.Errorf(`all["产品中心"] = %d, want %d`, idByPath["产品中心"], parent.Data.Category.ID)
	}
	if idByPath["产品中心/斩拌机"] != child.Data.Category.ID {
		t.Errorf(`all["产品中心/斩拌机"] = %d, want %d`, idByPath["产品中心/斩拌机"], child.Data.Category.ID)
	}
	if idByPath["产品中心/斩拌机/刀片"] != grand.Data.Category.ID {
		t.Errorf(`all["产品中心/斩拌机/刀片"] = %d, want %d`, idByPath["产品中心/斩拌机/刀片"], grand.Data.Category.ID)
	}

	// 防环：上级分类选为自身或子孙 → 422
	put := func(parentID int64, want int) {
		t.Helper()
		w := e.do(t, http.MethodPut, "/api/categories/"+strconv.FormatInt(parent.Data.Category.ID, 10),
			fmt.Sprintf(`{"name":"产品中心","slug":"product","parent_id":%d}`, parentID), tok)
		if w.Code != want {
			t.Errorf("PUT parent_id=%d = %d, want %d: %s", parentID, w.Code, want, w.Body.String())
		}
	}
	put(child.Data.Category.ID, http.StatusUnprocessableEntity)
	put(parent.Data.Category.ID, http.StatusUnprocessableEntity)

	// 改 parent 为另一合法分类（seed 新闻）→ 200 且读回正确；再还原顶级保持后续流程
	newsID := idByPath["新闻"]
	if w = e.do(t, http.MethodPut, "/api/categories/"+strconv.FormatInt(parent.Data.Category.ID, 10),
		fmt.Sprintf(`{"name":"产品中心","slug":"product","parent_id":%d}`, newsID), tok); w.Code != http.StatusOK {
		t.Fatalf("PUT 合法 parent = %d %s", w.Code, w.Body.String())
	}
	got, err := e.st.CategoryRepo().GetByID(context.Background(), parent.Data.Category.ID)
	if err != nil || got.ParentID != newsID {
		t.Errorf("改 parent 后 = parent %d, err %v, want %d", got.ParentID, err, newsID)
	}
	if w = e.do(t, http.MethodPut, "/api/categories/"+strconv.FormatInt(parent.Data.Category.ID, 10),
		`{"name":"产品中心","slug":"product","parent_id":0}`, tok); w.Code != http.StatusOK {
		t.Fatalf("PUT 还原 parent = %d %s", w.Code, w.Body.String())
	}

	// POST 不存在的 parent_id → 422
	if w = e.do(t, http.MethodPost, "/api/categories", `{"name":"幽灵上级","slug":"ghost-parent","parent_id":999999}`, tok); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("POST 不存在的 parent = %d, want 422: %s", w.Code, w.Body.String())
	}

	// 删除有子分类 → 403
	w = e.do(t, http.MethodDelete, "/api/categories/"+strconv.FormatInt(parent.Data.Category.ID, 10), "", tok)
	if w.Code != http.StatusForbidden {
		t.Errorf("DELETE 有子分类 = %d, want 403: %s", w.Code, w.Body.String())
	}
	// 删孙 → 删子 → 删父 依次成功
	for _, id := range []int64{grand.Data.Category.ID, child.Data.Category.ID, parent.Data.Category.ID} {
		w = e.do(t, http.MethodDelete, "/api/categories/"+strconv.FormatInt(id, 10), "", tok)
		if w.Code != http.StatusOK {
			t.Errorf("DELETE %d = %d, want 200: %s", id, w.Code, w.Body.String())
		}
	}
	// 删除无子分类但有内容 → 403（seed news 有文章）
	if w = e.do(t, http.MethodDelete, "/api/categories/"+strconv.FormatInt(idByPath["新闻"], 10), "", tok); w.Code != http.StatusForbidden {
		t.Errorf("DELETE 新闻 = %d, want 403: %s", w.Code, w.Body.String())
	}

	// slug 自动生成（按 slugify 规则："Mixer Pro"→"mixer-pro"）+ 撞车后缀 -2
	r := mk(`{"name":"Mixer Pro"}`)
	if r.Data.Category.Slug != "mixer-pro" {
		t.Errorf("自动 slug = %q, want mixer-pro", r.Data.Category.Slug)
	}
	r2 := mk(`{"name":"Mixer Pro"}`)
	if r2.Data.Category.Slug != "mixer-pro-2" {
		t.Errorf("撞车 slug = %q, want mixer-pro-2", r2.Data.Category.Slug)
	}
	// 显式 slug 撞车 → 422（不静默加后缀）
	if w = e.do(t, http.MethodPost, "/api/categories", `{"name":"显式撞车","slug":"mixer-pro"}`, tok); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("POST 显式撞车 slug = %d, want 422: %s", w.Code, w.Body.String())
	}
}

// catTreeNode 树形返回结构（仅测试用）。
type catTreeNode struct {
	ID                int64         `json:"id"`
	ParentID          int64         `json:"parent_id"`
	Name              string        `json:"name"`
	Slug              string        `json:"slug"`
	Description       string        `json:"description"`
	ContentCount      int           `json:"content_count"`
	TotalContentCount int           `json:"total_content_count"`
	Children          []catTreeNode `json:"children"`
}

func TestCategoryNameEnAPI(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	// 创建带 name_en
	var created struct {
		Data struct {
			Category struct {
				ID     int64  `json:"id"`
				Name   string `json:"name"`
				NameEn string `json:"name_en"`
			} `json:"category"`
		} `json:"data"`
	}
	w := e.do(t, http.MethodPost, "/api/categories", `{"name":"产品","name_en":"Products","slug":"prod-en"}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("create = %d %s", w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if created.Data.Category.NameEn != "Products" {
		t.Errorf("create name_en = %q", created.Data.Category.NameEn)
	}
	// 更新 name_en
	w = e.do(t, http.MethodPut, "/api/categories/"+strconv.FormatInt(created.Data.Category.ID, 10),
		`{"name":"产品","name_en":"Product","slug":"prod-en"}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("update = %d %s", w.Code, w.Body.String())
	}
	// 列表返回 name_en
	var list struct {
		Data struct {
			Items []struct {
				NameEn string `json:"name_en"`
			} `json:"items"`
		} `json:"data"`
	}
	w = e.do(t, http.MethodGet, "/api/categories", "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("list = %d %s", w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	found := false
	for _, it := range list.Data.Items {
		if it.NameEn == "Product" {
			found = true
		}
	}
	if !found {
		t.Errorf("列表未返回 name_en=Product: %+v", list.Data.Items)
	}
}

func TestCategoriesCountByLang(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	// seed news 分类 zh=3, en=3；products 分类 zh=2
	// 无 lang → 默认 zh
	w := e.do(t, http.MethodGet, "/api/categories", "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("list = %d", w.Code)
	}
	var list struct {
		Data struct {
			Items []catTreeNode `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	count := func(name string) int {
		for _, it := range list.Data.Items {
			if it.Name == name {
				return it.ContentCount
			}
		}
		return -1
	}
	if count("新闻") != 3 {
		t.Errorf("新闻 content_count = %d, want 3", count("新闻"))
	}
	// lang=en → 新闻也是 3（en 3 行）
	w = e.do(t, http.MethodGet, "/api/categories?lang=en", "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("list en = %d", w.Code)
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if count("新闻") != 3 {
		t.Errorf("新闻 en content_count = %d, want 3", count("新闻"))
	}
	// total_content_count 全语言计数：新闻 zh 3 + en 3 = 6；产品 zh 2 + en 2 = 4
	totalCount := func(name string) int {
		for _, it := range list.Data.Items {
			if it.Name == name {
				return it.TotalContentCount
			}
		}
		return -1
	}
	if totalCount("新闻") != 6 {
		t.Errorf("新闻 total_content_count = %d, want 6", totalCount("新闻"))
	}
	if totalCount("产品") != 4 {
		t.Errorf("产品 total_content_count = %d, want 4", totalCount("产品"))
	}
}
