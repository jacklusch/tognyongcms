package adminapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
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

func TestSearchByCategory(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	// 拿 products 分类 id
	w := e.do(t, http.MethodGet, "/api/categories", "", tok)
	var cl struct {
		Data struct {
			All []struct {
				ID   int64  `json:"id"`
				Path string `json:"path"`
			} `json:"all"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &cl)
	var productsID int64
	for _, a := range cl.Data.All {
		if a.Path == "产品" {
			productsID = a.ID
		}
	}
	if productsID == 0 {
		t.Fatal("未找到产品分类")
	}
	// products 分类内搜 "品" → 2 条
	w = e.do(t, http.MethodGet, "/api/search?type=article&lang=zh&q="+url.QueryEscape("品")+"&category="+strconv.FormatInt(productsID, 10), "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("search = %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Total int `json:"total"`
			Items []struct {
				CategoryName string `json:"category_name"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data.Total != 2 {
		t.Errorf("products zh search total = %d, want 2", resp.Data.Total)
	}
	if len(resp.Data.Items) == 0 || resp.Data.Items[0].CategoryName == "" {
		t.Errorf("products zh search items[0].category_name 应为非空: %s", w.Body.String())
	}
}
