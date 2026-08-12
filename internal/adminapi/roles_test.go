package adminapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestRoleCRUDAndPerms(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	// 权限点列表
	w := e.do(t, http.MethodGet, "/api/roles/perms", "", tok)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "content.read") {
		t.Errorf("perms = %d %s", w.Code, w.Body.String())
	}
	// 建角色
	w = e.do(t, http.MethodPost, "/api/roles", `{"name":"reviewer","permissions":["content.read","content.write"]}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("create role = %d %s", w.Code, w.Body.String())
	}
	// 列表含新角色
	w = e.do(t, http.MethodGet, "/api/roles", "", tok)
	if !strings.Contains(w.Body.String(), "reviewer") {
		t.Errorf("roles 缺新角色: %s", w.Body.String())
	}
	// 更新角色
	var list struct {
		Data struct {
			Items []struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
			} `json:"items"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &list)
	var rid int64
	for _, r := range list.Data.Items {
		if r.Name == "reviewer" {
			rid = r.ID
		}
	}
	if rid == 0 {
		t.Fatal("未找到 reviewer 角色")
	}
	w = e.do(t, http.MethodPut, "/api/roles/"+fmt.Sprintf("%d", rid), `{"name":"reviewer","permissions":["content.read"]}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("update role = %d %s", w.Code, w.Body.String())
	}
	// 删除角色（无引用）
	w = e.do(t, http.MethodDelete, "/api/roles/"+fmt.Sprintf("%d", rid), "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("delete role = %d %s", w.Code, w.Body.String())
	}
}

// roleIDByName 从角色列表解析指定名称角色 id。
func roleIDByName(e *env, t *testing.T, name string) int64 {
	t.Helper()
	w := e.do(t, http.MethodGet, "/api/roles", "", e.login(t))
	if w.Code != http.StatusOK {
		t.Fatalf("roles list = %d %s", w.Code, w.Body.String())
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
	for _, r := range list.Data.Items {
		if r.Name == name {
			return r.ID
		}
	}
	t.Fatalf("未找到角色 %s", name)
	return 0
}

func TestRoleBuiltinProtection(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	ctx := context.Background()

	// 内置拒删：admin/editor/author 均 403
	for _, name := range []string{"admin", "editor", "author"} {
		w := e.do(t, http.MethodDelete, fmt.Sprintf("/api/roles/%d", roleIDByName(e, t, name)), "", tok)
		if w.Code != http.StatusForbidden {
			t.Errorf("删除内置 %s = %d, want 403, body=%s", name, w.Code, w.Body.String())
		}
	}

	// C1 回归：改名内置 admin 角色 → 403；同名校改权限 → 200
	adminID := roleIDByName(e, t, "admin")
	w := e.do(t, http.MethodPut, fmt.Sprintf("/api/roles/%d", adminID), `{"name":"root","permissions":["*"]}`, tok)
	if w.Code != http.StatusForbidden {
		t.Errorf("改 admin 角色名 = %d, want 403, body=%s", w.Code, w.Body.String())
	}
	w = e.do(t, http.MethodPut, fmt.Sprintf("/api/roles/%d", adminID), `{"name":"admin","permissions":["*"]}`, tok)
	if w.Code != http.StatusOK {
		t.Errorf("admin 同名校改权限 = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	// M3：空角色名 → 422
	r, err := e.st.RoleRepo().Create(ctx, "tmp-empty")
	if err != nil {
		t.Fatal(err)
	}
	w = e.do(t, http.MethodPut, fmt.Sprintf("/api/roles/%d", r.ID), `{"name":"","permissions":[]}`, tok)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("空角色名 = %d, want 422, body=%s", w.Code, w.Body.String())
	}
}

func TestRoleDeleteReferencedForbidden(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	ctx := context.Background()

	// 建角色 + 建引用它的用户
	r, err := e.st.RoleRepo().Create(ctx, "reviewer")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.auth.CreateUser(ctx, "refuser", "secret123", r.ID); err != nil {
		t.Fatal(err)
	}
	// 有用户引用拒删（HTTP 层把 ErrForbidden 折叠为通用 403 文案）
	w := e.do(t, http.MethodDelete, fmt.Sprintf("/api/roles/%d", r.ID), "", tok)
	if w.Code != http.StatusForbidden {
		t.Errorf("删被引用角色 = %d, want 403, body=%s", w.Code, w.Body.String())
	}
}
