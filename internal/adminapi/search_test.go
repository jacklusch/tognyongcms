package adminapi

import (
	"net/http"
	"strings"
	"testing"
)

func TestSearchEndpoint(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	// seed 已有 article/hello-zh（标题 "你好，世界"）
	w := e.do(t, http.MethodGet, "/api/search?type=article&lang=zh&q=%E4%B8%96%E7%95%8C", "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("search = %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "hello-zh") {
		t.Errorf("搜索应命中 seed 文章: %s", w.Body.String())
	}
	// 缺 q → 400
	w = e.do(t, http.MethodGet, "/api/search?type=article&lang=zh", "", tok)
	if w.Code != http.StatusBadRequest {
		t.Errorf("缺 q = %d, want 400", w.Code)
	}
	// 未登录 → 401
	w = e.do(t, http.MethodGet, "/api/search?type=article&lang=zh&q=x", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("未登录 = %d, want 401", w.Code)
	}
}
