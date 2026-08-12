package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"dulizhan/internal/store/sqlite"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newRouter(t *testing.T) (*gin.Engine, *Service) {
	t.Helper()
	st, _ := sqlite.Open(":memory:")
	svc := New(st, time.Hour)
	ctx := context.Background()
	adminRole, _ := st.RoleRepo().Create(ctx, "admin")
	// RoleRepo.Create 存的是 []，授予 * 使 admin 拥有全部权限
	_ = st.RoleRepo().Update(ctx, adminRole.ID, "admin", `["*"]`)
	authorRole, _ := st.RoleRepo().Create(ctx, "author")
	_ = st.RoleRepo().Update(ctx, authorRole.ID, "author", `["content.read","content.write"]`)
	_, _ = svc.CreateUser(ctx, "admin", "secret123", adminRole.ID)
	_, _ = svc.CreateUser(ctx, "writer", "secret123", authorRole.ID)

	r := gin.New()
	return r, svc
}

func TestRequireAuthAndPerm(t *testing.T) {
	r, svc := newRouter(t)
	adminTok, _ := svc.Login(context.Background(), "admin", "secret123")
	authorTok, _ := svc.Login(context.Background(), "writer", "secret123")

	r.GET("/secure", RequireAuth(svc), RequirePerm("settings.manage"), func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	// 未登录 → 401
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/secure", nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("未登录 = %d, want 401", w.Code)
	}

	// 管理员 + 权限 → 200
	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: adminTok})
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("admin = %d, want 200", w.Code)
	}

	// author 无 settings.manage → 403
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/secure", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: authorTok})
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("author = %d, want 403", w.Code)
	}
}

func TestRequirePermType(t *testing.T) {
	r, svc := newRouter(t)
	// newRouter 已建 admin(全权) + author(content.read,content.write)
	adminTok, _ := svc.Login(context.Background(), "admin", "secret123")
	authorTok, _ := svc.Login(context.Background(), "writer", "secret123")

	r.GET("/type-secure/:type", RequireAuth(svc), func(c *gin.Context) {
		RequirePermType("content.read", c.Param("type"))(c)
		c.JSON(200, gin.H{"ok": true})
	})
	// 未登录 → 401
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/type-secure/article", nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("未登录 = %d, want 401", w.Code)
	}
	// admin → 200
	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/type-secure/article", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: adminTok})
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("admin = %d, want 200", w.Code)
	}
	// author（content.read 基础权限）→ 200（无类型后缀，HasPerm("content.read.article") 需 author 有 content.* 或 content.read.*；newRouter 的 author 只有 content.read/content.write → 403）
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/type-secure/article", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: authorTok})
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("author 无类型权限 = %d, want 403", w.Code)
	}
}
