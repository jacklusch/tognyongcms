package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminIndexMissing(t *testing.T) {
	// dist 尚无 index.html（占位阶段），adminIndex 应返回错误而非 panic
	if _, err := adminIndex(); err == nil {
		t.Log("dist/index.html 已存在（构建产物就绪）")
	} else {
		t.Log("dist/index.html 未构建（占位阶段），返回错误属预期")
	}
}

func TestAdminRouteRegistered(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	r := httptest.NewRequest(http.MethodGet, "/admin", nil)
	w := httptest.NewRecorder()
	srv.engine.ServeHTTP(w, r)
	// dist 已含真实构建产物（go:embed），必须返回 200；否则说明 embed 拷贝回归。
	if w.Code != http.StatusOK {
		t.Fatalf("/admin = %d, want 200", w.Code)
	}
}
