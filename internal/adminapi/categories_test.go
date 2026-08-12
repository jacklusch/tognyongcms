package adminapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
