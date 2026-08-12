package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"dulizhan/internal/errs"
	"dulizhan/internal/store"
	"dulizhan/internal/store/sqlite"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	st, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	return New(st, time.Hour)
}

func seedAdmin(t *testing.T, s *Service) int64 {
	t.Helper()
	ctx := context.Background()
	role, err := s.store.RoleRepo().Create(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	// Task 10 的 RoleRepo.Create 存的是 []，需授予 * 使 admin 拥有全部权限
	if err := s.store.RoleRepo().Update(ctx, role.ID, "admin", `["*"]`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser(ctx, "admin", "secret123", role.ID); err != nil {
		t.Fatal(err)
	}
	return role.ID
}

func TestLoginAuthenticateLogout(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)
	seedAdmin(t, s)

	token, err := s.Login(ctx, "admin", "secret123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if token == "" {
		t.Fatal("token 为空")
	}
	us, err := s.Authenticate(ctx, token)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if us.User.Username != "admin" {
		t.Errorf("username = %q", us.User.Username)
	}
	if !us.HasPerm("content.read") { // admin 绕过 → 任意权限
		t.Error("admin 应拥有全部权限")
	}

	if err := s.Logout(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx, token); err != errs.ErrForbidden {
		t.Errorf("登出后 Authenticate = %v, want ErrForbidden", err)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)
	seedAdmin(t, s)
	if _, err := s.Login(ctx, "admin", "wrong"); err != errs.ErrForbidden {
		t.Errorf("错误密码 = %v, want ErrForbidden", err)
	}
	if _, err := s.Login(ctx, "nobody", "x"); err != errs.ErrForbidden {
		t.Errorf("不存在用户 = %v, want ErrForbidden", err)
	}
}

func TestAuthenticateExpired(t *testing.T) {
	ctx := context.Background()
	st, _ := sqlite.Open(":memory:")
	s := New(st, -time.Hour) // 已过期
	role, _ := st.RoleRepo().Create(ctx, "admin")
	u, err := s.CreateUser(ctx, "admin", "secret123", role.ID)
	if err != nil {
		t.Fatal(err)
	}
	// 手动插入过期会话
	if err := st.SessionRepo().Create(ctx, &store.Session{Token: "tok", UserID: u.ID, ExpiresAt: time.Now().Add(-time.Minute).UTC()}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx, "tok"); err != errs.ErrForbidden {
		t.Errorf("过期会话 = %v, want ErrForbidden", err)
	}
}

func TestCreateUserDuplicate(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)
	role, _ := s.store.RoleRepo().Create(ctx, "editor")
	if _, err := s.CreateUser(ctx, "alice", "pw1234", role.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser(ctx, "alice", "pw5678", role.ID); err == nil {
		t.Error("重复用户名应报错")
	}
}

func TestSetUserRoleAndPassword(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)
	role, _ := s.store.RoleRepo().Create(ctx, "author")
	u, _ := s.CreateUser(ctx, "bob", "old123", role.ID)
	role2, _ := s.store.RoleRepo().Create(ctx, "editor")
	if err := s.SetUserRole(ctx, u.ID, role2.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.ChangePassword(ctx, u.ID, "", "newpass"); err != nil {
		t.Fatal(err)
	}
	tok, err := s.Login(ctx, "bob", "newpass")
	if err != nil {
		t.Fatalf("改密后登录失败: %v", err)
	}
	us, _ := s.Authenticate(ctx, tok)
	if us.Role.Name != "editor" {
		t.Errorf("role = %q", us.Role.Name)
	}
}

func TestAuthenticateExpiredDeletesSession(t *testing.T) {
	ctx := context.Background()
	st, _ := sqlite.Open(":memory:")
	s := New(st, -time.Hour)
	role, _ := st.RoleRepo().Create(ctx, "admin")
	u, _ := s.CreateUser(ctx, "admin", "secret123", role.ID)
	if err := st.SessionRepo().Create(ctx, &store.Session{Token: "tok-e", UserID: u.ID, ExpiresAt: time.Now().Add(-time.Minute).UTC()}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx, "tok-e"); err != errs.ErrForbidden {
		t.Fatalf("过期会话 = %v, want ErrForbidden", err)
	}
	// 过期会话应被删除
	if _, err := st.SessionRepo().Get(ctx, "tok-e"); err != errs.ErrNotFound {
		t.Errorf("过期会话未删除: %v", err)
	}
}

func TestHasPermTypePrefix(t *testing.T) {
	// 精确权限
	us1 := &UserSession{Perms: map[string]bool{"content.read.article": true}}
	if !us1.HasPerm("content.read.article") {
		t.Error("精确权限应命中")
	}
	if us1.HasPerm("content.read.page") {
		t.Error("无该类型权限应拒绝")
	}
	// content.* 通配
	us2 := &UserSession{Perms: map[string]bool{"content.*": true}}
	if !us2.HasPerm("content.read.article") {
		t.Error("content.* 应命中任意 content 操作")
	}
	// 基础权限（无类型后缀）
	us3 := &UserSession{Perms: map[string]bool{"content.read": true}}
	if !us3.HasPerm("content.read") {
		t.Error("基础权限自身应命中")
	}
	// admin *
	us4 := &UserSession{Perms: map[string]bool{"*": true}}
	if !us4.HasPerm("anything.at.all") {
		t.Error("admin * 应全命中")
	}
	// 无匹配
	us5 := &UserSession{Perms: map[string]bool{"menus.manage": true}}
	if us5.HasPerm("content.write.article") {
		t.Error("无匹配应拒绝")
	}
}

func TestCreateUserStoresBcrypt(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)
	role, _ := s.store.RoleRepo().Create(ctx, "admin")
	u, err := s.CreateUser(ctx, "bob", "secret123", role.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := s.store.UserRepo().GetByID(ctx, u.ID)
	if got.PasswordHash == "secret123" {
		t.Error("密码未哈希存储")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(got.PasswordHash), []byte("secret123")); err != nil {
		t.Errorf("哈希与密码不匹配: %v", err)
	}
}

func TestChangePasswordRequiresOld(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)
	role, _ := s.store.RoleRepo().Create(ctx, "admin")
	u, _ := s.CreateUser(ctx, "bob", "secret123", role.ID)
	// 旧密码错误 → ErrValidation
	if err := s.ChangePassword(ctx, u.ID, "wrong", "newpass1"); !errors.Is(err, errs.ErrValidation) {
		t.Errorf("旧密码错误 = %v, want ErrValidation", err)
	}
	// 旧密码正确 → 成功
	if err := s.ChangePassword(ctx, u.ID, "secret123", "newpass1"); err != nil {
		t.Fatalf("正确旧密码应成功: %v", err)
	}
	// 旧密码为空（admin 重置）→ 成功
	if err := s.ChangePassword(ctx, u.ID, "", "newpass2"); err != nil {
		t.Fatalf("空旧密码(admin重置)应成功: %v", err)
	}
}
