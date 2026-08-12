package adminapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestUserSelfProtection(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)

	// 获取当前 admin 用户 id
	var me struct {
		Data struct {
			User struct {
				ID int64 `json:"id"`
			} `json:"user"`
		} `json:"data"`
	}
	w := e.do(t, http.MethodGet, "/api/auth/me", "", tok)
	json.Unmarshal(w.Body.Bytes(), &me)
	adminID := me.Data.User.ID
	if adminID == 0 {
		t.Fatal("me 未返回用户 id")
	}

	// 不能删自己
	w = e.do(t, http.MethodDelete, fmt.Sprintf("/api/users/%d", adminID), "", tok)
	if w.Code != http.StatusForbidden {
		t.Errorf("删自己 = %d, want 403, body=%s", w.Code, w.Body.String())
	}
	// 不能改自己角色
	w = e.do(t, http.MethodPut, fmt.Sprintf("/api/users/%d/role", adminID), `{"role_id":2}`, tok)
	if w.Code != http.StatusForbidden {
		t.Errorf("改自己角色 = %d, want 403, body=%s", w.Code, w.Body.String())
	}
	// 不能删内置 admin（username=admin）
	w = e.do(t, http.MethodDelete, fmt.Sprintf("/api/users/%d", adminID), "", tok)
	if w.Code != http.StatusForbidden {
		t.Errorf("删内置 admin = %d, want 403, body=%s", w.Code, w.Body.String())
	}
}

func TestUserPasswordOldCheck(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	ctx := context.Background()
	// 建一个普通用户（admin 角色）
	adminRole, err := e.st.RoleRepo().GetByName(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	w := e.do(t, http.MethodPost, "/api/users", fmt.Sprintf(`{"username":"u1","password":"secret123","role_id":%d}`, adminRole.ID), tok)
	if w.Code != http.StatusOK {
		t.Fatalf("create user = %d %s", w.Code, w.Body.String())
	}
	var created struct {
		Data struct {
			User struct {
				ID int64 `json:"id"`
			} `json:"user"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &created)
	uid := created.Data.User.ID
	if uid == 0 {
		t.Fatal("创建未回填 ID")
	}

	// admin 改他人密码免验旧密码
	w = e.do(t, http.MethodPut, fmt.Sprintf("/api/users/%d/password", uid), `{"password":"newpass1"}`, tok)
	if w.Code != http.StatusOK {
		t.Errorf("admin 改他人密码 = %d %s", w.Code, w.Body.String())
	}
}

func TestUserSelfPasswordOldRequired(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)

	// 取当前登录用户 id
	var me struct {
		Data struct {
			User struct {
				ID int64 `json:"id"`
			} `json:"user"`
		} `json:"data"`
	}
	w := e.do(t, http.MethodGet, "/api/auth/me", "", tok)
	json.Unmarshal(w.Body.Bytes(), &me)
	selfID := me.Data.User.ID
	if selfID == 0 {
		t.Fatal("me 未返回用户 id")
	}

	// 改自己缺 old_password → 422
	w = e.do(t, http.MethodPut, fmt.Sprintf("/api/users/%d/password", selfID), `{"password":"newpass1"}`, tok)
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "改自己密码需提供旧密码") {
		t.Errorf("改自己缺旧密码 = %d, want 422, body=%s", w.Code, w.Body.String())
	}
	// 改自己 old_password 为空串 → 422
	w = e.do(t, http.MethodPut, fmt.Sprintf("/api/users/%d/password", selfID), `{"password":"newpass1","old_password":""}`, tok)
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "改自己密码需提供旧密码") {
		t.Errorf("改自己空旧密码 = %d, want 422, body=%s", w.Code, w.Body.String())
	}
	// 改自己 old_password 错误 → 422
	w = e.do(t, http.MethodPut, fmt.Sprintf("/api/users/%d/password", selfID), `{"password":"newpass1","old_password":"wrongpass"}`, tok)
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "旧密码不正确") {
		t.Errorf("改自己旧密码错 = %d, want 422, body=%s", w.Code, w.Body.String())
	}
	// 改自己 old_password 正确 → 200
	w = e.do(t, http.MethodPut, fmt.Sprintf("/api/users/%d/password", selfID), `{"password":"newpass1","old_password":"admin123"}`, tok)
	if w.Code != http.StatusOK {
		t.Errorf("改自己旧密码正确 = %d, want 200, body=%s", w.Code, w.Body.String())
	}
}

func TestUserCreateRoleNotFound(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	// M2：指定不存在的角色 → 422
	w := e.do(t, http.MethodPost, "/api/users", `{"username":"norole","password":"secret123","role_id":99999}`, tok)
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "角色不存在") {
		t.Errorf("创建用户带不存在角色 = %d, want 422, body=%s", w.Code, w.Body.String())
	}
	// M2：改他人角色为不存在 → 422（本人改自己角色被 403 前置拦截）
	ctx := context.Background()
	adminRole, err := e.st.RoleRepo().GetByName(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.auth.CreateUser(ctx, "m2target", "secret123", adminRole.ID); err != nil {
		t.Fatal(err)
	}
	target, err := e.st.UserRepo().GetByUsername(ctx, "m2target")
	if err != nil {
		t.Fatal(err)
	}
	w = e.do(t, http.MethodPut, fmt.Sprintf("/api/users/%d/role", target.ID), `{"role_id":99999}`, tok)
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "角色不存在") {
		t.Errorf("改用户角色为不存在 = %d, want 422, body=%s", w.Code, w.Body.String())
	}
}
